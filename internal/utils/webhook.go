package utils

import (
	"fmt"

	"github.com/theblitlabs/parity-runner/internal/tunnel"
)

var tunnelClient *tunnel.TunnelClient

func GetWebhookURL() string {
	cfg, err := GetConfig()
	if err != nil {
		return ""
	}

	// If tunnel is enabled and running, use tunnel URL
	if cfg.Runner.Tunnel.Enabled && tunnelClient != nil && tunnelClient.IsRunning() {
		return tunnelClient.GetPublicURL()
	}

	// Fallback to local URL
	webhookUrl := fmt.Sprintf("http://localhost:%d/webhook", cfg.Runner.WebhookPort)
	return webhookUrl
}

func SetTunnelClient(client *tunnel.TunnelClient) {
	tunnelClient = client
}
