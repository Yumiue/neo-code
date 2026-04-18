package provider

import "testing"

func TestNormalizeProviderProtocolSettingsLegacyMapping(t *testing.T) {
	t.Parallel()

	settings, err := NormalizeProviderProtocolSettings(
		DriverOpenAICompat,
		"",
		"",
		"",
		"",
		"",
		"",
		OpenAICompatibleAPIStyleResponses,
		DiscoveryResponseProfileGeneric,
	)
	if err != nil {
		t.Fatalf("NormalizeProviderProtocolSettings() error = %v", err)
	}

	if settings.ChatProtocol != ChatProtocolOpenAIResponses {
		t.Fatalf("expected chat protocol %q, got %q", ChatProtocolOpenAIResponses, settings.ChatProtocol)
	}
	if settings.ResponseProfile != DiscoveryResponseProfileGeneric {
		t.Fatalf("expected response profile %q, got %q", DiscoveryResponseProfileGeneric, settings.ResponseProfile)
	}
	if settings.DiscoveryEndpointPath != DiscoveryEndpointPathModels {
		t.Fatalf("expected discovery endpoint path %q, got %q", DiscoveryEndpointPathModels, settings.DiscoveryEndpointPath)
	}
}

func TestNormalizeProviderProtocolSettingsRejectsIllegalCombination(t *testing.T) {
	t.Parallel()

	_, err := NormalizeProviderProtocolSettings(
		DriverAnthropic,
		ChatProtocolAnthropicMessages,
		"",
		DiscoveryProtocolAnthropicModels,
		"",
		AuthStrategyBearer,
		DiscoveryResponseProfileGeneric,
		"",
		"",
	)
	if err == nil {
		t.Fatal("expected illegal chat/auth combination to fail")
	}
}

func TestNormalizeProviderProtocolSettingsUnknownDriverKeepsLegacyAPIStyleEmpty(t *testing.T) {
	t.Parallel()

	settings, err := NormalizeProviderProtocolSettings(
		"custom-driver",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("NormalizeProviderProtocolSettings() error = %v", err)
	}
	if settings.LegacyAPIStyle != "" {
		t.Fatalf("expected unknown driver to keep legacy api style empty, got %q", settings.LegacyAPIStyle)
	}
}
