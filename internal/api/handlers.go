package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"soulmask-control/internal/docker"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/docker/docker/pkg/stdcopy"
)

type API struct {
	docker         *docker.Service
	allowedOrigins []string
	upgrader       websocket.Upgrader
}

type LogMessage struct {
	Type    string `json:"type"` // "stdout" or "stderr"
	Content string `json:"content"`
}

type wsWriter struct {
	conn   *websocket.Conn
	stream string
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	msg := LogMessage{
		Type:    w.stream,
		Content: string(p),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}
	if err := w.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return 0, err
	}
	return len(p), nil
}

func NewAPI(docker *docker.Service, allowedOrigins []string) *API {
	a := &API{
		docker:         docker,
		allowedOrigins: allowedOrigins,
	}
	a.upgrader = websocket.Upgrader{
		CheckOrigin: a.checkOrigin,
	}
	return a
}

func (a *API) checkOrigin(r *http.Request) bool {
	if len(a.allowedOrigins) == 0 || (len(a.allowedOrigins) == 1 && a.allowedOrigins[0] == "") {
		return true
	}
	origin := r.Header.Get("Origin")
	for _, allowed := range a.allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (a *API) StatusHandler(w http.ResponseWriter, r *http.Request) {
	info, err := a.docker.GetStatus(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(info)
}

func (a *API) ActionHandler(w http.ResponseWriter, r *http.Request) {
	action := mux.Vars(r)["action"]
	var err error

	switch action {
	case "start":
		err = a.docker.Start(r.Context())
	case "stop":
		err = a.docker.Stop(r.Context())
	case "restart":
		err = a.docker.Restart(r.Context())
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

func (a *API) CheckUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// Use Background context for the manual trigger to avoid premature cancellation
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := a.docker.CheckAndUpdate(ctx); err != nil {
			log.Printf("[API] Manual update check failed: %v", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) LogsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[API] WebSocket upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	reader, err := a.docker.Logs(r.Context(), "100")
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error reading logs: "+err.Error()))
		return
	}
	defer func() { _ = reader.Close() }()

	// Drain reads to handle client closes
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	stdout := &wsWriter{conn: conn, stream: "stdout"}
	stderr := &wsWriter{conn: conn, stream: "stderr"}

	// stdcopy.StdCopy will demultiplex the Docker log stream
	if _, err := stdcopy.StdCopy(stdout, stderr, reader); err != nil {
		log.Printf("[API] Log streaming ended: %v", err)
	}
}

