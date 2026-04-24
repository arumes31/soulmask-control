package docker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockDockerClient struct {
	DockerClient
	inspectFunc    func(ctx context.Context, containerID string) (container.InspectResponse, error)
	startFunc      func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFunc       func(ctx context.Context, containerID string, options container.StopOptions) error
	restartFunc    func(ctx context.Context, containerID string, options container.StopOptions) error
	statsFunc      func(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	removeFunc     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	createFunc     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	imgInspectFunc func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	imgPullFunc    func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	eventsFunc     func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if m.inspectFunc != nil {
		return m.inspectFunc(ctx, containerID)
	}
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    "1234567890abcdef1234",
			State: &container.State{Status: "running"},
			Image: "old-image-id-1234567890",
		},
		Config: &container.Config{Image: "img1234567890"},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{},
		},
	}, nil
}
func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, containerID, options)
	}
	return nil
}
func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.stopFunc != nil {
		return m.stopFunc(ctx, containerID, options)
	}
	return nil
}
func (m *mockDockerClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.restartFunc != nil {
		return m.restartFunc(ctx, containerID, options)
	}
	return nil
}
func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log line")), nil
}
func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.imgPullFunc != nil {
		return m.imgPullFunc(ctx, ref, options)
	}
	return io.NopCloser(strings.NewReader(`{"status":"pulling"}`)), nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, containerID, options)
	}
	return nil
}
func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "new-id"}, nil
}
func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	if m.imgInspectFunc != nil {
		return m.imgInspectFunc(ctx, imageID)
	}
	return image.InspectResponse{ID: "current-id-123456789"}, nil, nil
}
func (m *mockDockerClient) ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error) {
	return nil, nil
}
func (m *mockDockerClient) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	if m.eventsFunc != nil {
		return m.eventsFunc(ctx, options)
	}
	chMsg := make(chan events.Message, 1)
	chErr := make(chan error, 1)
	chMsg <- events.Message{Action: "die", Actor: events.Actor{Attributes: map[string]string{"name": "soulmask-server"}}}
	return chMsg, chErr
}

