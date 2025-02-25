package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

// isPortAvailable verifies if a port is available for use
func isPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

func RunChain() {
	log := logger.Get()

	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Get or generate device ID
	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to verify device ID")
	}

	// Proxy request to the server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get the path and ensure it starts with /api
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api") {
			path = "/api" + path
		}

		// Create new request to forward to the server
		targetURL := fmt.Sprintf("%s%s", cfg.Runner.ServerURL, path)
		log.Debug().
			Str("method", r.Method).
			Str("path", path).
			Str("target_url", targetURL).
			Str("device_id", deviceID).
			Msg("Forwarding request")

		var proxyReq *http.Request
		var err error

		// Only modify body for POST/PUT requests with JSON content
		if (r.Method == "POST" || r.Method == "PUT") && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
				return
			}
			r.Body.Close()

			// Try to decode and modify JSON body
			var requestData map[string]interface{}
			if err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&requestData); err != nil {
				log.Error().Err(err).Msg("Failed to decode request body")
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Add device ID to request body
			requestData["creator_device_id"] = deviceID

			// Marshal modified body
			modifiedBody, err := json.Marshal(requestData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal modified request body")
				http.Error(w, "Error processing request", http.StatusInternalServerError)
				return
			}

			proxyReq, err = http.NewRequest(r.Method, targetURL, bytes.NewBuffer(modifiedBody))
			if err != nil {
				http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
				return
			}
		} else {
			// For other requests, forward the body as-is
			proxyReq, err = http.NewRequest(r.Method, targetURL, r.Body)
			if err != nil {
				http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
				return
			}
		}

		// Copy headers
		for header, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(header, value)
			}
		}

		// Always add device ID header
		proxyReq.Header.Set("X-Device-ID", deviceID)

		// Forward the request
		client := &http.Client{}
		resp, err := client.Do(proxyReq)
		if err != nil {
			log.Error().Err(err).Msg("Error forwarding request")
			http.Error(w, "Error forwarding request", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for header, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(header, value)
			}
		}

		// Set response status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Error().Err(err).Msg("Failed to copy response body")
		}
	})

	// Start local proxy server
	localAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, "3000")

	// Check if port 3000 is available
	if err := isPortAvailable(3000); err != nil {
		log.Fatal().Err(err).Int("port", 3000).Msg("Chain proxy port is not available")
	}

	log.Info().
		Str("address", localAddr).
		Str("device_id", deviceID).
		Msg("Starting chain proxy server")

	if err := http.ListenAndServe(localAddr, nil); err != nil {
		log.Fatal().Err(err).Msg("Failed to start chain proxy server")
	}
}
