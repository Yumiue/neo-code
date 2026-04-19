package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

func TestDriverClosuresAndSupportedProtocol(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "gpt-4.1", "name": "GPT-4.1"}},
		})
	}))
	defer server.Close()

	cfg := provider.RuntimeConfig{
		Name:         DriverName,
		Driver:       DriverName,
		BaseURL:      server.URL,
		DefaultModel: "gpt-4.1",
		APIKey:       "test-key",
	}
	driver := Driver()

	built, err := driver.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	typed, ok := built.(*Provider)
	if !ok || typed.client == nil || typed.client.Transport == nil {
		t.Fatalf("unexpected built provider: %T %+v", built, typed)
	}

	models, err := driver.Discover(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-4.1" {
		t.Fatalf("unexpected models: %+v", models)
	}

	if got, err := supportedChatProtocol(provider.RuntimeConfig{Driver: DriverName}); err != nil || got != provider.ChatProtocolOpenAIChatCompletions {
		t.Fatalf("expected default chat protocol, got protocol=%q err=%v", got, err)
	}
	if got, err := supportedChatProtocol(provider.RuntimeConfig{
		Driver:       DriverName,
		ChatProtocol: provider.ChatProtocolOpenAIResponses,
	}); err != nil || got != provider.ChatProtocolOpenAIResponses {
		t.Fatalf("expected explicit responses chat protocol, got protocol=%q err=%v", got, err)
	}
	if got, err := supportedChatProtocol(provider.RuntimeConfig{
		Driver:           DriverName,
		ChatEndpointPath: "/responses",
	}); err != nil || got != provider.ChatProtocolOpenAIResponses {
		t.Fatalf("expected endpoint inferred responses protocol, got protocol=%q err=%v", got, err)
	}
	if _, err := supportedChatProtocol(provider.RuntimeConfig{Driver: provider.DriverAnthropic}); err == nil ||
		!strings.Contains(err.Error(), "unsupported chat protocol") {
		t.Fatalf("expected unsupported anthropic protocol error, got %v", err)
	}
}

func TestFetchModelsAndGenerateExtraBranches(t *testing.T) {
	t.Parallel()

	p := &Provider{
		cfg: provider.RuntimeConfig{
			Name:    DriverName,
			Driver:  DriverName,
			BaseURL: "://bad",
			APIKey:  "test-key",
		},
		client: &http.Client{},
	}
	if _, err := discoverRawModels(context.Background(), p); err == nil || !strings.Contains(err.Error(), "build models request") {
		t.Fatalf("expected build models request error, got %v", err)
	}

	p = &Provider{
		cfg: provider.RuntimeConfig{
			Name:                  DriverName,
			Driver:                DriverName,
			BaseURL:               "https://api.example.com/v1",
			APIKey:                "test-key",
			DiscoveryEndpointPath: "https://api.example.com/models",
		},
		client: &http.Client{},
	}
	if _, err := discoverRawModels(context.Background(), p); err == nil || !provider.IsDiscoveryConfigError(err) {
		t.Fatalf("expected discovery config error, got %v", err)
	}

	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	p = &Provider{
		cfg: provider.RuntimeConfig{
			Name:    DriverName,
			Driver:  DriverName,
			BaseURL: server.URL,
			APIKey:  "   ",
		},
		client: server.Client(),
	}
	if _, err := discoverRawModels(context.Background(), p); err != nil {
		t.Fatalf("discoverRawModels() error = %v", err)
	}
	if auth != "" {
		t.Fatalf("expected no authorization header, got %q", auth)
	}

	p, err := New(provider.RuntimeConfig{
		Name:         DriverName,
		Driver:       provider.DriverAnthropic,
		BaseURL:      "https://api.example.com/v1",
		DefaultModel: "gpt-4.1",
		APIKey:       "test-key",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = p.Generate(context.Background(), providertypes.GenerateRequest{
		Messages: []providertypes.Message{{Role: providertypes.RoleUser, Parts: []providertypes.ContentPart{providertypes.NewTextPart("hello")}}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported chat protocol") {
		t.Fatalf("expected unsupported chat protocol error, got %v", err)
	}
}

func TestValidateCatalogIdentityRejectsInvalidDiscoverySettings(t *testing.T) {
	t.Parallel()

	err := validateCatalogIdentity(provider.ProviderIdentity{
		Driver:                DriverName,
		BaseURL:               "https://api.example.com/v1",
		DiscoveryEndpointPath: "https://api.example.com/models",
	})
	if err == nil || !provider.IsDiscoveryConfigError(err) {
		t.Fatalf("expected discovery config error for endpoint path, got %v", err)
	}

	err = validateCatalogIdentity(provider.ProviderIdentity{
		Driver:          DriverName,
		BaseURL:         "https://api.example.com/v1",
		ResponseProfile: "unsupported",
	})
	if err == nil || !provider.IsDiscoveryConfigError(err) {
		t.Fatalf("expected discovery config error for response profile, got %v", err)
	}
}
