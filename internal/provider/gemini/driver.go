package gemini

import (
	"context"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是 Gemini 协议驱动的唯一标识。
const DriverName = provider.DriverGemini

// Driver 返回 Gemini 协议驱动定义。
func Driver() provider.DriverDefinition {
	compatDriver := openaicompat.Driver()

	return provider.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			normalized, err := normalizeRuntimeConfig(cfg)
			if err != nil {
				return nil, err
			}
			return compatDriver.Build(ctx, normalized)
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			normalized, err := normalizeRuntimeConfig(cfg)
			if err != nil {
				return nil, err
			}
			return compatDriver.Discover(ctx, normalized)
		},
		ValidateCatalogIdentity: validateCatalogIdentity,
	}
}

// normalizeRuntimeConfig 统一收敛 Gemini runtime 配置并兼容映射到 openaicompat 执行路径。
func normalizeRuntimeConfig(cfg provider.RuntimeConfig) (provider.RuntimeConfig, error) {
	normalizedProtocols, err := provider.NormalizeProviderProtocolSettings(
		cfg.Driver,
		cfg.ChatProtocol,
		cfg.ChatEndpointPath,
		cfg.DiscoveryProtocol,
		cfg.DiscoveryEndpointPath,
		cfg.AuthStrategy,
		cfg.ResponseProfile,
		cfg.APIStyle,
		cfg.DiscoveryResponseProfile,
	)
	if err != nil {
		return provider.RuntimeConfig{}, provider.NewDiscoveryConfigError(err.Error())
	}

	normalized := cfg
	normalized.Driver = provider.DriverOpenAICompat
	normalized.ChatProtocol = normalizedProtocols.ChatProtocol
	normalized.ChatEndpointPath = normalizedProtocols.ChatEndpointPath
	normalized.DiscoveryProtocol = normalizedProtocols.DiscoveryProtocol
	normalized.AuthStrategy = normalizedProtocols.AuthStrategy
	normalized.ResponseProfile = normalizedProtocols.ResponseProfile
	normalized.APIStyle = normalizedProtocols.LegacyAPIStyle
	normalized.DiscoveryEndpointPath = normalizedProtocols.DiscoveryEndpointPath
	normalized.DiscoveryResponseProfile = normalizedProtocols.ResponseProfile
	return normalized, nil
}

// validateCatalogIdentity 在 catalog 路径上执行 Gemini 协议静态校验，避免无效快照误导选择流程。
func validateCatalogIdentity(identity provider.ProviderIdentity) error {
	_, err := provider.NormalizeProviderProtocolSettings(
		identity.Driver,
		identity.ChatProtocol,
		identity.ChatEndpointPath,
		identity.DiscoveryProtocol,
		identity.DiscoveryEndpointPath,
		identity.AuthStrategy,
		identity.ResponseProfile,
		identity.APIStyle,
		identity.DiscoveryResponseProfile,
	)
	if err != nil {
		return provider.NewDiscoveryConfigError(err.Error())
	}
	return nil
}
