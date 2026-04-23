package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"soulmask-control/internal/notification"
)

type DockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	ImageRemove(ctx context.Context, imageID string, options image.RemoveOptions) ([]image.DeleteResponse, error)
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
}

type ContainerInfo struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	Image        string       `json:"image"`
	ImageID      string       `json:"imageId"`
	UpdateStatus UpdateStatus `json:"updateStatus"`
	Stats        *Stats       `json:"stats,omitempty"`
	LatestPatch  *PatchInfo   `json:"latestPatch,omitempty"`
}

type PatchInfo struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	ReleaseDate time.Time `json:"releaseDate"`
	Content     string    `json:"content"`
}

type Stats struct {
	CPUPercentage float64 `json:"cpuPercentage"`
	MemoryUsage   uint64  `json:"memoryUsage"`
	MemoryLimit   uint64  `json:"memoryLimit"`
	DiskRead      uint64  `json:"diskRead"`
	DiskWrite     uint64  `json:"diskWrite"`
}

type UpdateStatus struct {
	IsChecking     bool      `json:"isChecking"`
	IsUpdating     bool      `json:"isUpdating"`
	IsPending      bool      `json:"isPending"`
	PendingTime    time.Time `json:"pendingTime"`
	LastCheck      time.Time `json:"lastCheck"`
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	Error          string    `json:"error"`
	Progress       string    `json:"progress"`
}

type Service struct {
	cli           DockerClient
	target        string
	mu            sync.RWMutex
	updateStatus  UpdateStatus
	notifier      notification.Notifier
	pendingCancel context.CancelFunc
	steamAppID    string
}

func (s *Service) setChecking(checking bool, errStr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateStatus.IsChecking = checking
	if !checking {
		s.updateStatus.LastCheck = time.Now()
		s.updateStatus.Progress = ""
	}
	if errStr != "" {
		s.updateStatus.Error = errStr
	} else if checking {
		s.updateStatus.Error = ""
	}
}

func (s *Service) setUpdating(updating bool, progress string, errStr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateStatus.IsUpdating = updating
	s.updateStatus.Progress = progress
	if updating {
		s.updateStatus.IsPending = false
	}
	if errStr != "" {
		s.updateStatus.Error = errStr
	}
}

func (s *Service) setPending(pending bool, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateStatus.IsPending = pending
	s.updateStatus.PendingTime = t
	if pending {
		s.updateStatus.Progress = "Update scheduled"
	} else {
		s.updateStatus.Progress = ""
	}
}

func NewService(target string, notifier notification.Notifier, steamAppID string) (*Service, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Service{
		cli:    cli,
		target: target,
		updateStatus: UpdateStatus{
			LastCheck: time.Now(),
		},
		notifier:   notifier,
		steamAppID: steamAppID,
	}, nil
}

func NewServiceWithClient(target string, cli DockerClient, notifier notification.Notifier) *Service {
	return &Service{cli: cli, target: target, notifier: notifier}
}

func (s *Service) notify(format string, args ...interface{}) {
	if s.notifier == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if err := s.notifier.Notify(msg); err != nil {
		log.Printf("[Notification] Failed: %v", err)
	}
}

