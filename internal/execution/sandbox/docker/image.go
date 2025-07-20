package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker/executils"
)

type ImageManager struct{}

func NewImageManager() *ImageManager {
	return &ImageManager{}
}

func (im *ImageManager) PullImage(ctx context.Context, imageName string) error {
	log := gologger.WithComponent("docker.image")

	log.Info().Str("image", imageName).Msg("Pulling image from registry")
	if _, err := executils.ExecCommand(ctx, "docker", "pull", imageName); err != nil {
		log.Error().Err(err).Str("image", imageName).Msg("Pull failed")
		return fmt.Errorf("image pull failed: %w", err)
	}

	return nil
}

func (im *ImageManager) DownloadAndLoadImage(ctx context.Context, imageURL, imageName string) error {
	log := gologger.WithComponent("docker.image")

	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		log.Error().Err(err).Str("url", imageURL).Msg("Failed to parse image URL")
		return fmt.Errorf("failed to parse image URL: %w", err)
	}

	if strings.Contains(parsedURL.Path, "/ipfs/") {
		log.Info().Str("url", imageURL).Msg("Downloading Docker image from IPFS")
	} else {
		log.Info().Str("url", imageURL).Msg("Downloading Docker image from HTTP")
	}

	// For IPFS URLs, use the API endpoint directly to avoid gateway redirect issues
	if strings.Contains(imageURL, "/ipfs/") {
		// Extract CID from URL like http://localhost:8080/ipfs/QmXXX
		parts := strings.Split(imageURL, "/ipfs/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid IPFS URL format: %s", imageURL)
		}

		cid := strings.Split(parts[1], "?")[0] // Remove any query parameters

		// Use IPFS API directly: http://localhost:5001/api/v0/cat?arg=CID
		apiURL := "http://localhost:5001/api/v0/cat?arg=" + cid
		log.Info().Str("api_url", apiURL).Str("cid", cid).Msg("Using IPFS API to download image")

		req, err := http.NewRequest("POST", apiURL, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create IPFS API request")
			return fmt.Errorf("failed to create IPFS API request: %w", err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Error().Err(err).Msg("Failed to download Docker image")
			return fmt.Errorf("failed to download Docker image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Error().Int("status_code", resp.StatusCode).Msg("Failed to download Docker image")
			return fmt.Errorf("failed to download Docker image: status code %d", resp.StatusCode)
		}

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

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			log.Error().Err(err).Msg("Failed to save Docker image")
			return fmt.Errorf("failed to save Docker image: %w", err)
		}

		log.Info().Str("image", imageName).Msg("Loading Docker image")
		if _, err := executils.ExecCommand(context.Background(), "docker", "load", "-i", tmpFile.Name()); err != nil {
			log.Error().Err(err).Msg("Failed to load Docker image")
			return fmt.Errorf("failed to load Docker image: %w", err)
		}

		return nil
	}

	// For non-IPFS URLs, use regular HTTP GET
	log.Info().Str("url", imageURL).Msg("Downloading Docker image from HTTP")

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create HTTP request")
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", "parity-runner/1.0")
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to download Docker image")
		return fmt.Errorf("failed to download Docker image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Msg("Failed to download Docker image")
		return fmt.Errorf("failed to download Docker image: status code %d", resp.StatusCode)
	}

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

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		log.Error().Err(err).Msg("Failed to save Docker image")
		return fmt.Errorf("failed to save Docker image: %w", err)
	}

	log.Info().Str("image", imageName).Msg("Loading Docker image")
	if _, err := executils.ExecCommand(ctx, "docker", "load", "-i", tmpFile.Name()); err != nil {
		log.Error().Err(err).Msg("Failed to load Docker image")
		return fmt.Errorf("failed to load Docker image: %w", err)
	}

	return nil
}

func (im *ImageManager) EnsureImageAvailable(ctx context.Context, imageName, imageURL string) error {
	if imageURL != "" {
		return im.DownloadAndLoadImage(ctx, imageURL, imageName)
	}
	return im.PullImage(ctx, imageName)
}
