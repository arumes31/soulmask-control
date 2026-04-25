package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier interface {
	Notify(message string) error
}

type DiscordNotifier struct {
	WebhookURL string
}

func NewDiscordNotifier(url string) *DiscordNotifier {
	if url == "" {
		return nil
	}
	return &DiscordNotifier{WebhookURL: url}
}

func (d *DiscordNotifier) Notify(message string) error {
	if d == nil || d.WebhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"description": message,
				"color":       0x5865F2,
				"timestamp":   time.Now().Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord notification failed with status: %d", resp.StatusCode)
	}

	return nil
}
