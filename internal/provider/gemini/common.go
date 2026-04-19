package gemini

import (
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/provider"
)

const errorPrefix = "gemini provider: "

// validateRuntimeConfig 校验 Gemini 运行时最小配置，提前阻断空地址与空密钥场景。
func validateRuntimeConfig(cfg provider.RuntimeConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New(errorPrefix + "base url is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New(errorPrefix + "api key is empty")
	}
	return nil
}

// supportedChatProtocol 校验 Gemini 驱动解析后的聊天协议，防止错误路由到其他协议实现。
func supportedChatProtocol(cfg provider.RuntimeConfig) error {
	normalized := provider.ResolveDriverProtocolDefaults(cfg.Driver).ChatProtocol
	if normalized == provider.ChatProtocolGeminiNative {
		return nil
	}
	return provider.NewDiscoveryConfigError(
		fmt.Sprintf("gemini driver: chat protocol %q is not supported", normalized),
	)
}
