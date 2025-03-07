package services

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type HeartbeatMonitor struct {
	taskService *TaskService
	heartbeats  map[string]time.Time
	mu          sync.RWMutex
	logger      zerolog.Logger
}

func NewHeartbeatMonitor(taskService *TaskService) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		taskService: taskService,
		heartbeats:  make(map[string]time.Time),
		logger:      logger.Get().With().Str("component", "heartbeat_monitor").Logger(),
	}
}

func (h *HeartbeatMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	h.logger.Info().Msg("Starting heartbeat monitor")

	for {
		select {
		case <-ctx.Done():
			h.logger.Info().Msg("Stopping heartbeat monitor")
			return
		case <-ticker.C:
			h.checkHeartbeats()
		}
	}
}

func (h *HeartbeatMonitor) RecordHeartbeat(webhookID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.heartbeats[webhookID] = time.Now()
	h.logger.Info().
		Str("webhook_id", webhookID).
		Msg("Recorded heartbeat")
}

func (h *HeartbeatMonitor) checkHeartbeats() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info().Int("total_webhooks", len(h.heartbeats)).Msg("Checking heartbeats")

	now := time.Now()
	staleThreshold := 2 * time.Minute

	for webhookID, lastHeartbeat := range h.heartbeats {
		h.logger.Info().Msgf("Checking heartbeat for webhook %s", webhookID)
		if now.Sub(lastHeartbeat) > staleThreshold {
			h.logger.Warn().
				Str("webhook_id", webhookID).
				Time("last_heartbeat", lastHeartbeat).
				Msg("Webhook connection appears to be stale")

			delete(h.heartbeats, webhookID)

			if h.taskService != nil {
				if err := h.taskService.HandleStaleWebhook(webhookID); err != nil {
					h.logger.Error().
						Err(err).
						Str("webhook_id", webhookID).
						Msg("Failed to handle stale webhook")
				}
			}
		}
		h.logger.Info().Msgf("Heartbeat for webhook %s is still valid", webhookID)
	}
}
