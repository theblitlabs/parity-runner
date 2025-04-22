
package utils

import (
	"fmt"
)

func GetWebhookURL() string {
	cfg, err := GetConfig()
	if err != nil {
		return ""
	}

	publicIP, err := GetPublicIP()
	if err != nil {
		return ""
	}

	webhookUrl := fmt.Sprintf("http://%s:%d/webhook", publicIP, cfg.Runner.WebhookPort)

	return webhookUrl
}
