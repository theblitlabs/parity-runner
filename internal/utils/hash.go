package utils

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"strings"
)

func VerifyImageHash(imageName string) (string, error) {
	cmd := exec.Command("docker", "image", "inspect", "--format={{.Id}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect image: %w", err)
	}

	imageID := strings.TrimSpace(string(output))
	imageID = strings.TrimPrefix(imageID, "sha256:")

	return imageID, nil
}

func ComputeCommandHash(command []string) string {
	commandStr := strings.Join(command, " ")
	hash := sha256.Sum256([]byte(commandStr))
	return fmt.Sprintf("%x", hash)
}

func ComputeResultHash(stdout, stderr string, exitCode int) string {
	combined := fmt.Sprintf("%s%s%d", stdout, stderr, exitCode)
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

func LoadAndVerifyImage(imagePath string) (string, error) {
	cmd := exec.Command("docker", "load", "-i", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to load image: %w", err)
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				imageName := strings.TrimSpace(strings.Join(parts[1:], ":"))
				return imageName, nil
			}
		}
	}

	return "", fmt.Errorf("could not determine loaded image name")
}
