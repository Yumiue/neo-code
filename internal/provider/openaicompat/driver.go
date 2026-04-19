package openaicompat

import (
	"context"
	"net/http"

	"neo-code/internal/provider"
	"neo-code/internal/provider/transport"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是当前 OpenAI-compatible 协议驱动的唯一标识。
const DriverName = provider.DriverOpenAICompat

// defaultRetryTransport 返回内置的带重试 HTTP Transport。
func defaultRetryTransport() http.RoundTripper {
	return transport.NewRetryTransport(http.DefaultTransport, transport.DefaultRetryConfig())
}

// Driver 返回 OpenAI-compatible 协议驱动定义。
func Driver() provider.DriverDefinition {
	return driverDefinition(DriverName)
}

// validateCatalogIdentity 在 catalog 快照与缓存路径上复用协议归一化校验，避免无效静态配置进入选择流程。
func validateCatalogIdentity(identity provider.ProviderIdentity) error {
	_, err := provider.NormalizeProviderProtocolSettings(
		identity.Driver,
		identity.ChatProtocol,
		identity.ChatEndpointPath,
		identity.DiscoveryProtocol,
		identity.DiscoveryEndpointPath,
		identity.AuthStrategy,
		identity.ResponseProfile,
		"",
		"",
	)
	if err != nil {
		return provider.NewDiscoveryConfigError(err.Error())
	}
	return nil
}

// driverDefinition 根据驱动名构造共享的 OpenAI-compatible 协议驱动定义。
func driverDefinition(name string) provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: name,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			return New(
				cfg,
				withTransport(defaultRetryTransport()),
				withExecutionMode(resolveExecutionModeFromEnv()),
			)
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			p, err := New(
				cfg,
				withTransport(defaultRetryTransport()),
				withExecutionMode(resolveExecutionModeFromEnv()),
			)
			if err != nil {
				return nil, err
			}
			return p.DiscoverModels(ctx)
		},
		ValidateCatalogIdentity: validateCatalogIdentity,
	}
}
