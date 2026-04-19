package openaicompat

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"neo-code/internal/provider"
)

const executionModeEnvName = "NEOCODE_OPENAICOMPAT_EXECUTION_MODE"

const (
	executionModeAuto = "auto"
	executionModeHTTP = "http"
	executionModeSDK  = "sdk"
)

func normalizeExecutionMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return executionModeAuto, nil
	}
	switch mode {
	case executionModeAuto, executionModeHTTP, executionModeSDK:
		return mode, nil
	default:
		return "", fmt.Errorf("%sinvalid execution mode %q (expected: auto/http/sdk)", errorPrefix, raw)
	}
}

func resolveExecutionModeFromEnv() string {
	mode, err := normalizeExecutionMode(os.Getenv(executionModeEnvName))
	if err != nil {
		return executionModeAuto
	}
	return mode
}

func resolveExecutionMode(cfg provider.RuntimeConfig, chatProtocol string, configuredMode string) string {
	switch configuredMode {
	case executionModeHTTP, executionModeSDK:
		return configuredMode
	default:
		if shouldPreferSDKInAuto(cfg, chatProtocol) {
			return executionModeSDK
		}
		return executionModeHTTP
	}
}

func shouldPreferSDKInAuto(cfg provider.RuntimeConfig, chatProtocol string) bool {
	switch chatProtocol {
	case provider.ChatProtocolOpenAIChatCompletions, provider.ChatProtocolOpenAIResponses:
	default:
		return false
	}

	endpoint, err := provider.ResolveChatEndpointURL(cfg.BaseURL, cfg.ChatEndpointPath)
	if err != nil {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "api.openai.com"
}
