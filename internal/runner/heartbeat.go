package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/models"
)

// HeartbeatConfig contains configuration for the heartbeat service
type HeartbeatConfig struct {
	ServerURL     string
	DeviceID      string
	WalletAddress string
	BaseInterval  time.Duration
	MaxBackoff    time.Duration
	BaseBackoff   time.Duration
	MaxRetries    int
}

// HeartbeatService manages periodic heartbeat signals to the server
type HeartbeatService struct {
	config          HeartbeatConfig
	ticker          *time.Ticker
	stopChan        chan struct{}
	mu              sync.Mutex
	started         bool
	startTime       time.Time
	statusProvider  StatusProvider
	metricsProvider MetricsProvider
}

// StatusProvider interface for getting runner status
type StatusProvider interface {
	IsProcessing() bool
}

// MetricsProvider interface for getting system metrics
type MetricsProvider interface {
	GetSystemMetrics() (memory int64, cpu float64)
}

// NewHeartbeatService creates a new heartbeat service
func NewHeartbeatService(config HeartbeatConfig, statusProvider StatusProvider, metricsProvider MetricsProvider) *HeartbeatService {
	return &HeartbeatService{
		config:          config,
		stopChan:        make(chan struct{}),
		startTime:       time.Now(),
		statusProvider:  statusProvider,
		metricsProvider: metricsProvider,
	}
}

// Start begins the heartbeat service
func (h *HeartbeatService) Start() error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}

	h.ticker = time.NewTicker(h.config.BaseInterval)
	h.started = true
	h.mu.Unlock()

	log := gologger.WithComponent("heartbeat")
	log.Info().
		Dur("interval", h.config.BaseInterval).
		Str("device_id", h.config.DeviceID).
		Msg("Starting heartbeat service")

	// Send initial heartbeat
	if err := h.sendHeartbeatWithRetry(); err != nil {
		log.Error().Err(err).Msg("Failed to send initial heartbeat after retries")
	} else {
		log.Info().Msg("Initial heartbeat sent successfully")
	}

	// Start heartbeat loop in a single goroutine
	go func() {
		consecutiveFailures := 0
		for {
			select {
			case <-h.ticker.C:
				if err := h.sendHeartbeatWithRetry(); err != nil {
					consecutiveFailures++
					backoff := time.Duration(float64(h.config.BaseBackoff) * float64(consecutiveFailures))
					if backoff > h.config.MaxBackoff {
						backoff = h.config.MaxBackoff
					}
					log.Warn().
						Err(err).
						Int("consecutive_failures", consecutiveFailures).
						Dur("next_retry", backoff).
						Msg("Heartbeat failed, will retry with backoff")

					h.mu.Lock()
					h.ticker.Reset(backoff)
					h.mu.Unlock()
				} else {
					consecutiveFailures = 0
					h.mu.Lock()
					h.ticker.Reset(h.config.BaseInterval)
					h.mu.Unlock()
				}
			case <-h.stopChan:
				log.Info().Msg("Stopping heartbeat service")
				return
			}
		}
	}()

	return nil
}

// sendHeartbeatWithRetry attempts to send a heartbeat with retries
func (h *HeartbeatService) sendHeartbeatWithRetry() error {
	var lastErr error
	for attempt := 1; attempt <= h.config.MaxRetries; attempt++ {
		if err := h.sendHeartbeat(); err != nil {
			lastErr = err
			if attempt < h.config.MaxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", h.config.MaxRetries, lastErr)
}

// sendHeartbeat sends a single heartbeat to the server
func (h *HeartbeatService) sendHeartbeat() error {
	log := gologger.WithComponent("heartbeat")

	type HeartbeatPayload struct {
		DeviceID      string              `json:"device_id"`
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Timestamp     int64               `json:"timestamp"`
		Uptime        int64               `json:"uptime"`
		Memory        int64               `json:"memory_usage"`
		CPU           float64             `json:"cpu_usage"`
	}

	status := models.RunnerStatusOnline
	if h.statusProvider.IsProcessing() {
		status = models.RunnerStatusBusy
	}

	memory, cpu := h.metricsProvider.GetSystemMetrics()

	payload := HeartbeatPayload{
		DeviceID:      h.config.DeviceID,
		WalletAddress: h.config.WalletAddress,
		Status:        status,
		Timestamp:     time.Now().Unix(),
		Uptime:        int64(time.Since(h.startTime).Seconds()),
		Memory:        memory,
		CPU:           cpu,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat payload: %w", err)
	}

	message := struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}{
		Type:    "heartbeat",
		Payload: payloadBytes,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat message: %w", err)
	}

	heartbeatURL := fmt.Sprintf("%s/api/runners/heartbeat", h.config.ServerURL)
	req, err := http.NewRequest("POST", heartbeatURL, bytes.NewBuffer(messageBytes))
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ParityRunner/1.0")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Debug().
		Str("device_id", h.config.DeviceID).
		Str("status", string(status)).
		Float64("cpu", cpu).
		Int64("memory", memory).
		Msg("Heartbeat sent successfully")

	return nil
}

// Stop stops the heartbeat service
func (h *HeartbeatService) Stop() {
	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return
	}

	log := gologger.WithComponent("heartbeat")
	log.Info().Msg("Stopping heartbeat service...")

	// Signal stop to the heartbeat loop
	close(h.stopChan)

	// Stop and cleanup ticker
	if h.ticker != nil {
		h.ticker.Stop()
		h.ticker = nil
	}

	// Reset state
	h.started = false
	h.stopChan = make(chan struct{})
	h.mu.Unlock()

	// Give a moment for the goroutine to exit
	time.Sleep(100 * time.Millisecond)

	log.Info().Msg("Heartbeat service stopped successfully")
}

// SetInterval updates the base interval for heartbeats
func (h *HeartbeatService) SetInterval(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config.BaseInterval = interval
	if h.ticker != nil {
		h.ticker.Reset(interval)
	}
}
