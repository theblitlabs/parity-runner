package ipfs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/theblitlabs/parity-protocol/internal/config"
	ipfsapi "github.com/theblitlabs/parity-protocol/pkg/ipfs"
)

// Client implements the ipfs.Client interface using HTTP API
type Client struct {
	apiURL string
}

// Ensure Client implements the ipfs.Client interface
var _ ipfsapi.Client = (*Client)(nil)

func NewClient(cfg *config.Config) *Client {
	return &Client{
		apiURL: cfg.IPFS.APIURL,
	}
}

// StoreJSON marshals data to JSON and stores it in IPFS
func (c *Client) StoreJSON(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	return c.StoreData(jsonData)
}

// StoreData stores raw data in IPFS
func (c *Client) StoreData(data []byte) (string, error) {
	url := fmt.Sprintf("%s/api/v0/add", c.apiURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "data")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("IPFS API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Hash string `json:"Hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Hash, nil
}

// GetData retrieves data from IPFS by CID
func (c *Client) GetData(cid string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v0/cat/%s", c.apiURL, cid)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("IPFS API error: %s - %s", resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GetJSON retrieves JSON data from IPFS and unmarshals it
func (c *Client) GetJSON(cid string, target interface{}) error {
	data, err := c.GetData(cid)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

// UploadFile implements the ipfs.Client interface
func (c *Client) UploadFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return c.StoreData(data)
}

// UploadData implements the ipfs.Client interface
func (c *Client) UploadData(data []byte) (string, error) {
	return c.StoreData(data)
}

// RetrieveFile implements the ipfs.Client interface
func (c *Client) RetrieveFile(cid, outputPath string) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	data, err := c.GetData(cid)
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

// RetrieveData implements the ipfs.Client interface
func (c *Client) RetrieveData(cid string) ([]byte, error) {
	return c.GetData(cid)
}

// RetrieveToWriter implements the ipfs.Client interface
func (c *Client) RetrieveToWriter(cid string, writer io.Writer) error {
	data, err := c.GetData(cid)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}