func (m *mockDockerClient) ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
	if m.statsFunc != nil {
		return m.statsFunc(ctx, containerID, stream)
	}
	return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":2},"system_cpu_usage":2,"online_cpus":1},"precpu_stats":{"cpu_usage":{"total_usage":1},"system_cpu_usage":1},"memory_stats":{"usage":1024,"limit":2048},"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":100},{"op":"Write","value":50}]}}`))}, nil
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

	t.Run("Stop", func(t *testing.T) {
		called := false
		mock.stopFunc = func(ctx context.Context, containerID string, options container.StopOptions) error {
			called = true
			return nil
		}
		if err := svc.Stop(context.Background()); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !called {
			t.Error("ContainerStop was not called")
		}
	})

	t.Run("Restart", func(t *testing.T) {
		called := false
		mock.restartFunc = func(ctx context.Context, containerID string, options container.StopOptions) error {
			called = true
			return nil
		}
		if err := svc.Restart(context.Background()); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !called {
			t.Error("ContainerRestart was not called")
		}
	})

	t.Run("Logs", func(t *testing.T) {
		reader, err := svc.Logs(context.Background(), "10")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		defer func() { _ = reader.Close() }()

		buf, _ := io.ReadAll(reader)
		if string(buf) != "log line" {
			t.Errorf("Expected 'log line', got '%s'", string(buf))
		}
	})

	t.Run("SetUpdateStatusForTest", func(t *testing.T) {
		svc.SetUpdateStatusForTest(true)
		if !svc.updateStatus.IsPending {
			t.Error("Expected IsPending to be true")
		}
		svc.SetUpdateStatusForTest(false)
	})

	t.Run("setChecking", func(t *testing.T) {
		svc.setChecking(true, "")
		if !svc.updateStatus.IsChecking {
			t.Error("Expected IsChecking to be true")
		}
		svc.setChecking(false, "")
	})

	t.Run("setUpdating", func(t *testing.T) {
		svc.setUpdating(true, "test step", "test error")
		if !svc.updateStatus.IsUpdating || svc.updateStatus.Progress != "test step" || svc.updateStatus.Error != "test error" {
			t.Error("setUpdating failed to set fields")
		}
		svc.setUpdating(false, "", "")
	})

	t.Run("setPending", func(t *testing.T) {
		svc.setPending(true, time.Now())
		if !svc.updateStatus.IsPending {
			t.Error("setPending failed to set fields")
		}
		svc.setPending(false, time.Time{})
	})

	t.Run("notify nil notifier", func(t *testing.T) {
		// Should not panic or error
		svc.notify("test message")
	})

	t.Run("UpdateNow no pending", func(t *testing.T) {
		svc.SetUpdateStatusForTest(false)
		err := svc.UpdateNow(context.Background())
		if err == nil || err.Error() != "no update pending" {
			t.Errorf("Expected 'no update pending' error, got %v", err)
		}
	})

	t.Run("PullImage", func(t *testing.T) {
		err := svc.PullImage(context.Background(), "test-image")
		if err != nil {
			t.Errorf("Unexpected error pulling image: %v", err)
		}
	})

	t.Run("CheckAndUpdate", func(t *testing.T) {
		mock.inspectFunc = nil
		err := svc.CheckAndUpdate(context.Background())
		if err != nil {
			t.Errorf("Unexpected error in CheckAndUpdate: %v", err)
		}
	})

	t.Run("PerformUpdate", func(t *testing.T) {
		mock.inspectFunc = nil
		inspectResponse, _ := mock.ContainerInspect(context.Background(), "target")
		err := svc.PerformUpdate(context.Background(), inspectResponse)
		if err != nil {
			t.Errorf("Unexpected error in PerformUpdate: %v", err)
		}
	})

	t.Run("ListenForEvents", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		// Don't cancel immediately, let it process the mock event
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		svc.ListenForEvents(ctx)
	})

	t.Run("StartLatencyMonitor", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		// Allow some time for ping logic to run
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		svc.StartLatencyMonitor(ctx)
	})
}

// We will mock http.DefaultTransport to intercept requests to api.steampowered.com
type mockTransport struct {
	http.RoundTripper
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestFetchNewsMocked(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)
	svc.steamAppID = "2886870"

	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	// Test fallback and successful fetch
	http.DefaultTransport = &mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "appid=2886870") {
				// No news items
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"appnews":{"newsitems":[]}}`)),
				}, nil
			}
			if strings.Contains(req.URL.String(), "appid=2401390") {
				// Has patch note
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"appnews":{"newsitems":[{"title":"Patch","feedlabel":"Community Announcements","url":"test.com","date":12345,"contents":"fix"}]}}`)),
				}, nil
			}
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
		},
	}

	patch, err := svc.getLatestPatch(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if patch == nil || patch.Title != "Patch" {
		t.Errorf("Expected patch title 'Patch', got %v", patch)
	}

	// Test error decoding
	http.DefaultTransport = &mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`invalid json`)),
			}, nil
		},
	}

	_, err = svc.getLatestPatch(context.Background())
	if err == nil {
		t.Errorf("Expected error decoding JSON")
	}

	// Test network error
	http.DefaultTransport = &mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		},
	}

	_, err = svc.getLatestPatch(context.Background())
	if err == nil {
		t.Errorf("Expected error from RoundTrip")
	}
}

