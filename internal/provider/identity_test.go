package provider

import "testing"

func TestProviderIdentityKeyIncludesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		Driver:                "openaicompat",
		BaseURL:               "https://api.example.com/v1",
		ChatProtocol:          ChatProtocolOpenAIChatCompletions,
		DiscoveryProtocol:     DiscoveryProtocolOpenAIModels,
		AuthStrategy:          AuthStrategyBearer,
		ResponseProfile:       DiscoveryResponseProfileOpenAI,
		DiscoveryEndpointPath: "/v2/models",
	}

	if got, want := identity.Key(), "openaicompat|https://api.example.com/v1|openai_chat_completions|openai_models|bearer|openai|/v2/models"; got != want {
		t.Fatalf("expected identity key %q, got %q", want, got)
	}
}

func TestNormalizeProviderIdentityUsesDriverSpecificNormalization(t *testing.T) {
	t.Parallel()

	identity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                " OpenAICompat ",
		BaseURL:               "https://API.EXAMPLE.COM/v1/",
		DiscoveryEndpointPath: " models ",
		ResponseProfile:       " Generic ",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() error = %v", err)
	}

	if identity.Driver != DriverOpenAICompat {
		t.Fatalf("expected normalized driver %q, got %q", DriverOpenAICompat, identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.ChatProtocol != ChatProtocolOpenAIChatCompletions {
		t.Fatalf("expected normalized chat protocol %q, got %q", ChatProtocolOpenAIChatCompletions, identity.ChatProtocol)
	}
	if identity.DiscoveryProtocol != DiscoveryProtocolOpenAIModels {
		t.Fatalf("expected normalized discovery protocol %q, got %q", DiscoveryProtocolOpenAIModels, identity.DiscoveryProtocol)
	}
	if identity.DiscoveryEndpointPath != "/models" {
		t.Fatalf("expected normalized discovery endpoint path %q, got %q", "/models", identity.DiscoveryEndpointPath)
	}
	if identity.ResponseProfile != DiscoveryResponseProfileGeneric {
		t.Fatalf("expected normalized response profile %q, got %q", DiscoveryResponseProfileGeneric, identity.ResponseProfile)
	}
}

func TestNormalizeProviderIdentityPreservesDriverSpecificFields(t *testing.T) {
	t.Parallel()

	identity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                " Gemini ",
		BaseURL:               "https://API.EXAMPLE.COM/v1/",
		DiscoveryEndpointPath: "/models",
		ResponseProfile:       "gemini",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() error = %v", err)
	}

	if identity.Driver != DriverGemini {
		t.Fatalf("expected normalized driver %q, got %q", DriverGemini, identity.Driver)
	}
	if identity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected normalized base url %q, got %q", "https://api.example.com/v1", identity.BaseURL)
	}
	if identity.ChatProtocol != ChatProtocolGeminiNative {
		t.Fatalf("expected normalized chat protocol %q, got %q", ChatProtocolGeminiNative, identity.ChatProtocol)
	}
	if identity.DiscoveryProtocol != DiscoveryProtocolGeminiModels {
		t.Fatalf("expected normalized discovery protocol %q, got %q", DiscoveryProtocolGeminiModels, identity.DiscoveryProtocol)
	}
	if identity.DiscoveryEndpointPath != "/models" || identity.ResponseProfile != DiscoveryResponseProfileGemini {
		t.Fatalf("expected normalized discovery settings, got %+v", identity)
	}
}

