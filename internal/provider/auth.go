package provider

import (
	"net/http"
	"strings"
)

const defaultAnthropicAPIVersion = "2023-06-01"

// ApplyAuthHeaders 根据认证策略写入请求头，统一收敛 provider 层鉴权行为并避免上层重复分支。
func ApplyAuthHeaders(header http.Header, authStrategy string, apiKey string, apiVersion string) {
	if header == nil {
		return
	}

	trimmedAPIKey := strings.TrimSpace(apiKey)
	if trimmedAPIKey == "" {
		return
	}

	switch NormalizeProviderAuthStrategy(authStrategy) {
	case "", AuthStrategyBearer:
		header.Set("Authorization", "Bearer "+trimmedAPIKey)
	case AuthStrategyXAPIKey:
		header.Set("X-API-Key", trimmedAPIKey)
	case AuthStrategyAnthropic:
		header.Set("x-api-key", trimmedAPIKey)
		version := strings.TrimSpace(apiVersion)
		if version == "" {
			version = defaultAnthropicAPIVersion
		}
		header.Set("anthropic-version", version)
	default:
		header.Set("Authorization", "Bearer "+trimmedAPIKey)
	}
}
