package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neo-code/internal/provider"
)

func TestDriverBuild(t *testing.T) {
	t.Parallel()

	driver := Driver()
	p, err := driver.Build(context.Background(), provider.RuntimeConfig{
		Driver:  DriverName,
		BaseURL: "https://api.anthropic.com/v1",
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestDriverDiscover(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("expected /models path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("expected anthropic x-api-key header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("expected anthropic-version header")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{{
				"id":   "claude-3-7-sonnet",
				"name": "Claude 3.7 Sonnet",
			}},
		})
	}))
	defer server.Close()

	driver := Driver()
	models, err := driver.Discover(context.Background(), provider.RuntimeConfig{
		Driver:                DriverName,
		BaseURL:               server.URL,
		APIKey:                "test-key",
		DiscoveryEndpointPath: "/models",
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "claude-3-7-sonnet" {
		t.Fatalf("unexpected models result: %+v", models)
	}
}

func TestDriverValidateCatalogIdentity(t *testing.T) {
	t.Parallel()

	driver := Driver()

	t.Run("valid identity", func(t *testing.T) {
		t.Parallel()

		err := driver.ValidateCatalogIdentity(provider.ProviderIdentity{
			Driver:                DriverName,
			DiscoveryEndpointPath: "/models",
		})
		if err != nil {
			t.Fatalf("expected valid identity, got %v", err)
		}
	})

	t.Run("invalid discovery endpoint path", func(t *testing.T) {
		t.Parallel()

		err := driver.ValidateCatalogIdentity(provider.ProviderIdentity{
			Driver:                DriverName,
			DiscoveryEndpointPath: "https://api.example.com/models",
		})
		if err == nil {
			t.Fatal("expected discovery config error")
		}
		if !provider.IsDiscoveryConfigError(err) {
			t.Fatalf("expected discovery config error, got %v", err)
		}
		if !strings.Contains(err.Error(), "must be a relative path") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}
