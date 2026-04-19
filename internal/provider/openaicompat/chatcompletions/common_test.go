package chatcompletions

import (
	"net/http"
	"testing"

	"neo-code/internal/provider"
)

func TestApplyAuthHeaders(t *testing.T) {
	t.Parallel()

	t.Run("openaicompat uses bearer", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		applyAuthHeaders(header, provider.RuntimeConfig{
			Driver: provider.DriverOpenAICompat,
			APIKey: "x-key",
		})
		if got := header.Get("Authorization"); got != "Bearer x-key" {
			t.Fatalf("expected bearer authorization header, got %q", got)
		}
	})

	t.Run("anthropic default version", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		applyAuthHeaders(header, provider.RuntimeConfig{
			Driver: provider.DriverAnthropic,
			APIKey: "anthropic-key",
		})
		if got := header.Get("x-api-key"); got != "anthropic-key" {
			t.Fatalf("expected anthropic x-api-key, got %q", got)
		}
		if got := header.Get("anthropic-version"); got == "" {
			t.Fatal("expected default anthropic version")
		}
	})
}
