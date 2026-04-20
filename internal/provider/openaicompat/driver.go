package openaicompat

import (
	"context"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是当前 OpenAI-compatible 协议驱动的唯一标识。
const DriverName = provider.DriverOpenAICompat

// Driver 返回 OpenAI-compatible 协议驱动定义。
func Driver() provider.DriverDefinition {
	return driverDefinition(DriverName)
}

// validateCatalogIdentity 在 catalog 快照与缓存路径上校验最小 OpenAI-compatible 身份字段。
func validateCatalogIdentity(identity provider.ProviderIdentity) error {
	normalized, err := provider.NormalizeProviderIdentity(identity)
	if err != nil {
		return provider.NewDiscoveryConfigError(err.Error())
	}
	if normalized.Driver != DriverName {
		return provider.NewDiscoveryConfigError("openaicompat driver: identity driver is unsupported")
	}
	return nil
}

// driverDefinition 根据驱动名构造共享的 OpenAI-compatible 协议驱动定义。
func driverDefinition(name string) provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: name,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			return New(cfg)
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			p, err := New(cfg)
			if err != nil {
				return nil, err
			}
			return p.DiscoverModels(ctx)
		},
		ValidateCatalogIdentity: validateCatalogIdentity,
	}
}