func TestGetStats(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	// Test success
	stats, err := svc.getStats(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if stats == nil {
		t.Errorf("Expected stats, got nil")
	} else {
		if stats.CPUPercentage != 100.0 {
			t.Errorf("Expected CPUUsage 100.0, got %.2f", stats.CPUPercentage)
		}
		if stats.MemoryUsage != 1024 {
			t.Errorf("Expected MemoryUsage 1024, got %d", stats.MemoryUsage)
		}
		if stats.MemoryLimit != 2048 {
			t.Errorf("Expected MemoryLimit 2048, got %d", stats.MemoryLimit)
		}
		if stats.DiskRead != 100 {
			t.Errorf("Expected DiskRead 100, got %d", stats.DiskRead)
		}
		if stats.DiskWrite != 50 {
			t.Errorf("Expected DiskWrite 50, got %d", stats.DiskWrite)
		}
	}
}

func TestGetStatsError(t *testing.T) {
	mockErrorClient := &mockDockerClient{}
	// override container stats method to return error
	mockErrorClient.statsFunc = func(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
		return container.StatsResponseReader{}, context.DeadlineExceeded
	}
	svcError := NewServiceWithClient("target", mockErrorClient, nil)

	_, err := svcError.getStats(context.Background())
	if err == nil {
		t.Errorf("Expected error from getStats")
	}
}

func TestCheckAndUpdate(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	// mock inspect to not return an image ID to fail right away
	mock.inspectFunc = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
		return container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				ID:    "1234567890abcdef1234",
				State: &container.State{Status: "running"},
				Image: "old-image-id-1234567890",
			},
			Config: &container.Config{Image: ""},
		}, nil
	}

	err := svc.CheckAndUpdate(context.Background())
	if err == nil || err.Error() != "no image configured" {
		t.Errorf("Expected 'no image configured' error, got %v", err)
	}

	// Mock to simulate an update detected
	mock.inspectFunc = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
		return container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				ID:    "1234567890abcdef1234",
				State: &container.State{Status: "running"},
				Image: "old-image-id-1234567890",
			},
			Config: &container.Config{Image: "img1234567890"},
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{},
			},
		}, nil
	}

	err = svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !svc.updateStatus.IsPending {
		t.Error("Expected update to be pending")
	}

	// Trigger pending update
	err = svc.UpdateNow(context.Background())
	if err != nil {
		t.Errorf("Unexpected error calling UpdateNow: %v", err)
	}
}

func TestPerformUpdateMockErrors(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	inspectResponse := container.InspectResponse{
		Config: &container.Config{},
		ContainerJSONBase: &container.ContainerJSONBase{
			HostConfig: &container.HostConfig{},
			Image:      "old-image-id",
		},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{},
		},
	}

	// 1. ContainerRemove Error
	mock.removeFunc = func(ctx context.Context, containerID string, options container.RemoveOptions) error {
		return context.DeadlineExceeded
	}
	err := svc.PerformUpdate(context.Background(), inspectResponse)
	if err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Errorf("Expected remove failed error, got: %v", err)
	}

	// 2. ContainerCreate Error
	mock.removeFunc = nil
	mock.createFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{}, context.DeadlineExceeded
	}
	err = svc.PerformUpdate(context.Background(), inspectResponse)
	if err == nil || !strings.Contains(err.Error(), "create failed") {
		t.Errorf("Expected create failed error, got: %v", err)
	}

	// 3. ContainerStart Error
	mock.createFunc = nil
	mock.startFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
		return context.DeadlineExceeded
	}
	err = svc.PerformUpdate(context.Background(), inspectResponse)
	if err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Errorf("Expected start failed error, got: %v", err)
	}
}

func TestCheckAndUpdatePullError(t *testing.T) {
	mock := &mockDockerClient{
		imgPullFunc: func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return nil, context.DeadlineExceeded
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	err := svc.CheckAndUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("Expected pull failed error, got %v", err)
	}
}

func TestCheckAndUpdateImgInspectError(t *testing.T) {
	mock := &mockDockerClient{
		imgInspectFunc: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
			return image.InspectResponse{}, nil, context.DeadlineExceeded
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	err := svc.CheckAndUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "post-pull inspect failed") {
		t.Errorf("Expected post-pull inspect failed error, got %v", err)
	}
}

