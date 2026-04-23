package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"soulmask-control/internal/docker"

	"github.com/docker/docker/api/types/container"
)

type mockClient struct {
	docker.DockerClient
}

func (m *mockClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID: "1234567890abcdef",
			State: &container.State{Status: "running"},
		},
		Config: &container.Config{Image: "img"},
	}, nil
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
}
