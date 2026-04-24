package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDiscordNotifier(t *testing.T) {
	n := NewDiscordNotifier("")
	if n != nil {
		t.Errorf("Expected nil when URL is empty")
	}

	n = NewDiscordNotifier("http://example.com")
	if n == nil || n.WebhookURL != "http://example.com" {
		t.Errorf("Expected DiscordNotifier with given URL")
	}
}

func TestNotify(t *testing.T) {
	// Test nil notifier
	var n *DiscordNotifier
	err := n.Notify("test")
	if err != nil {
		t.Errorf("Expected nil error for nil notifier, got %v", err)
	}

	// Test empty URL
	n = &DiscordNotifier{}
	err = n.Notify("test")
	if err != nil {
		t.Errorf("Expected nil error for empty URL, got %v", err)
	}

	// Test successful notification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			t.Errorf("Error decoding payload: %v", err)
		}
		embeds, ok := payload["embeds"].([]interface{})
		if !ok || len(embeds) != 1 {
			t.Errorf("Expected 1 embed, got %v", embeds)
		}
		embed := embeds[0].(map[string]interface{})
		if embed["description"] != "test message" {
			t.Errorf("Expected description 'test message', got '%v'", embed["description"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n = NewDiscordNotifier(server.URL)
	err = n.Notify("test message")
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	// Test HTTP error
	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverError.Close()

	n = NewDiscordNotifier(serverError.URL)
	err = n.Notify("test message")
	if err == nil {
		t.Errorf("Expected error for 500 status code")
	}

	// Test connection error
	n = NewDiscordNotifier("http://invalid-url-that-does-not-exist.local")
	err = n.Notify("test message")
	if err == nil {
		t.Errorf("Expected error for connection failure")
	}
}
