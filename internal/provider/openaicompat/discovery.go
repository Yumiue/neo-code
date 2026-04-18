package openaicompat

import (
	"context"

	"neo-code/internal/provider"
	httpdiscovery "neo-code/internal/provider/discovery/http"
)

// fetchModels 调用通用 discovery HTTP 引擎，并输出标准化原始模型对象列表。
func (p *Provider) fetchModels(ctx context.Context) ([]map[string]any, error) {
	normalizedProtocols, err := provider.NormalizeProviderProtocolSettings(
		p.cfg.Driver,
		p.cfg.ChatProtocol,
		p.cfg.ChatEndpointPath,
		p.cfg.DiscoveryProtocol,
		p.cfg.DiscoveryEndpointPath,
		p.cfg.AuthStrategy,
		p.cfg.ResponseProfile,
		p.cfg.APIStyle,
		p.cfg.DiscoveryResponseProfile,
	)
	if err != nil {
		return nil, provider.NewDiscoveryConfigError(err.Error())
	}

	return httpdiscovery.DiscoverRawModels(ctx, p.client, httpdiscovery.RequestConfig{
		BaseURL:           p.cfg.BaseURL,
		EndpointPath:      normalizedProtocols.DiscoveryEndpointPath,
		DiscoveryProtocol: normalizedProtocols.DiscoveryProtocol,
		ResponseProfile:   normalizedProtocols.ResponseProfile,
		AuthStrategy:      normalizedProtocols.AuthStrategy,
		APIKey:            p.cfg.APIKey,
		APIVersion:        p.cfg.APIVersion,
	})
}