func (s *Service) ListenForEvents(ctx context.Context) {
	msgs, errs := s.cli.Events(ctx, events.ListOptions{})
	
	log.Printf("[Events] Started listening for Docker events on %s", s.target)

	for {
		select {
		case err := <-errs:
			if err != nil && err != io.EOF && ctx.Err() == nil {
				log.Printf("[Events] Error: %v", err)
				// Reconnect after delay
				time.Sleep(5 * time.Second)
				msgs, errs = s.cli.Events(ctx, events.ListOptions{})
			}
		case msg := <-msgs:
			if msg.Type != events.ContainerEventType {
				continue
			}
			
			// Match by ID or Name (target is name in this app)
			if msg.Actor.Attributes["name"] != s.target && msg.Actor.ID[:12] != s.target[:12] && msg.Actor.ID != s.target {
				continue
			}

			switch msg.Action {
			case "start":
				s.notify("🚀 Container **%s** started", s.target)
			case "stop":
				s.notify("🛑 Container **%s** stopped", s.target)
			case "die":
				// If it wasn't a clean stop
				if msg.Actor.Attributes["exitCode"] != "0" {
					s.notify("💀 Container **%s** crashed (Exit Code: %s)", s.target, msg.Actor.Attributes["exitCode"])
				}
			case "oom":
				s.notify("🚨 Container **%s** ran out of memory!", s.target)
			case "restart":
				s.notify("🔄 Container **%s** restarted", s.target)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) GetStatus(ctx context.Context) (*ContainerInfo, error) {
	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.updateStatus.CurrentVersion == "" {
		s.updateStatus.CurrentVersion = inspect.Image
	}
	updateStatus := s.updateStatus
	s.mu.Unlock()

	var stats *Stats
	if inspect.State.Running {
		stats, _ = s.getStats(ctx)
	}

	patch, _ := s.getLatestPatch(ctx)

	return &ContainerInfo{
		ID:           inspect.ID[:12],
		Status:       inspect.State.Status,
		Image:        inspect.Config.Image,
		ImageID:      inspect.Image,
		UpdateStatus: updateStatus,
		Stats:        stats,
		LatestPatch:  patch,
	}, nil
}

func (s *Service) getLatestPatch(ctx context.Context) (*PatchInfo, error) {
	if s.steamAppID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("https://api.steampowered.com/ISteamNews/GetNewsForApp/v0002/?appid=%s&count=1&maxlength=500&format=json", s.steamAppID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		AppNews struct {
			NewsItems []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Date  int64  `json:"date"`
				Contents string `json:"contents"`
			} `json:"newsitems"`
		} `json:"appnews"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.AppNews.NewsItems) == 0 {
		return nil, nil
	}

	item := result.AppNews.NewsItems[0]
	return &PatchInfo{
		Title:       item.Title,
		URL:         item.URL,
		ReleaseDate: time.Unix(item.Date, 0),
		Content:     item.Contents,
	}, nil
}

func (s *Service) getStats(ctx context.Context) (*Stats, error) {
	resp, err := s.cli.ContainerStats(ctx, s.target, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var v container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}

	cpuPercent := 0.0
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	onlineCPUs := float64(v.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	var rx, tx uint64
	for _, blk := range v.BlkioStats.IoServiceBytesRecursive {
		switch blk.Op {
		case "Read":
			rx += blk.Value
		case "Write":
			tx += blk.Value
		}
	}

	return &Stats{
		CPUPercentage: cpuPercent,
		MemoryUsage:   v.MemoryStats.Usage,
		MemoryLimit:   v.MemoryStats.Limit,
		DiskRead:      rx,
		DiskWrite:     tx,
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	return s.cli.ContainerStart(ctx, s.target, container.StartOptions{})
}

func (s *Service) Stop(ctx context.Context) error {
	return s.cli.ContainerStop(ctx, s.target, container.StopOptions{})
}

func (s *Service) Restart(ctx context.Context) error {
	return s.cli.ContainerRestart(ctx, s.target, container.StopOptions{})
}

func (s *Service) Logs(ctx context.Context, tail string) (io.ReadCloser, error) {
	return s.cli.ContainerLogs(ctx, s.target, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
	})
}

func (s *Service) PullImage(ctx context.Context, imageRef string) error {
	reader, err := s.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	decoder := json.NewDecoder(reader)
	for {
		var message struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if message.Error != "" {
			return fmt.Errorf("docker pull error: %s", message.Error)
		}
		
		s.mu.Lock()
		if message.Progress != "" {
			s.updateStatus.Progress = fmt.Sprintf("%s %s", message.Status, message.Progress)
		} else {
			s.updateStatus.Progress = message.Status
		}
		s.mu.Unlock()
	}
	return nil
}

func (s *Service) CheckAndUpdate(ctx context.Context) (err error) {
	s.mu.RLock()
	if s.updateStatus.IsUpdating || s.updateStatus.IsChecking {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.setChecking(true, "")
	defer func() {
		if err != nil {
			s.setChecking(false, err.Error())
			s.notify("❌ Update check failed for **%s**: %v", s.target, err)
		} else {
			s.setChecking(false, "")
		}
	}()

	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return fmt.Errorf("inspect failed: %w", err)
	}

	imageRef := inspect.Config.Image
	oldImageID := inspect.Image

	log.Printf("[UpdateWorker] Checking %s (ID: %s)", imageRef, oldImageID)

	if err = s.PullImage(ctx, imageRef); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	newImage, _, err := s.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("post-pull inspect failed: %w", err)
	}

	s.mu.Lock()
	s.updateStatus.CurrentVersion = oldImageID
	s.updateStatus.LatestVersion = newImage.ID
	s.mu.Unlock()

	if newImage.ID == oldImageID {
		log.Printf("[UpdateWorker] %s is up to date", imageRef)
		return nil
	}

	s.mu.RLock()
	if s.updateStatus.IsPending {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	log.Printf("[UpdateWorker] Update detected: %s -> %s. Delaying 15m.", oldImageID, newImage.ID)
	
	pendingUntil := time.Now().Add(15 * time.Minute)
	s.setPending(true, pendingUntil)
	
	s.notify("✨ New version detected for **%s**\n`%s` ➡️ `%s`\n\n🕒 **Update scheduled in 15 minutes.**", 
		s.target, oldImageID[:12], newImage.ID[:12])

	// Create a cancelable context for the pending update
	pCtx, pCancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.pendingCancel != nil {
		s.pendingCancel()
	}
	s.pendingCancel = pCancel
	s.mu.Unlock()

	// Delayed execution
	go func() { // #nosec G118
		timer := time.NewTimer(15 * time.Minute)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Proceed with update
		case <-pCtx.Done():
			// Cancelled (either by UpdateNow or another check)
			return
		}
		
		// Perform update with a fresh context
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute) // #nosec G118
		defer cancel()
		
		latestInspect, err := s.cli.ContainerInspect(ctx, s.target)
		if err != nil {
			s.setPending(false, time.Time{})
			s.notify("❌ Scheduled update failed for **%s**: could not re-inspect container", s.target)
			return
		}
		
		s.notify("🚀 **Update starting now** for **%s** after 15-minute delay", s.target)
		if err := s.PerformUpdate(ctx, latestInspect); err != nil {
			log.Printf("[UpdateWorker] Delayed update failed: %v", err)
		}
	}()

	return nil
}

func (s *Service) UpdateNow(ctx context.Context) error {
	s.mu.Lock()
	if !s.updateStatus.IsPending {
		s.mu.Unlock()
		return fmt.Errorf("no update pending")
	}
	if s.pendingCancel != nil {
		s.pendingCancel()
		s.pendingCancel = nil
	}
	s.mu.Unlock()

	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return err
	}

	return s.PerformUpdate(ctx, inspect)
}

func (s *Service) PerformUpdate(ctx context.Context, oldInspect container.InspectResponse) (err error) {
	s.setUpdating(true, "Initializing update...", "")
	defer func() {
		if err != nil {
			s.setUpdating(false, "", err.Error())
			s.notify("🚨 Update failed for **%s**: %v", s.target, err)
		} else {
			s.setUpdating(false, "", "")
		}
	}()

	log.Printf("[UpdateWorker] Restarting container %s with new image", s.target)
	
	// 1. Stop
	s.setUpdating(true, "Stopping container...", "")
	_ = s.Stop(ctx) // Best effort stop
	
	// 2. Remove
	s.setUpdating(true, "Removing old container...", "")
	if err = s.cli.ContainerRemove(ctx, s.target, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}
	
	// 3. Prepare Configuration
	config := oldInspect.Config
	hostConfig := oldInspect.HostConfig
	
	// Sanitize networking to prevent conflicts
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: oldInspect.NetworkSettings.Networks,
	}
	
	// 4. Create
	s.setUpdating(true, "Recreating container...", "")
	resp, err := s.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, s.target)
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	
	// 5. Start
	s.setUpdating(true, "Starting new container...", "")
	if err = s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}
	
	// 6. Cleanup
	log.Printf("[UpdateWorker] Cleaning up old image %s", oldInspect.Image)
	_, _ = s.cli.ImageRemove(ctx, oldInspect.Image, image.RemoveOptions{PruneChildren: true})

	log.Printf("[UpdateWorker] Successfully updated %s", s.target)
	s.notify("✅ Successfully updated **%s** to new image", s.target)
	return nil
}
