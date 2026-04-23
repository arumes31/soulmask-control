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
	"soulmask-control/internal/notification"

	"github.com/gorilla/mux"
)

type Config struct {
	TargetContainer string
	AdminPassword   string
	TrustProxy      bool
	AllowedOrigins  []string
	Port            string
	DiscordWebhook  string
	SteamAppID      string
}

func main() {
	cfg := loadConfig()

	// Initialize notification
	var notifier notification.Notifier
	if cfg.DiscordWebhook != "" {
		notifier = notification.NewDiscordNotifier(cfg.DiscordWebhook)
		log.Printf("[Main] Discord notifications enabled")
	}

	// Initialize services
	dockerService, err := docker.NewService(cfg.TargetContainer, notifier, cfg.SteamAppID)
	if err != nil {
		log.Fatalf("[Main] Failed to initialize Docker service: %v", err)
	}

	authenticator := auth.NewAuthenticator(cfg.AdminPassword, cfg.TrustProxy)
	apiServer := api.NewAPI(dockerService, cfg.AllowedOrigins)

	// Context for background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background workers
	go startUpdateWorker(ctx, dockerService)
	go dockerService.ListenForEvents(ctx)

	// Router setup
	r := mux.NewRouter()
	r.Use(middleware.IPMiddleware(cfg.TrustProxy))
	r.Use(middleware.LoggingMiddleware)

	// Auth & Dashboard Routes
	setupWebRoutes(r, authenticator)

	// API Subrouter
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/status", apiServer.StatusHandler).Methods("GET")
	apiRouter.HandleFunc("/action/{action}", apiServer.ActionHandler).Methods("POST")
	apiRouter.HandleFunc("/logs", apiServer.LogsHandler)
	apiRouter.HandleFunc("/check-update", apiServer.CheckUpdateHandler).Methods("POST")

	// Static Assets
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      r,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Server runner
	go func() {
		log.Printf("[Main] Soulmask Control starting on %s (Target: %s)", cfg.Port, cfg.TargetContainer)
		if notifier != nil {
			_ = notifier.Notify("🛠️ **Soulmask Control** starting up...")
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] Server error: %v", err)
		}
	}()

	// Graceful Shutdown Logic
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[Main] Shutting down...")
	cancel() // Signal background worker to stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[Main] Forced shutdown: %v", err)
	}
	if notifier != nil {
		_ = notifier.Notify("💤 **Soulmask Control** shut down")
	}
	log.Println("[Main] Exited successfully")
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	return Config{
		TargetContainer: getEnv("TARGET_CONTAINER", "soulmask-server"),
		AdminPassword:   getEnv("ADMIN_PASSWORD", "admin"),
		TrustProxy:      os.Getenv("TRUST_PROXY") == "true",
		AllowedOrigins:  strings.Split(os.Getenv("ALLOWED_ORIGINS"), ","),
		Port:            port,
		DiscordWebhook:  os.Getenv("DISCORD_WEBHOOK_URL"),
		SteamAppID:      getEnv("STEAM_APP_ID", "2401390"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func setupWebRoutes(r *mux.Router, auth *auth.Authenticator) {
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if auth.IsAuthenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			http.ServeFile(w, r, "./static/login.html")
			return
		}
		auth.LoginHandler(w, r)
	}).Methods(http.MethodGet, http.MethodPost)

	r.HandleFunc("/logout", auth.LogoutHandler).Methods(http.MethodPost)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./static/index.html")
	}).Methods(http.MethodGet)
}

func startUpdateWorker(ctx context.Context, svc *docker.Service) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			workCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			if err := svc.CheckAndUpdate(workCtx); err != nil {
				log.Printf("[UpdateWorker] Scheduled check failed: %v", err)
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}
