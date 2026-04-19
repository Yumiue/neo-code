package anthropic

import (
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/provider"
)

const errorPrefix = "anthropic provider: "

// validateRuntimeConfig 校验 Anthropic 运行时最小配置，避免请求阶段才暴露空字段错误。
func validateRuntimeConfig(cfg provider.RuntimeConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New(errorPrefix + "base url is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New(errorPrefix + "api key is empty")
	}
	return nil
}

// supportedChatProtocol 校验 Anthropic 驱动解析后的聊天协议，防止误走其他协议实现。
func supportedChatProtocol(cfg provider.RuntimeConfig) error {
	normalized := provider.ResolveDriverProtocolDefaults(cfg.Driver).ChatProtocol
	if normalized == provider.ChatProtocolAnthropicMessages {
		return nil
	}
	return provider.NewDiscoveryConfigError(
		fmt.Sprintf("anthropic driver: chat protocol %q is not supported", normalized),
	)
}
