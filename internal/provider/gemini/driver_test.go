package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"neo-code/internal/provider"
)

func TestDriverDiscover(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "gemini-2.5-flash"},
			},
		})
	}))
	defer server.Close()

	driver := Driver()
	models, err := driver.Discover(context.Background(), provider.RuntimeConfig{
		Driver:                DriverName,
		BaseURL:               server.URL,
		APIKey:                "test-key",
		DiscoveryProtocol:     provider.DiscoveryProtocolGeminiModels,
		ResponseProfile:       provider.DiscoveryResponseProfileGemini,
		DiscoveryEndpointPath: "/models",
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected one model, got %+v", models)
	}
}
