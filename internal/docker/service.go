package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Image  string `json:"image"`
}

type Service struct {
	cli    *client.Client
	target string
}

func NewService(target string) (*Service, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Service{cli: cli, target: target}, nil
}

func (s *Service) GetStatus(ctx context.Context) (*ContainerInfo, error) {
	inspect, err := s.cli.ContainerInspect(ctx, s.target)
	if err != nil {
		return nil, err
	}
	return &ContainerInfo{
		ID:     inspect.ID[:12],
		Status: inspect.State.Status,
		Image:  inspect.Config.Image,
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
