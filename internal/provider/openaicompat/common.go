package openaicompat

import (
	"errors"
	"strings"

	"neo-code/internal/provider"
)

const errorPrefix = "openaicompat provider: "

// validateRuntimeConfig 校验 OpenAI-compatible 运行时最小配置，确保后续请求阶段不出现空地址或空密钥。
func validateRuntimeConfig(cfg provider.RuntimeConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New(errorPrefix + "base url is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New(errorPrefix + "api key is empty")
	}
	return nil
}