func TestProviderIdentityStringMatchesKey(t *testing.T) {
	t.Parallel()

	identity := ProviderIdentity{
		Driver:       "openaicompat",
		BaseURL:      "https://api.example.com/v1",
		ChatProtocol: ChatProtocolOpenAIChatCompletions,
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
	if _, err := NewProviderIdentity("openaicompat", "https://token@api.example.com/v1"); err == nil {
		t.Fatalf("expected base URL with userinfo to fail")
	}
}

func TestNormalizeProviderIdentityAnthropicAndUnknownDriver(t *testing.T) {
	t.Parallel()

	anthropicIdentity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:  " Anthropic ",
		BaseURL: "https://API.EXAMPLE.COM/v1/",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() anthropic error = %v", err)
	}
	if anthropicIdentity.Driver != DriverAnthropic {
		t.Fatalf("expected anthropic driver, got %+v", anthropicIdentity)
	}
	if anthropicIdentity.AuthStrategy != AuthStrategyAnthropic {
		t.Fatalf("expected anthropic auth strategy %q, got %+v", AuthStrategyAnthropic, anthropicIdentity)
	}
	if anthropicIdentity.DiscoveryProtocol != DiscoveryProtocolAnthropicModels {
		t.Fatalf("expected anthropic discovery protocol %q, got %+v", DiscoveryProtocolAnthropicModels, anthropicIdentity)
	}

	fallbackIdentity, err := NormalizeProviderIdentity(ProviderIdentity{
		Driver:                " custom ",
		BaseURL:               "https://API.EXAMPLE.COM/v1/",
		DiscoveryEndpointPath: "gateway/models",
		ResponseProfile:       "generic",
	})
	if err != nil {
		t.Fatalf("NormalizeProviderIdentity() fallback error = %v", err)
	}
	if fallbackIdentity.Driver != "custom" || fallbackIdentity.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected fallback identity to normalize driver and base URL, got %+v", fallbackIdentity)
	}
	if fallbackIdentity.DiscoveryEndpointPath != "/gateway/models" || fallbackIdentity.ResponseProfile != "generic" {
		t.Fatalf("expected fallback identity to preserve normalized discovery settings, got %+v", fallbackIdentity)
	}
}

func TestNormalizeProviderIdentityUsesDriverDefaultsMatrix(t *testing.T) {
	t.Parallel()

	tests := []string{DriverOpenAICompat, DriverGemini, DriverAnthropic}
	for _, driver := range tests {
		driver := driver
		t.Run(driver, func(t *testing.T) {
			t.Parallel()

			identity, err := NormalizeProviderIdentity(ProviderIdentity{
				Driver:  driver,
				BaseURL: "https://api.example.com/v1",
			})
			if err != nil {
				t.Fatalf("NormalizeProviderIdentity() error = %v", err)
			}
			defaults := ResolveDriverProtocolDefaults(driver)
			if identity.ChatProtocol != defaults.ChatProtocol {
				t.Fatalf("expected chat protocol %q, got %q", defaults.ChatProtocol, identity.ChatProtocol)
			}
			if identity.DiscoveryProtocol != defaults.DiscoveryProtocol {
				t.Fatalf("expected discovery protocol %q, got %q", defaults.DiscoveryProtocol, identity.DiscoveryProtocol)
			}
			if identity.AuthStrategy != defaults.AuthStrategy {
				t.Fatalf("expected auth strategy %q, got %q", defaults.AuthStrategy, identity.AuthStrategy)
			}
			if identity.ResponseProfile != defaults.ResponseProfile {
				t.Fatalf("expected response profile %q, got %q", defaults.ResponseProfile, identity.ResponseProfile)
			}
			if identity.DiscoveryEndpointPath != DiscoveryEndpointPathModels {
				t.Fatalf("expected discovery endpoint %q, got %q", DiscoveryEndpointPathModels, identity.DiscoveryEndpointPath)
			}
		})
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

	endpointPath, responseProfile, err := NormalizeProviderDiscoverySettings(DriverOpenAICompat, "", "")
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoverySettings() openaicompat error = %v", err)
	}
	if endpointPath != DiscoveryEndpointPathModels || responseProfile != DiscoveryResponseProfileOpenAI {
		t.Fatalf("expected openaicompat defaults, got endpoint=%q profile=%q", endpointPath, responseProfile)
	}

	endpointPath, responseProfile, err = NormalizeProviderDiscoverySettings("custom-driver", "", "")
	if err != nil {
		t.Fatalf("NormalizeProviderDiscoverySettings() custom driver error = %v", err)
	}
	if endpointPath != "" || responseProfile != "" {
		t.Fatalf("expected custom driver to keep empty discovery settings, got endpoint=%q profile=%q", endpointPath, responseProfile)
	}
}
