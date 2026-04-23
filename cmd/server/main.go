package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"soulmask-control/internal/api"
	"soulmask-control/internal/auth"
	"soulmask-control/internal/docker"
	"soulmask-control/internal/middleware"

	"github.com/gorilla/mux"
)

func main() {
	targetContainer := os.Getenv("TARGET_CONTAINER")
	if targetContainer == "" {
		targetContainer = "soulmask-server"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin"
		log.Println("WARNING: Using default password 'admin'")
	}

	trustProxy := os.Getenv("TRUST_PROXY") == "true"
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")

	// Initialize services
	dockerService, err := docker.NewService(targetContainer)
	if err != nil {
		log.Fatal("Failed to initialize Docker service:", err)
	}

	authenticator := auth.NewAuthenticator(adminPassword, trustProxy)
	apiServer := api.NewAPI(dockerService, allowedOrigins)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start update worker
	go startUpdateWorker(ctx, dockerService)

	// Router setup
	r := mux.NewRouter()
	// ... (rest of router setup)
	r.Use(middleware.IPMiddleware(trustProxy))
	r.Use(middleware.LoggingMiddleware)

	// Auth Routes
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if authenticator.IsAuthenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			http.ServeFile(w, r, "./static/login.html")
			return
		}
		authenticator.LoginHandler(w, r)
	}).Methods("GET", "POST")

	r.HandleFunc("/logout", authenticator.LogoutHandler).Methods("POST")

	// Dashboard Root
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !authenticator.IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./static/index.html")
	}).Methods("GET")

	// API Subrouter with Auth
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/status", apiServer.StatusHandler).Methods("GET")
	apiRouter.HandleFunc("/action/{action}", apiServer.ActionHandler).Methods("POST")
	apiRouter.HandleFunc("/logs", apiServer.LogsHandler)
	apiRouter.HandleFunc("/check-update", apiServer.CheckUpdateHandler).Methods("POST")

	// Static files (for assets like CSS and JS)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Soulmask Control starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	cancel() // Stop worker

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

func startUpdateWorker(ctx context.Context, svc *docker.Service) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	log.Println("Update worker started (15m interval)")
	for {
		select {
		case <-ticker.C:
			log.Println("Starting scheduled update check...")
			workCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			if err := svc.CheckAndUpdate(workCtx); err != nil {
				log.Printf("Scheduled update check failed: %v", err)
			}
			cancel()
		case <-ctx.Done():
			log.Println("Update worker shutting down")
			return
		}
	}
}
