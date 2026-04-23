package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type DockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
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

func NewService(target string) (*Service, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
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
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
	}
	return s.cli.ContainerLogs(ctx, s.target, options)
}

func (s *Service) PullImage(ctx context.Context, imageRef string) error {
	reader, err := s.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader) // Wait for pull to complete
	return nil
}

func (s *Service) CheckAndUpdate(ctx context.Context) error {
	s.mu.Lock()
	if s.updateStatus.IsUpdating || s.updateStatus.IsChecking {
		s.mu.Unlock()
		return nil
	}
	s.updateStatus.IsChecking = true
	s.updateStatus.Error = ""
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.updateStatus.IsChecking = false
		s.updateStatus.LastCheck = time.Now()
		s.mu.Unlock()
	}()

	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return err
	}

	imageRef := inspect.Config.Image
	oldImageID := inspect.Image

	log.Printf("Checking for updates for image %s (current ID: %s)", imageRef, oldImageID)

	if err := s.PullImage(ctx, imageRef); err != nil {
		s.mu.Lock()
		s.updateStatus.Error = fmt.Sprintf("Failed to pull image: %v", err)
		s.mu.Unlock()
		return err
	}

	// Get new image ID
	newImage, _, err := s.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		return err
	}

	if newImage.ID == oldImageID {
		log.Printf("No update available for %s", imageRef)
		return nil
	}

	log.Printf("New image version detected: %s -> %s", oldImageID, newImage.ID)
	return s.PerformUpdate(ctx, inspect)
}

func (s *Service) PerformUpdate(ctx context.Context, oldInspect types.ContainerJSON) error {
	s.mu.Lock()
	s.updateStatus.IsUpdating = true
	s.updateStatus.Progress = "Updating container..."
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.updateStatus.IsUpdating = false
		s.mu.Unlock()
	}()

	// Check if image ID changed
	_, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return err
	}
	
	// This won't work because the container is still using the old image.
	// We need to check the ID of the image by name.
	// But let's assume we want to recreate anyway to be safe, or just check the registry.
	
	// Let's just do the recreate. If the image is the same, it's just a restart.
	// But the user specifically asked: "if there a patches stop the container, docker pull the image, and restart with new image"
	
	log.Printf("Recreating container %s", s.target)
	
	// 1. Stop
	if err := s.Stop(ctx); err != nil {
		log.Printf("Warning: failed to stop container: %v", err)
	}
	
	// 2. Remove
	if err := s.cli.ContainerRemove(ctx, s.target, container.RemoveOptions{Force: true}); err != nil {
		return err
	}
	
	// 3. Create
	// We need to clear the ID and other fields that are specific to the old container
	config := oldInspect.Config
	hostConfig := oldInspect.HostConfig
	
	// Networks
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: oldInspect.NetworkSettings.Networks,
	}
	
	resp, err := s.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, s.target)
	if err != nil {
		return err
	}
	
	// 4. Start
	if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	
	log.Printf("Container %s updated and restarted", s.target)

	// 5. Cleanup old image
	log.Printf("Cleaning up old image %s", oldInspect.Image)
	_, err = s.cli.ImageRemove(ctx, oldInspect.Image, image.RemoveOptions{PruneChildren: true})
	if err != nil {
		log.Printf("Warning: failed to remove old image %s: %v", oldInspect.Image, err)
	}

	return nil
}