func TestCheckAndUpdateUpToDate(t *testing.T) {
	mock := &mockDockerClient{
		imgInspectFunc: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
			return image.InspectResponse{ID: "old-image-id-1234567890"}, nil, nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	err := svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if svc.updateStatus.IsPending {
		t.Error("Expected update to NOT be pending")
	}
}

func TestListenForEventsAllBranches(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	chMsg := make(chan events.Message)
	chErr := make(chan error)

	mock.eventsFunc = func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
		return chMsg, chErr
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.ListenForEvents(ctx)

	// Send an error that triggers reconnect
	chErr <- io.ErrUnexpectedEOF

	// Send ignored type
	chMsg <- events.Message{Type: events.ImageEventType}

	// Send unmatched container
	chMsg <- events.Message{
		Type:  events.ContainerEventType,
		Actor: events.Actor{Attributes: map[string]string{"name": "other-target"}},
	}

	// Send matched container - start
	chMsg <- events.Message{
		Type:   events.ContainerEventType,
		Action: "start",
		Actor:  events.Actor{Attributes: map[string]string{"name": "target"}},
	}

	// Send matched container - stop
	chMsg <- events.Message{
		Type:   events.ContainerEventType,
		Action: "stop",
		Actor:  events.Actor{Attributes: map[string]string{"name": "target"}},
	}

	// Send matched container - restart
	chMsg <- events.Message{
		Type:   events.ContainerEventType,
		Action: "restart",
		Actor:  events.Actor{Attributes: map[string]string{"name": "target"}},
	}

	// Send matched container - oom
	chMsg <- events.Message{
		Type:   events.ContainerEventType,
		Action: "oom",
		Actor:  events.Actor{Attributes: map[string]string{"name": "target"}},
	}

	// Send matched container - die non-zero exit code
	chMsg <- events.Message{
		Type:   events.ContainerEventType,
		Action: "die",
		Actor:  events.Actor{Attributes: map[string]string{"name": "target", "exitCode": "1"}},
	}

	// Wait a bit to process messages
	time.Sleep(10 * time.Millisecond)
	cancel()
}

func TestStartLatencyMonitorError(t *testing.T) {
	// Let ping fail
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	// Temporarily override ping behavior or expect it to return error?
	// It pings google and cloudflare directly via OS ping.
	// In StartLatencyMonitor, it runs an infinite loop calling `exec.Command("ping")`.
	// Just coverage test.
	ctx, cancel := context.WithCancel(context.Background())
	go svc.StartLatencyMonitor(ctx)
	cancel()
}

type mockErrNotifier struct{}

func (mockErrNotifier) Notify(msg string) error {
	return context.DeadlineExceeded
}

func TestStartLatencyMonitorFastCancel(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	ctx, cancel := context.WithCancel(context.Background())
	// cancel immediately to test context done
	cancel()
	svc.StartLatencyMonitor(ctx)
}

func TestNotifyError(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)
	svc.notifier = mockErrNotifier{}
	svc.notify("test err")
}

func TestNewService(t *testing.T) {
	_, _ = NewService("target", nil, "steam_id")
}

func TestGetStatusInspectError(t *testing.T) {
	mock := &mockDockerClient{
		inspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{}, context.DeadlineExceeded
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	_, err := svc.GetStatus(context.Background())
	if err == nil {
		t.Errorf("Expected error from GetStatus")
	}
}

func TestCheckAndUpdateCheckingUpdating(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewServiceWithClient("target", mock, nil)

	svc.setChecking(true, "")
	err := svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Errorf("Expected nil error when already checking, got %v", err)
	}

	svc.setChecking(false, "")
	svc.setUpdating(true, "", "")
	err = svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Errorf("Expected nil error when already updating, got %v", err)
	}
}

