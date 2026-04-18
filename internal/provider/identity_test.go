package provider

import "testing"

func TestProviderIdentityKeyIncludesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		Driver:                   "openaicompat",
		BaseURL:                  "https://api.example.com/v1",
		APIStyle:                 "responses",
		DeploymentMode:           "ignored",
		APIVersion:               "ignored",
		DiscoveryEndpointPath:    "/v2/models",
		DiscoveryResponseProfile: "generic",
	}

	if got, want := identity.Key(), "openaicompat|https://api.example.com/v1|responses|ignored|ignored|/v2/models|generic"; got != want {
		t.Fatalf("expected identity key %q, got %q", want, got)
	}
}

func TestNormalizeProviderIdentityUsesDriverSpecificNormalization(t *testing.T) {
	t.Parallel()

	identity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                   " OpenAICompat ",
		BaseURL:                  "https://API.EXAMPLE.COM/v1/",
		APIStyle:                 " Responses ",
		DiscoveryEndpointPath:    " models ",
		DiscoveryResponseProfile: " Generic ",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() error = %v", err)
	}

	if identity.Driver != "openaicompat" {
		t.Fatalf("expected normalized driver %q, got %q", "openaicompat", identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.APIStyle != "responses" {
		t.Fatalf("expected normalized api_style %q, got %q", "responses", identity.APIStyle)
	}
	if identity.DiscoveryEndpointPath != "/models" {
		t.Fatalf("expected normalized discovery endpoint path %q, got %q", "/models", identity.DiscoveryEndpointPath)
	}
	if identity.DiscoveryResponseProfile != DiscoveryResponseProfileGeneric {
		t.Fatalf(
			"expected normalized discovery response profile %q, got %q",
			DiscoveryResponseProfileGeneric,
			identity.DiscoveryResponseProfile,
		)
	}
}

func TestNormalizeProviderIdentityPreservesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                   " Gemini ",
		BaseURL:                  "https://API.EXAMPLE.COM/v1/",
		DeploymentMode:           " Vertex ",
		DiscoveryEndpointPath:    "/models",
		DiscoveryResponseProfile: "gemini",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() error = %v", err)
	}

	if identity.Driver != "gemini" {
		t.Fatalf("expected normalized driver %q, got %q", "gemini", identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.DeploymentMode != "vertex" {
		t.Fatalf("expected normalized deployment_mode %q, got %q", "vertex", identity.DeploymentMode)
	}
	if identity.DiscoveryEndpointPath != "/models" || identity.DiscoveryResponseProfile != DiscoveryResponseProfileGemini {
		t.Fatalf("expected normalized discovery settings, got %+v", identity)
	}
}

func TestProviderIdentityStringMatchesKey(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		Driver:   "openaicompat",
		BaseURL:  "https://api.example.com/v1",
		APIStyle: "responses",
	}
	if identity.String() != identity.Key() {
		t.Fatalf("expected String() to match Key(), got %q vs %q", identity.String(), identity.Key())
	}
}

func TestNewProviderIdentityValidatesInputs(t *testing.T) {
	t.Parallel()

	identity, err := NewProviderIdentity(" OpenAICompat ", "https://API.EXAMPLE.COM/v1/")
	if err != nil {
		t.Fatalf("NewProviderIdentity() error = %v", err)
	}
	if identity.Driver != "openaicompat" || identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected identity: %+v", identity)
	}

	if _, err := NewProviderIdentity("   ", "https://api.example.com/v1"); err == nil {
		t.Fatalf("expected empty driver to fail")
	}
	if _, err := NewProviderIdentity("openaicompat", "not-a-url"); err == nil {
		t.Fatalf("expected invalid base URL to fail")
	}
}

func TestNormalizeProviderIdentityAnthropicAndUnknownDriver(t *testing.T) {
	t.Parallel()

	anthropicIdentity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:     " Anthropic ",
		BaseURL:    "https://API.EXAMPLE.COM/v1/",
		APIVersion: " 2023-06-01 ",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() anthropic error = %v", err)
	}
	if anthropicIdentity.Driver != "anthropic" {
		t.Fatalf("expected anthropic driver, got %+v", anthropicIdentity)
	}
	if anthropicIdentity.APIVersion != "2023-06-01" {
		t.Fatalf("expected normalized api version, got %+v", anthropicIdentity)
	}

	fallbackIdentity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                   " custom ",
		BaseURL:                  "https://API.EXAMPLE.COM/v1/",
		APIStyle:                 "responses",
		DeploymentMode:           "vertex",
		APIVersion:               "2023-06-01",
		DiscoveryEndpointPath:    "gateway/models",
		DiscoveryResponseProfile: "generic",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() fallback error = %v", err)
	}
	if fallbackIdentity.Driver != "custom" || fallbackIdentity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected fallback identity to normalize driver and base URL, got %+v", fallbackIdentity)
	}
	if fallbackIdentity.APIStyle != "" || fallbackIdentity.DeploymentMode != "" || fallbackIdentity.APIVersion != "" {
		t.Fatalf("expected fallback identity to drop protocol-specific fields, got %+v", fallbackIdentity)
	}
	if fallbackIdentity.DiscoveryEndpointPath != "/gateway/models" || fallbackIdentity.DiscoveryResponseProfile != "generic" {
		t.Fatalf("expected fallback identity to preserve normalized discovery settings, got %+v", fallbackIdentity)
	}
}

func TestNormalizeProviderDiscoveryEndpointPath(t *testing.T) {
	t.Parallel()

	got, err := NormalizeProviderDiscoveryEndpointPath(" models ")
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoveryEndpointPath() error = %v", err)
	}
	if got != "/models" {
		t.Fatalf("expected /models, got %q", got)
	}

	if _, err := NormalizeProviderDiscoveryEndpointPath("https://api.example.com/models"); err == nil {
		t.Fatalf("expected absolute URL to be rejected")
	}
	if _, err := NormalizeProviderDiscoveryEndpointPath("/models?x=1"); err == nil {
		t.Fatalf("expected query string to be rejected")
	}
}

func TestNormalizeProviderDiscoveryResponseProfile(t *testing.T) {
	t.Parallel()

	got, err := NormalizeProviderDiscoveryResponseProfile(" Gemini ")
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoveryResponseProfile() error = %v", err)
	}
	if got != DiscoveryResponseProfileGemini {
		t.Fatalf("expected gemini, got %q", got)
	}

	if _, err := NormalizeProviderDiscoveryResponseProfile("unsupported-profile"); err == nil {
		t.Fatalf("expected unsupported profile to fail")
	}
}

func TestNormalizeProviderDiscoverySettings(t *testing.T) {
	t.Parallel()

	endpointPath, responseProfile, err := NormalizeProviderDiscoverySettings(
		DriverOpenAICompat,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoverySettings() openaicompat error = %v", err)
	}
	if endpointPath != DiscoveryEndpointPathModels || responseProfile != DiscoveryResponseProfileOpenAI {
		t.Fatalf("expected openaicompat defaults, got endpoint=%q profile=%q", endpointPath, responseProfile)
	}

	endpointPath, responseProfile, err = NormalizeProviderDiscoverySettings(
		"custom-driver",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoverySettings() custom driver error = %v", err)
	}
	if endpointPath != "" || responseProfile != "" {
		t.Fatalf("expected custom driver to keep empty discovery settings, got endpoint=%q profile=%q", endpointPath, responseProfile)
	}
}
