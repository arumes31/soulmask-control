package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"soulmask-control/internal/docker"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type API struct {
	docker         *docker.Service
	allowedOrigins []string
	upgrader       websocket.Upgrader
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
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_found",
			"error":  err.Error(),
		}); err != nil {
			log.Printf("Error encoding error response: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("Error encoding status response: %v", err)
	}
}

func (a *API) ActionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	action := vars["action"]
	ctx := r.Context()

	var err error
	switch action {
	case "start":
		err = a.docker.Start(ctx)
	case "stop":
		err = a.docker.Stop(ctx)
	case "restart":
		err = a.docker.Restart(ctx)
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
	go func() {
		ctx := r.Context()
		if err := a.docker.CheckAndUpdate(ctx); err != nil {
			log.Printf("Manual update check failed: %v", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) LogsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer func() { _ = conn.Close() }()

	reader, err := a.docker.Logs(r.Context(), "100")
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error reading logs: "+err.Error()))
		return
	}
	defer func() { _ = reader.Close() }()

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if n > 8 {
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

func stripDockerHeader(data []byte) []byte {
	var result []byte
	dataLen := len(data)
	for i := 0; i < dataLen; {
		if i+8 > dataLen {
			result = append(result, data[i:]...)
			break
		}
		// Explicitly check indices for gosec
		b4 := data[i+4] // #nosec G602
		b5 := data[i+5] // #nosec G602
		b6 := data[i+6] // #nosec G602
		b7 := data[i+7] // #nosec G602
		size := int(b4)<<24 | int(b5)<<16 | int(b6)<<8 | int(b7)

		start := i + 8
		end := start + size
		if end > dataLen || end < start {
			result = append(result, data[start:]...)
			break
		}
		result = append(result, data[start:end]...)
		i = end
	}
	if len(result) == 0 && dataLen > 0 {
		return data
	}
	return result
}
