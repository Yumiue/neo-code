package shared

import (
	"errors"
	"net/http"
	"strings"

	"neo-code/internal/provider"
	"neo-code/internal/provider/discovery"
)

// ErrorPrefix 统一收敛 OpenAI 兼容 provider 的错误前缀
const ErrorPrefix = "openaicompat provider: "

func ValidateRuntimeConfig(cfg provider.RuntimeConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New(ErrorPrefix + "base url is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New(ErrorPrefix + "api key is empty")
	}
	return nil
}

func Endpoint(baseURL string, path string) string {
	return discovery.ResolveEndpoint(baseURL, path)
}

func SetBearerAuthorization(header http.Header, apiKey string) {
	provider.ApplyAuthHeaders(header, provider.AuthStrategyBearer, apiKey, "")
}

// ApplyAuthHeaders 根据 runtime 配置中的 auth strategy 写入鉴权头。
func ApplyAuthHeaders(header http.Header, cfg provider.RuntimeConfig) {
	provider.ApplyAuthHeaders(header, cfg.AuthStrategy, cfg.APIKey, cfg.APIVersion)
}
