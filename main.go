package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	targetContainer = os.Getenv("TARGET_CONTAINER")
	adminPassword   = os.Getenv("ADMIN_PASSWORD")
	trustProxy      = os.Getenv("TRUST_PROXY") == "true"
	allowedOrigins  = strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	sessionCookie   = "soulmask_session"
)

func main() {
	if targetContainer == "" {
		targetContainer = "soulmask-server"
	}
	if adminPassword == "" {
		adminPassword = "admin" // Default password
		log.Println("WARNING: Using default password 'admin'")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()

	// Middleware for real IP and logging
	r.Use(ipMiddleware)
	r.Use(loggingMiddleware)

	// Auth middleware
	authR := r.PathPrefix("/api").Subrouter()
	authR.Use(authMiddleware)

	// Routes
	r.HandleFunc("/login", loginHandler).Methods("POST")
	r.HandleFunc("/logout", logoutHandler).Methods("POST")

	authR.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		statusHandler(w, r, cli)
	}).Methods("GET")

	authR.HandleFunc("/action/{action}", func(w http.ResponseWriter, r *http.Request) {
		actionHandler(w, r, cli)
	}).Methods("POST")

	authR.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		logsHandler(w, r, cli)
	})

	// Serve static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Server starting on :8080")
	log.Fatal(srv.ListenAndServe())
}

func ipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if trustProxy {
			if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
				r.RemoteAddr = net.JoinHostPort(cfIP, "0")
			} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ips := strings.Split(xff, ",")
				r.RemoteAddr = net.JoinHostPort(strings.TrimSpace(ips[0]), "0")
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value != adminPassword {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if creds.Password == adminPassword {
		cookie := &http.Cookie{
			Name:     sessionCookie,
			Value:    adminPassword,
			Path:     "/",
			HttpOnly: true,
			Secure:   trustProxy, // Use secure if behind proxy (assuming SSL termination)
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
}

func statusHandler(w http.ResponseWriter, r *http.Request, cli *client.Client) {
	inspect, err := cli.ContainerInspect(context.Background(), targetContainer)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_found",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": inspect.State.Status,
		"image":  inspect.Config.Image,
		"id":     inspect.ID[:12],
	})
}

func actionHandler(w http.ResponseWriter, r *http.Request, cli *client.Client) {
	vars := mux.Vars(r)
	action := vars["action"]
	ctx := context.Background()

	var err error
	switch action {
	case "start":
		err = cli.ContainerStart(ctx, targetContainer, container.StartOptions{})
	case "stop":
		err = cli.ContainerStop(ctx, targetContainer, container.StopOptions{})
	case "restart":
		err = cli.ContainerRestart(ctx, targetContainer, container.StopOptions{})
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		if len(allowedOrigins) == 0 || (len(allowedOrigins) == 1 && allowedOrigins[0] == "") {
			return true // Allow all if not specified (use with caution)
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

func logsHandler(w http.ResponseWriter, r *http.Request, cli *client.Client) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	}

	reader, err := cli.ContainerLogs(ctx, targetContainer, options)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error reading logs: "+err.Error()))
		return
	}
	defer reader.Close()

	go func() {
		// Keep alive and check for closure
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if n > 0 {
				// Docker log format has 8 byte header
				// [stream type (1 for stdout, 2 for stderr)][3 empty bytes][4 byte payload size]
				// We strip it for simplicity in text mode, or just send as is if we want raw.
				// For the UI, we just send strings.
				if n > 8 {
					// Very basic stripping of docker header
					cleanLog := stripDockerHeader(buf[:n])
					if err := conn.WriteMessage(websocket.TextMessage, cleanLog); err != nil {
						return
					}
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Println("Reader error:", err)
				}
				return
			}
		}
	}
}

func stripDockerHeader(data []byte) []byte {
	// Docker headers are 8 bytes long
	var result []byte
	for i := 0; i < len(data); {
		if i+8 > len(data) {
			break
		}
		size := int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
		start := i + 8
		end := start + size
		if end > len(data) {
			result = append(result, data[start:]...)
			break
		}
		result = append(result, data[start:end]...)
		i = end
	}
	if len(result) == 0 && len(data) > 0 {
		return data // Fallback if format is unexpected
	}
	return result
}
