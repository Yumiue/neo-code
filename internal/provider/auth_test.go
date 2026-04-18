package provider

import (
	"net/http"
	"testing"
)

func TestApplyAuthHeaders(t *testing.T) {
	t.Parallel()

	t.Run("bearer", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		ApplyAuthHeaders(header, AuthStrategyBearer, " test-key ", "")
		if got := header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer header, got %q", got)
		}
	})

	t.Run("x_api_key", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		ApplyAuthHeaders(header, AuthStrategyXAPIKey, "test-key", "")
		if got := header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("expected x-api-key header, got %q", got)
		}
	})

	t.Run("anthropic", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		ApplyAuthHeaders(header, AuthStrategyAnthropic, "test-key", "")
		if got := header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("expected anthropic x-api-key header, got %q", got)
		}
		if got := header.Get("anthropic-version"); got != defaultAnthropicAPIVersion {
			t.Fatalf("expected default anthropic version %q, got %q", defaultAnthropicAPIVersion, got)
		}
	})
}
