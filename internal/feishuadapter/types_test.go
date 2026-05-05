package feishuadapter

import (
	"testing"
	"time"
)

func TestConfigValidateWebhookRequiresListenAndVerifyFields(t *testing.T) {
	cfg := Config{
		IngressMode:         IngressModeWebhook,
		AppID:               "app",
		AppSecret:           "secret",
		RequestTimeout:      3 * time.Second,
		IdempotencyTTL:      time.Minute,
		ReconnectBackoffMin: time.Second,
		ReconnectBackoffMax: 2 * time.Second,
		RebindInterval:      3 * time.Second,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected webhook mode validation error")
	}
}

func TestConfigValidateSDKDoesNotRequireWebhookFields(t *testing.T) {
	cfg := Config{
		IngressMode:         IngressModeSDK,
		AppID:               "app",
		AppSecret:           "secret",
		RequestTimeout:      3 * time.Second,
		IdempotencyTTL:      time.Minute,
		ReconnectBackoffMin: time.Second,
		ReconnectBackoffMax: 2 * time.Second,
		RebindInterval:      3 * time.Second,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate sdk mode: %v", err)
	}
}
