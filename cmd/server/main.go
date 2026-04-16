package main

import (
	"log"
	"net/http"
	"os"
	"strings"
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

	// Router setup
	r := mux.NewRouter()

	// Global Middleware
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

	// Static files (for assets like CSS and JS)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Soulmask Control starting on :8080")
	log.Fatal(srv.ListenAndServe())
}
