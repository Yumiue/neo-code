package anthropic

import (
	"context"
	"net/http"
	"time"

	"neo-code/internal/provider"
	httpdiscovery "neo-code/internal/provider/discovery/http"
	"neo-code/internal/provider/transport"
	providertypes "neo-code/internal/provider/types"
)

// DriverName 是 Anthropic 协议驱动的唯一标识。
const DriverName = provider.DriverAnthropic

// defaultRetryTransport 返回 Anthropic 驱动默认使用的重试传输层。
func defaultRetryTransport() http.RoundTripper {
	return transport.NewRetryTransport(http.DefaultTransport, transport.DefaultRetryConfig())
}

// Driver 返回 Anthropic 协议驱动定义。
func Driver() provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			return New(cfg, withTransport(defaultRetryTransport()))
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			requestCfg, err := httpdiscovery.RequestConfigFromRuntime(cfg)
			if err != nil {
				return nil, err
			}
			client := &http.Client{
				Timeout:   90 * time.Second,
				Transport: defaultRetryTransport(),
			}
			return httpdiscovery.DiscoverModelDescriptors(ctx, client, requestCfg)
		},
		ValidateCatalogIdentity: validateCatalogIdentity,
	}
}

// validateCatalogIdentity 在 catalog 路径上执行 Anthropic 静态校验。
func validateCatalogIdentity(identity provider.ProviderIdentity) error {
	if _, err := provider.NormalizeProviderChatEndpointPath(identity.ChatEndpointPath); err != nil {
		return provider.NewDiscoveryConfigError(err.Error())
	}
	if _, _, _, err := provider.ResolveDriverDiscoveryConfig(identity.Driver, identity.DiscoveryEndpointPath); err != nil {
		return provider.NewDiscoveryConfigError(err.Error())
	}
	return nil
}
