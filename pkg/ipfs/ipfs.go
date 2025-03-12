package ipfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

// Client defines the interface for IPFS operations
type Client interface {
	// Upload methods
	UploadFile(filePath string) (string, error)
	UploadData(data []byte) (string, error)

	// Retrieve methods
	RetrieveFile(cid, outputPath string) error
	RetrieveData(cid string) ([]byte, error)
	RetrieveToWriter(cid string, writer io.Writer) error
}

// Service represents the IPFS service implementation
type Service struct {
	shell *shell.Shell
}

// Config represents the IPFS service configuration
type Config struct {
	APIEndpoint string // IPFS API endpoint (e.g., "localhost:5001")
	UDPBuffer   struct {
		ReceiveSize string // UDP receive buffer size (e.g., "7168KB")
		SendSize    string // UDP send buffer size (e.g., "7168KB")
	}
}

// New creates a new IPFS service instance
func New(config Config) (*Service, error) {
	if config.APIEndpoint == "" {
		config.APIEndpoint = "localhost:5001" // Default IPFS API endpoint
	}

	// Set default UDP buffer sizes if not specified
	if config.UDPBuffer.ReceiveSize == "" {
		config.UDPBuffer.ReceiveSize = "7168KB"
	}
	if config.UDPBuffer.SendSize == "" {
		config.UDPBuffer.SendSize = "7168KB"
	}

	// Set UDP buffer sizes using sysctl
	if err := setUDPBufferSizes(config.UDPBuffer.ReceiveSize, config.UDPBuffer.SendSize); err != nil {
		return nil, fmt.Errorf("failed to set UDP buffer sizes: %w", err)
	}

	sh := shell.NewShell(config.APIEndpoint)

	// Test connection
	if _, err := sh.ID(); err != nil {
		return nil, fmt.Errorf("failed to connect to IPFS node: %w", err)
	}

	return &Service{shell: sh}, nil
}

// setUDPBufferSizes attempts to set the system's UDP buffer sizes
func setUDPBufferSizes(receiveSize, sendSize string) error {
	// Convert sizes to bytes
	recvBytes, err := parseSize(receiveSize)
	if err != nil {
		return fmt.Errorf("invalid receive size: %w", err)
	}

	sendBytes, err := parseSize(sendSize)
	if err != nil {
		return fmt.Errorf("invalid send size: %w", err)
	}

	// Run sysctl commands to set buffer sizes
	commands := []struct {
		param string
		value int
	}{
		{"net.core.rmem_max", recvBytes},
		{"net.core.wmem_max", sendBytes},
		{"net.core.rmem_default", recvBytes},
		{"net.core.wmem_default", sendBytes},
	}

	for _, cmd := range commands {
		if err := runSysctl(cmd.param, cmd.value); err != nil {
			return fmt.Errorf("failed to set %s: %w", cmd.param, err)
		}
	}

	return nil
}

// parseSize converts a size string (e.g., "7168KB") to bytes
func parseSize(size string) (int, error) {
	var value int
	var unit string
	if _, err := fmt.Sscanf(size, "%d%s", &value, &unit); err != nil {
		return 0, fmt.Errorf("invalid size format: %s", size)
	}

	var multiplier int
	switch unit {
	case "KB", "kb":
		multiplier = 1024
	case "MB", "mb":
		multiplier = 1024 * 1024
	case "GB", "gb":
		multiplier = 1024 * 1024 * 1024
	default:
		multiplier = 1
	}

	return value * multiplier, nil
}

// runSysctl attempts to set a sysctl parameter
func runSysctl(param string, value int) error {
	cmd := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%d", param, value))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sysctl failed: %s: %w", string(output), err)
	}
	return nil
}

// verifyContentAvailability checks if the content with given CID is available on the IPFS network
func (s *Service) verifyContentAvailability(cid string, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		// Check if we can actually retrieve the content
		reader, err := s.shell.Cat(cid)
		if err != nil {
			done <- fmt.Errorf("content not retrievable: %w", err)
			return
		}
		defer reader.Close()

		// Try reading a small amount of data to verify it's actually available
		buf := make([]byte, 1024)
		_, err = reader.Read(buf)
		if err != nil && err != io.EOF {
			done <- fmt.Errorf("content not readable: %w", err)
			return
		}

		// Ensure the content is pinned locally
		err = s.shell.Pin(cid)
		if err != nil {
			done <- fmt.Errorf("content not pinnable: %w", err)
			return
		}

		// Verify pin status
		pins, err := s.shell.Pins()
		if err != nil {
			done <- fmt.Errorf("failed to verify pin status: %w", err)
			return
		}

		_, isPinned := pins[cid]
		if !isPinned {
			done <- fmt.Errorf("content uploaded but not pinned")
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("operation timed out after %v", timeout)
	}
}

// UploadFile uploads a file to IPFS, pins it, returns its CID, and verifies its availability
func (s *Service) UploadFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	cid, err := s.shell.Add(file, shell.Pin(true), shell.CidVersion(1))
	if err != nil {
		return "", fmt.Errorf("failed to upload file to IPFS: %w", err)
	}

	// Verify content availability and pin with a timeout
	if err := s.verifyContentAvailability(cid, 30*time.Second); err != nil {
		return cid, fmt.Errorf("content uploaded but availability or pinning uncertain: %w", err)
	}

	return cid, nil
}

// UploadData uploads raw data to IPFS, pins it, returns its CID, and verifies its availability
func (s *Service) UploadData(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	cid, err := s.shell.Add(reader, shell.Pin(true), shell.CidVersion(1))
	if err != nil {
		return "", fmt.Errorf("failed to upload data to IPFS: %w", err)
	}

	// Verify content availability and pin with a timeout
	if err := s.verifyContentAvailability(cid, 30*time.Second); err != nil {
		return cid, fmt.Errorf("content uploaded but availability or pinning uncertain: %w", err)
	}

	return cid, nil
}

// RetrieveFile downloads a file from IPFS using its CID and saves it to the specified path
func (s *Service) RetrieveFile(cid, outputPath string) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	return s.RetrieveToWriter(cid, file)
}

// RetrieveData retrieves data from IPFS using its CID and returns it as a byte slice
func (s *Service) RetrieveData(cid string) ([]byte, error) {
	reader, err := s.shell.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve data from IPFS: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from IPFS stream: %w", err)
	}

	return data, nil
}

// RetrieveToWriter retrieves data from IPFS and writes it to the provided writer
func (s *Service) RetrieveToWriter(cid string, writer io.Writer) error {
	reader, err := s.shell.Cat(cid)
	if err != nil {
		return fmt.Errorf("failed to retrieve data from IPFS: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("failed to copy data from IPFS stream: %w", err)
	}
	return nil
}
