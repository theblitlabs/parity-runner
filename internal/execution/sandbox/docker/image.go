package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/theblitlabs/gologger"
)

// ImageManager handles Docker image operations
type ImageManager struct{}

// NewImageManager creates a new ImageManager
func NewImageManager() *ImageManager {
	return &ImageManager{}
}

// PullImage pulls a Docker image from a registry
func (im *ImageManager) PullImage(ctx context.Context, imageName string) error {
	log := gologger.WithComponent("docker.image")

	log.Info().Str("image", imageName).Msg("Pulling image from registry")
	if _, err := execCommand(ctx, "docker", "pull", imageName); err != nil {
		log.Error().Err(err).Str("image", imageName).Msg("Pull failed")
		return fmt.Errorf("image pull failed: %w", err)
	}

	return nil
}

// DownloadAndLoadImage downloads a Docker image from a URL and loads it
func (im *ImageManager) DownloadAndLoadImage(ctx context.Context, imageURL, imageName string) error {
	log := gologger.WithComponent("docker.image")

	log.Info().Str("url", imageURL).Msg("Downloading Docker image")

	// Download image from URL
	resp, err := http.Get(imageURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to download Docker image")
		return fmt.Errorf("failed to download Docker image: %w", err)
	}
	defer resp.Body.Close()

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "docker-image-*.tar")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create temporary file")
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			log.Debug().Err(err).Str("file", tmpFile.Name()).Msg("Failed to remove temporary file")
		}
	}()
	defer tmpFile.Close()

	// Copy image to temporary file
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		log.Error().Err(err).Msg("Failed to save Docker image")
		return fmt.Errorf("failed to save Docker image: %w", err)
	}

	// Load the Docker image
	log.Info().Str("image", imageName).Msg("Loading Docker image")
	if _, err := execCommand(ctx, "docker", "load", "-i", tmpFile.Name()); err != nil {
		log.Error().Err(err).Msg("Failed to load Docker image")
		return fmt.Errorf("failed to load Docker image: %w", err)
	}

	return nil
}

// EnsureImageAvailable makes sure an image is available, either by pulling it or downloading and loading it
func (im *ImageManager) EnsureImageAvailable(ctx context.Context, imageName, imageURL string) error {
	if imageURL != "" {
		return im.DownloadAndLoadImage(ctx, imageURL, imageName)
	}
	return im.PullImage(ctx, imageName)
}
