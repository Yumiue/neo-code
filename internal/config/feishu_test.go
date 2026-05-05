package config

import "testing"

func TestFeishuConfigValidateDisabledAllowsEmpty(t *testing.T) {
	var cfg FeishuConfig
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate disabled feishu config: %v", err)
	}
}

func TestFeishuConfigValidateEnabledRequiresFields(t *testing.T) {
	cfg := FeishuConfig{Enabled: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for incomplete enabled config")
	}
}

func TestFeishuConfigApplyDefaults(t *testing.T) {
	var cfg FeishuConfig
	cfg.ApplyDefaults(FeishuConfig{
		Adapter: FeishuAdapterConfig{
			Listen:   DefaultFeishuAdapterListen,
			EventURI: DefaultFeishuAdapterEventPath,
			CardURI:  DefaultFeishuAdapterCardPath,
		},
		RequestTimeoutSec:    DefaultFeishuGatewayRequestTimeoutSec,
		IdempotencyTTLSec:    DefaultFeishuIdempotencyTTLSec,
		ReconnectBackoffMinM: DefaultFeishuReconnectBackoffMinMs,
		ReconnectBackoffMaxM: DefaultFeishuReconnectBackoffMaxMs,
		RebindIntervalSec:    DefaultFeishuRebindIntervalSec,
	})
	if cfg.Adapter.Listen == "" || cfg.Adapter.EventURI == "" || cfg.Adapter.CardURI == "" {
		t.Fatalf("adapter defaults not applied: %#v", cfg.Adapter)
	}
	if cfg.RequestTimeoutSec <= 0 || cfg.IdempotencyTTLSec <= 0 || cfg.RebindIntervalSec <= 0 {
		t.Fatalf("scalar defaults not applied: %#v", cfg)
	}
}
