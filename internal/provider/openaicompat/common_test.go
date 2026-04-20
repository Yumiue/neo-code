package openaicompat

import (
	"testing"

	"neo-code/internal/provider"
)

func TestValidateRuntimeConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty base url", func(t *testing.T) {
		t.Parallel()
		err := validateRuntimeConfig(provider.RuntimeConfig{
			BaseURL: "",
			APIKey:  "test-key",
		})
		if err == nil || err.Error() != errorPrefix+"base url is empty" {
			t.Fatalf("expected base url error, got %v", err)
		}
	})

	t.Run("empty api key", func(t *testing.T) {
		t.Parallel()
		err := validateRuntimeConfig(provider.RuntimeConfig{
			BaseURL: "https://api.example.com/v1",
			APIKey:  "   ",
		})
		if err == nil || err.Error() != errorPrefix+"api key is empty" {
			t.Fatalf("expected api key error, got %v", err)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		err := validateRuntimeConfig(provider.RuntimeConfig{
			BaseURL: " https://api.example.com/v1 ",
			APIKey:  " test-key ",
		})
		if err != nil {
			t.Fatalf("expected valid config, got %v", err)
		}
	})
}