func TestPullImageErrors(t *testing.T) {
	// Test pull error in stream
	mock := &mockDockerClient{
		imgPullFunc: func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"error":"pull failed"}`)), nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	err := svc.PullImage(context.Background(), "test-image")
	if err == nil || !strings.Contains(err.Error(), "docker pull error") {
		t.Errorf("Expected docker pull error, got %v", err)
	}

	// Test progress output
	mock.imgPullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"status":"Downloading","progress":"10%"}`)), nil
	}

	err = svc.PullImage(context.Background(), "test-image")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if svc.updateStatus.Progress != "Downloading 10%" {
		t.Errorf("Expected progress 'Downloading 10%%', got %s", svc.updateStatus.Progress)
	}

	// Test decode error
	mock.imgPullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`invalid json`)), nil
	}

	err = svc.PullImage(context.Background(), "test-image")
	if err == nil {
		t.Errorf("Expected json decode error")
	}
}

func TestCheckAndUpdateInspectErrInsideCheck(t *testing.T) {
	mock := &mockDockerClient{
		inspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{}, context.DeadlineExceeded
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	err := svc.CheckAndUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "inspect failed") {
		t.Errorf("Expected inspect failed error")
	}
}

func TestPendingUpdateGoroutine(t *testing.T) {
	// We want to trigger the CheckAndUpdate, let it detect an update,
	// then it spawns a goroutine. We can cancel it via UpdateNow immediately,
	// but let's test the path where the timer fires.
	// We can't mock time.After easily, but we can override the CheckAndUpdate pending goroutine?
	// It's inside CheckAndUpdate.
	// Actually, UpdateNow cancels the pending update.
	// We have UpdateNow tests but let's make sure it hits the exact return nil path.
	mock := &mockDockerClient{
		inspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:    "1234567890abcdef1234",
					State: &container.State{Status: "running"},
					Image: "old-image-id-1234567890",
				},
				Config: &container.Config{Image: "img1234567890"},
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{},
				},
			}, nil
		},
		imgInspectFunc: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
			return image.InspectResponse{ID: "new-image-id-1234567890"}, nil, nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	// Start pending update
	err := svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Fatalf("Unexpected err: %v", err)
	}

	// Pending goroutine is sleeping for 15 min.
	// If we cancel the context, it should return.
	// Wait, we don't have direct access to cancel it other than UpdateNow, which cancels `pCtx`.
	_ = svc.UpdateNow(context.Background())

	// Wait a tiny bit for the goroutine to exit
	time.Sleep(10 * time.Millisecond)
}

func TestGetStatsDecodeError(t *testing.T) {
	mock := &mockDockerClient{
		statsFunc: func(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error) {
			return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`invalid json`))}, nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	_, err := svc.getStats(context.Background())
	if err == nil {
		t.Errorf("Expected json decode error from getStats")
	}
}

func TestCheckAndUpdateAlreadyPending(t *testing.T) {
	mock := &mockDockerClient{
		inspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:    "1234567890abcdef1234",
					State: &container.State{Status: "running"},
					Image: "old-image-id-1234567890",
				},
				Config: &container.Config{Image: "img1234567890"},
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{},
				},
			}, nil
		},
		imgInspectFunc: func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
			return image.InspectResponse{ID: "new-image-id-1234567890"}, nil, nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)
	svc.setPending(true, time.Now())

	err := svc.CheckAndUpdate(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGetStatusRunningStats(t *testing.T) {
	mock := &mockDockerClient{
		inspectFunc: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:    "1234567890abcdef1234",
					State: &container.State{Status: "running", Running: true},
					Image: "old-image-id-1234567890",
				},
				Config: &container.Config{Image: "img1234567890"},
				NetworkSettings: &container.NetworkSettings{
					Networks: map[string]*network.EndpointSettings{},
				},
			}, nil
		},
	}
	svc := NewServiceWithClient("target", mock, nil)

	info, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info.Stats == nil {
		t.Errorf("Expected stats to be populated")
	}
}
