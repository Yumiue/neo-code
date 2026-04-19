package openaicompat

import (
	"testing"

	"neo-code/internal/provider"
)

func TestNormalizeExecutionMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults auto", input: "", want: executionModeAuto},
		{name: "auto", input: "auto", want: executionModeAuto},
		{name: "http", input: "http", want: executionModeHTTP},
		{name: "sdk", input: "sdk", want: executionModeSDK},
		{name: "case-insensitive", input: " SDK ", want: executionModeSDK},
		{name: "invalid", input: "grpc", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeExecutionMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeExecutionMode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveExecutionModeFromEnv(t *testing.T) {
	t.Setenv(executionModeEnvName, "sdk")
	if got := resolveExecutionModeFromEnv(); got != executionModeSDK {
		t.Fatalf("expected sdk mode from env, got %q", got)
	}

	t.Setenv(executionModeEnvName, "invalid-mode")
	if got := resolveExecutionModeFromEnv(); got != executionModeAuto {
		t.Fatalf("invalid env should fallback to auto, got %q", got)
	}
}

func TestResolveExecutionModeAutoHeuristic(t *testing.T) {
	t.Parallel()

	openAICfg := provider.RuntimeConfig{
		Driver:           provider.DriverOpenAICompat,
		BaseURL:          "https://api.openai.com/v1",
		ChatEndpointPath: "/chat/completions",
	}
	if got := resolveExecutionMode(openAICfg, provider.ChatProtocolOpenAIChatCompletions, executionModeAuto); got != executionModeSDK {
		t.Fatalf("expected auto mode to prefer sdk on api.openai.com, got %q", got)
	}

	customCfg := provider.RuntimeConfig{
		Driver:           provider.DriverOpenAICompat,
		BaseURL:          "https://gateway.example.com/v1",
		ChatEndpointPath: "/chat/completions",
	}
	if got := resolveExecutionMode(customCfg, provider.ChatProtocolOpenAIChatCompletions, executionModeAuto); got != executionModeHTTP {
		t.Fatalf("expected auto mode to fallback to http on custom host, got %q", got)
	}
}
