package docker

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDockerClient struct {
	DockerClient
	inspectFunc func(ctx context.Context, containerID string) (container.InspectResponse, error)
	startFunc   func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFunc    func(ctx context.Context, containerID string, options container.StopOptions) error
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return m.inspectFunc(ctx, containerID)
}
func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return m.startFunc(ctx, containerID, options)
}
func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return m.stopFunc(ctx, containerID, options)
}
func (m *mockDockerClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}
func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log line")), nil
}
func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"status":"pulling"}`)), nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return nil
}
func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{ID: "new-id"}, nil
}
func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	return image.InspectResponse{ID: "current-id"}, nil, nil
}
func (m *mockDockerClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	return nil, nil
}

func TestService(t *testing.T) {
	target := "soulmask-server"
	mock := &mockDockerClient{}
	svc := NewServiceWithClient(target, mock, nil)

	t.Run("GetStatus", func(t *testing.T) {
		mock.inspectFunc = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID: "1234567890abcdef",
					State: &container.State{
						Status: "running",
					},
				},
				Config: &container.Config{
					Image: "soulmask:latest",
				},
			}, nil
		}

		info, err := svc.GetStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.ID != "1234567890ab" {
			t.Errorf("Expected truncated ID 1234567890ab, got %s", info.ID)
		}
		if info.Status != "running" {
			t.Errorf("Expected status running, got %s", info.Status)
		}
	})

	t.Run("Start", func(t *testing.T) {
		called := false
		mock.startFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
			called = true
			return nil
		}
		if err := svc.Start(context.Background()); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !called {
			t.Error("ContainerStart was not called")
		}
	})
}
