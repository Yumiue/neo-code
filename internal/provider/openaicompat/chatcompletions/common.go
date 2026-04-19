package chatcompletions

import (
	"errors"
	"net/http"
	"strings"

	"neo-code/internal/provider"
)

const errorPrefix = "openaicompat provider: "

// validateRuntimeConfig 校验 Chat Completions 执行所需的最小配置，提前失败以减少运行期分支。
func validateRuntimeConfig(cfg provider.RuntimeConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New(errorPrefix + "base url is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New(errorPrefix + "api key is empty")
	}
	return nil
}

// applyAuthHeaders 根据 driver 默认鉴权规则写入请求头，避免调用侧重复处理策略分支。
func applyAuthHeaders(header http.Header, cfg provider.RuntimeConfig) {
	authStrategy, apiVersion := provider.ResolveDriverAuthConfig(cfg.Driver)
	provider.ApplyAuthHeaders(header, authStrategy, cfg.APIKey, apiVersion)
}
