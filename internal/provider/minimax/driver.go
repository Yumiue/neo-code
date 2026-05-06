package minimax

import (
	"context"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

const DriverName = provider.DriverMiniMax

func Driver() provider.DriverDefinition {
	return provider.DriverDefinition{
		Name: DriverName,
		Build: func(ctx context.Context, cfg provider.RuntimeConfig) (provider.Provider, error) {
			return New(cfg)
		},
		Discover: func(ctx context.Context, cfg provider.RuntimeConfig) ([]providertypes.ModelDescriptor, error) {
			return nil, nil
		},
		ValidateCatalogIdentity: func(identity provider.ProviderIdentity) error {
			return nil
		},
	}
}
