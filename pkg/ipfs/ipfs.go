package ipfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

const component = "ipfs"

// Service represents the IPFS service
type Service struct {
	shell *shell.Shell
}

// Config represents the IPFS service configuration
type Config struct {
	APIEndpoint string // IPFS API endpoint (e.g., "localhost:5001")
}

// New creates a new IPFS service instance
func New(config Config) (*Service, error) {
	if config.APIEndpoint == "" {
		config.APIEndpoint = "localhost:5001" // Default IPFS API endpoint
	}

	sh := shell.NewShell(config.APIEndpoint)

	// Test connection
	if _, err := sh.ID(); err != nil {
		return nil, fmt.Errorf("failed to connect to IPFS node: %w", err)
	}

	return &Service{shell: sh}, nil
}

// UploadFile uploads a file to IPFS and returns its CID
func (s *Service) UploadFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error(component, err, "Failed to open file")
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	cid, err := s.shell.Add(file)
	if err != nil {
		logger.Error(component, err, "Failed to upload file to IPFS")
		return "", fmt.Errorf("failed to upload file to IPFS: %w", err)
	}

	logger.Info(component, fmt.Sprintf("File uploaded successfully with CID: %s", cid))
	return cid, nil
}

// UploadData uploads raw data to IPFS and returns its CID
func (s *Service) UploadData(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	cid, err := s.shell.Add(reader)
	if err != nil {
		logger.Error(component, err, "Failed to upload data to IPFS")
		return "", fmt.Errorf("failed to upload data to IPFS: %w", err)
	}

	logger.Info(component, fmt.Sprintf("Data uploaded successfully with CID: %s", cid))
	return cid, nil
}

// RetrieveFile downloads a file from IPFS using its CID and saves it to the specified path
func (s *Service) RetrieveFile(cid, outputPath string) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		logger.Error(component, err, "Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		logger.Error(component, err, "Failed to create output file")
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	return s.RetrieveToWriter(cid, file)
}

// RetrieveData retrieves data from IPFS using its CID and returns it as a byte slice
func (s *Service) RetrieveData(cid string) ([]byte, error) {
	reader, err := s.shell.Cat(cid)
	if err != nil {
		logger.Error(component, err, fmt.Sprintf("Failed to retrieve data for CID: %s", cid))
		return nil, fmt.Errorf("failed to retrieve data from IPFS: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		logger.Error(component, err, "Failed to read data from IPFS stream")
		return nil, fmt.Errorf("failed to read data from IPFS stream: %w", err)
	}

	logger.Info(component, fmt.Sprintf("Data retrieved successfully for CID: %s", cid))
	return data, nil
}

// RetrieveToWriter retrieves data from IPFS and writes it to the provided writer
func (s *Service) RetrieveToWriter(cid string, writer io.Writer) error {
	reader, err := s.shell.Cat(cid)
	if err != nil {
		logger.Error(component, err, fmt.Sprintf("Failed to retrieve data for CID: %s", cid))
		return fmt.Errorf("failed to retrieve data from IPFS: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		logger.Error(component, err, "Failed to copy data from IPFS stream")
		return fmt.Errorf("failed to copy data from IPFS stream: %w", err)
	}

	logger.Info(component, fmt.Sprintf("Data retrieved successfully for CID: %s", cid))
	return nil
}
