package anthropic

import (
	"context"
	"net/http"
	"time"

	"neo-code/internal/provider"
	httpdiscovery "neo-code/internal/provider/discovery/http"
	"neo-code/internal/provider/openaicompat"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是 Anthropic 协议驱动的唯一标识。
const DriverName = provider.DriverAnthropic

// Driver 返回 Anthropic 协议驱动定义。
func Driver() provider.DriverDefinition {
	compatDriver := openaicompat.Driver()

	return provider.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			normalized, err := normalizeRuntimeConfig(cfg)
			if err != nil {
				return nil, err
			}
			if normalized.ChatProtocol == provider.ChatProtocolAnthropicMessages {
				return nil, provider.NewDiscoveryConfigError("anthropic driver: chat_protocol anthropic_messages is not supported yet")
			}
			normalized.Driver = provider.DriverOpenAICompat
			return compatDriver.Build(ctx, normalized)
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			normalized, err := normalizeRuntimeConfig(cfg)
			if err != nil {
				return nil, err
			}

			rawModels, err := httpdiscovery.DiscoverRawModels(ctx, &http.Client{Timeout: 90 * time.Second}, httpdiscovery.RequestConfig{
				BaseURL:           normalized.BaseURL,
				EndpointPath:      normalized.DiscoveryEndpointPath,
				DiscoveryProtocol: normalized.DiscoveryProtocol,
				ResponseProfile:   normalized.ResponseProfile,
				AuthStrategy:      normalized.AuthStrategy,
				APIKey:            normalized.APIKey,
				APIVersion:        normalized.APIVersion,
			})
			if err != nil {
				return nil, err
			}

			descriptors := make([]providertypes.ModelDescriptor, 0, len(rawModels))
			for _, raw := range rawModels {
				descriptor, ok := providertypes.DescriptorFromRawModel(raw)
				if !ok {
					continue
				}
				descriptors = append(descriptors, descriptor)
			}
			return providertypes.MergeModelDescriptors(descriptors), nil
		},
		ValidateCatalogIdentity: validateCatalogIdentity,
	}
}

// normalizeRuntimeConfig 统一收敛 Anthropic runtime 配置并执行组合校验。
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

// validateCatalogIdentity 在 catalog 路径上执行 Anthropic 协议静态校验。
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
