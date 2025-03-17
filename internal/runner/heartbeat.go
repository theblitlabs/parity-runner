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

	"github.com/go-co-op/gocron"
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

type HeartbeatService struct {
	config              HeartbeatConfig
	scheduler           *gocron.Scheduler
	mu                  sync.Mutex
	started             bool
	startTime           time.Time
	statusProvider      StatusProvider
	metricsProvider     MetricsProvider
	job                 *gocron.Job
	consecutiveFailures int
}

type StatusProvider interface {
	IsProcessing() bool
}

type MetricsProvider interface {
	GetSystemMetrics() (memory int64, cpu float64)
}

func NewHeartbeatService(config HeartbeatConfig, statusProvider StatusProvider, metricsProvider MetricsProvider) *HeartbeatService {
	return &HeartbeatService{
		config:              config,
		scheduler:           gocron.NewScheduler(time.UTC),
		startTime:           time.Now(),
		statusProvider:      statusProvider,
		metricsProvider:     metricsProvider,
		consecutiveFailures: 0,
	}
}

func (h *HeartbeatService) Start() error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	h.started = true
	h.mu.Unlock()

	log := gologger.WithComponent("heartbeat")
	log.Info().
		Dur("interval", h.config.BaseInterval).
		Str("device_id", h.config.DeviceID).
		Msg("Starting heartbeat service")

	if err := h.sendHeartbeatWithRetry(); err != nil {
		log.Error().Err(err).Msg("Failed to send initial heartbeat after retries")
	} else {
		log.Info().Msg("Initial heartbeat sent successfully")
	}

	job, err := h.scheduler.Every(h.config.BaseInterval).Do(h.heartbeatTask)

	if err != nil {
		return fmt.Errorf("failed to schedule heartbeat job: %w", err)
	}

	h.job = job
	h.scheduler.StartAsync()

	return nil
}

func (h *HeartbeatService) heartbeatTask() {
	log := gologger.WithComponent("heartbeat")

	if err := h.sendHeartbeatWithRetry(); err != nil {
		h.mu.Lock()
		h.consecutiveFailures++
		backoff := time.Duration(float64(h.config.BaseBackoff) * float64(h.consecutiveFailures))
		if backoff > h.config.MaxBackoff {
			backoff = h.config.MaxBackoff
		}
		h.mu.Unlock()

		log.Warn().
			Err(err).
			Int("consecutive_failures", h.consecutiveFailures).
			Dur("next_retry", backoff).
			Msg("Heartbeat failed, will retry with backoff")

		h.mu.Lock()
		if h.job != nil && backoff != h.config.BaseInterval {
			h.scheduler.RemoveByReference(h.job)
			h.job, _ = h.scheduler.Every(backoff).Do(h.heartbeatTask)
		}
		h.mu.Unlock()
	} else {
		h.mu.Lock()
		h.consecutiveFailures = 0

		if h.job != nil && len(h.scheduler.Jobs()) > 0 {
			nextRun := h.scheduler.Jobs()[0].NextRun()
			currentInterval := time.Until(nextRun)

			baseIntervalFloat := float64(h.config.BaseInterval)
			lowerBound := time.Duration(baseIntervalFloat * 0.9)
			upperBound := time.Duration(baseIntervalFloat * 1.1)

			if currentInterval < lowerBound || currentInterval > upperBound {
				h.scheduler.RemoveByReference(h.job)
				h.job, _ = h.scheduler.Every(h.config.BaseInterval).Do(h.heartbeatTask)
			}
		}
		h.mu.Unlock()
	}
}

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

func (h *HeartbeatService) Stop() {
	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return
	}

	log := gologger.WithComponent("heartbeat")
	log.Info().Msg("Stopping heartbeat service...")

	h.scheduler.Stop()

	h.started = false
	h.mu.Unlock()

	log.Info().Msg("Heartbeat service stopped successfully")
}

func (h *HeartbeatService) SetInterval(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config.BaseInterval = interval
	if h.started && h.job != nil {
		h.scheduler.RemoveByReference(h.job)
		h.job, _ = h.scheduler.Every(interval).Do(h.heartbeatTask)
	}
}
