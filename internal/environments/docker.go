package environments

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

type DockerEnvironment struct {
	client      *client.Client
	config      *DockerConfig
	containerId string
}

type DockerConfig struct {
	Image       string            `json:"image"`
	Command     []string          `json:"command"`
	Environment []string          `json:"env"`
	WorkDir     string            `json:"workdir"`
	Volumes     map[string]string `json:"volumes"`
}

func NewDockerEnvironment(config map[string]interface{}) (*DockerEnvironment, error) {

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Convert generic config to DockerConfig
	dockerConfig := &DockerConfig{
		Image:       config["image"].(string),
		WorkDir:     config["workdir"].(string),
		Environment: make([]string, 0),
	}

	// Convert environment variables
	if env, ok := config["env"].([]interface{}); ok {
		for _, v := range env {
			if str, ok := v.(string); ok {
				dockerConfig.Environment = append(dockerConfig.Environment, str)
			}
		}
	}

	return &DockerEnvironment{
		client: cli,
		config: dockerConfig,
	}, nil
}

func (d *DockerEnvironment) Setup() error {
	log := logger.Get()
	ctx := context.Background()

	log.Info().Str("image", d.config.Image).Msg("Pulling Docker image")
	reader, err := d.client.ImagePull(ctx, d.config.Image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull Docker image: %w", err)
	}
	defer reader.Close()

	// Wait for pull to complete
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	log.Info().Str("image", d.config.Image).Msg("Docker image pulled successfully")
	return nil
}

func (d *DockerEnvironment) Run(task *models.Task) error {
	ctx := context.Background()

	// Create container
	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image:      d.config.Image,
			Cmd:        d.config.Command,
			Env:        d.config.Environment,
			WorkingDir: d.config.WorkDir,
		},
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	d.containerId = resp.ID

	// Start container
	if err := d.client.ContainerStart(ctx, d.containerId, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

func (d *DockerEnvironment) Cleanup() error {
	if d.containerId != "" {
		ctx := context.Background()
		return d.client.ContainerRemove(ctx, d.containerId, types.ContainerRemoveOptions{
			Force: true,
		})
	}
	return nil
}

func (d *DockerEnvironment) GetType() string {
	return "docker"
}
