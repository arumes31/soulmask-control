package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
}

type ContainerInfo struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	Image        string       `json:"image"`
	ImageID      string       `json:"imageId"`
	UpdateStatus UpdateStatus `json:"updateStatus"`
}

type UpdateStatus struct {
	IsChecking bool      `json:"isChecking"`
	IsUpdating bool      `json:"isUpdating"`
	LastCheck  time.Time `json:"lastCheck"`
	Error      string    `json:"error"`
	Progress   string    `json:"progress"`
}

type Service struct {
	cli          DockerClient
	target       string
	mu           sync.RWMutex
	updateStatus UpdateStatus
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
	if errStr != "" {
		s.updateStatus.Error = errStr
	}
}

func NewService(target string) (*Service, error) {
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
	}, nil
}

func NewServiceWithClient(target string, cli DockerClient) *Service {
	return &Service{cli: cli, target: target}
}

func (s *Service) GetStatus(ctx context.Context) (*ContainerInfo, error) {
	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return &ContainerInfo{
		ID:           inspect.ID[:12],
		Status:       inspect.State.Status,
		Image:        inspect.Config.Image,
		ImageID:      inspect.Image,
		UpdateStatus: s.updateStatus,
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
	defer reader.Close()

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

	if newImage.ID == oldImageID {
		log.Printf("[UpdateWorker] %s is up to date", imageRef)
		return nil
	}

	log.Printf("[UpdateWorker] Update detected: %s -> %s", oldImageID, newImage.ID)
	return s.PerformUpdate(ctx, inspect)
}

func (s *Service) PerformUpdate(ctx context.Context, oldInspect container.InspectResponse) (err error) {
	s.setUpdating(true, "Initializing update...", "")
	defer func() {
		if err != nil {
			s.setUpdating(false, "", err.Error())
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
	return nil
}
