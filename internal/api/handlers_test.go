package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"soulmask-control/internal/docker"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockClient struct {
	docker.DockerClient
}

func (m *mockClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    "1234567890abcdef",
			State: &container.State{Status: "running"},
		},
		Config: &container.Config{Image: "img"},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{},
		},
	}, nil
}
func (m *mockClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return nil
}
func (m *mockClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}
func (m *mockClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}
func (m *mockClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	return image.InspectResponse{
		RepoTags: []string{"img:latest"},
	}, []byte{}, nil
}
func (m *mockClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString(`{"status":"pulling..."}`)), nil
}
func (m *mockClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{ID: "new123"}, nil
}
func (m *mockClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return nil
}
func (m *mockClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	return nil, nil
}
func (m *mockClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	// stdcopy format: [stream type byte] [0 0 0] [size uint32] [payload]
	// 1 for stdout
	msg := []byte{1, 0, 0, 0, 0, 0, 0, 4, 't', 'e', 's', 't'}
	return io.NopCloser(bytes.NewReader(msg)), nil
}

type mockClientError struct {
	docker.DockerClient
}

func (m *mockClientError) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return container.InspectResponse{}, context.DeadlineExceeded
}

func (m *mockClientError) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return context.DeadlineExceeded
}
func (m *mockClientError) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return context.DeadlineExceeded
}
func (m *mockClientError) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	return context.DeadlineExceeded
}
func (m *mockClientError) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, context.DeadlineExceeded
}

func TestAPIHandlers(t *testing.T) {
	mockCli := &mockClient{}
	svc := docker.NewServiceWithClient("target", mockCli, nil)
	api := NewAPI(svc, []string{"*"})

	t.Run("StatusHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/status", nil)
		w := httptest.NewRecorder()

		api.StatusHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var info docker.ContainerInfo
		if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if info.Status != "running" {
			t.Error("Expected status running")
		}
	})

	t.Run("StatusHandler error", func(t *testing.T) {
		errMockCli := &mockClientError{}
		svcErr := docker.NewServiceWithClient("target", errMockCli, nil)
		apiErr := NewAPI(svcErr, []string{})
		req := httptest.NewRequest("GET", "/api/status", nil)
		w := httptest.NewRecorder()

		apiErr.StatusHandler(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})

	t.Run("ActionHandler Start", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/action/start", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "start"})
		w := httptest.NewRecorder()

		api.ActionHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("ActionHandler Stop", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/action/stop", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "stop"})
		w := httptest.NewRecorder()

		api.ActionHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("ActionHandler Restart", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/action/restart", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "restart"})
		w := httptest.NewRecorder()

		api.ActionHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("ActionHandler UpdateNow", func(t *testing.T) {
		svc.SetUpdateStatusForTest(true)
		req := httptest.NewRequest("POST", "/api/action/update-now", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "update-now"})
		w := httptest.NewRecorder()

		api.ActionHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("ActionHandler Invalid", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/action/invalid", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "invalid"})
		w := httptest.NewRecorder()

		api.ActionHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("ActionHandler Error", func(t *testing.T) {
		errMockCli := &mockClientError{}
		svcErr := docker.NewServiceWithClient("target", errMockCli, nil)
		apiErr := NewAPI(svcErr, []string{})

		req := httptest.NewRequest("POST", "/api/action/start", nil)
		req = mux.SetURLVars(req, map[string]string{"action": "start"})
		w := httptest.NewRecorder()

		apiErr.ActionHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", w.Code)
		}
	})

	t.Run("CheckUpdateHandler", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/check-update", nil)
		w := httptest.NewRecorder()

		api.CheckUpdateHandler(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected 202, got %d", w.Code)
		}
	})

	t.Run("checkOrigin", func(t *testing.T) {
		apiOrigins := NewAPI(svc, []string{"http://example.com"})
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		if !apiOrigins.checkOrigin(req) {
			t.Error("Expected origin check to pass")
		}

		req.Header.Set("Origin", "http://other.com")
		if apiOrigins.checkOrigin(req) {
			t.Error("Expected origin check to fail")
		}

		apiEmpty := NewAPI(svc, []string{})
		if !apiEmpty.checkOrigin(req) {
			t.Error("Expected origin check to pass for empty origins")
		}
	})

	t.Run("LogsHandler GET no upgrade", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/logs", nil)
		w := httptest.NewRecorder()

		api.LogsHandler(w, req)
		// Should return 400 bad request without websocket upgrade headers
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("LogsHandler websocket fail logs read", func(t *testing.T) {
		errMockCli := &mockClientError{}
		svcErr := docker.NewServiceWithClient("target", errMockCli, nil)
		apiErr := NewAPI(svcErr, []string{})

		server := httptest.NewServer(http.HandlerFunc(apiErr.LogsHandler))
		defer server.Close()

		url := "ws" + server.URL[4:]
		ws, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("could not open websocket connection: %v", err)
		}
		defer func() { _ = ws.Close() }()

		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("expected message, got error: %v", err)
		}
		if string(msg) != "Error reading logs: context deadline exceeded" {
			t.Errorf("unexpected message: %s", msg)
		}
	})

	t.Run("LogsHandler websocket success logs read", func(t *testing.T) {
		apiLocal := NewAPI(svc, []string{})
		server := httptest.NewServer(http.HandlerFunc(apiLocal.LogsHandler))
		defer server.Close()

		url := "ws" + server.URL[4:]
		ws, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("could not open websocket connection: %v", err)
		}
		defer func() { _ = ws.Close() }()

		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("expected message, got error: %v", err)
		}

		var logMsg LogMessage
		if err := json.Unmarshal(msg, &logMsg); err != nil {
			t.Fatalf("failed to parse log message: %v", err)
		}

		if logMsg.Content != "test" {
			t.Errorf("Expected test, got %s", logMsg.Content)
		}
	})
}
