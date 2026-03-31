package builtin

import (
	"context"
	"errors"
	"testing"

	"neo-code/internal/config"
	"neo-code/internal/provider"
	"neo-code/internal/provider/openai"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reg     *provider.Registry
		wantErr string
	}{
		{
			name:    "nil registry",
			reg:     nil,
			wantErr: "builtin provider registry is nil",
		},
		{
			name: "registers openai driver",
			reg:  provider.NewRegistry(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Register(tt.reg)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.reg.Supports(openai.Name) {
				t.Fatalf("expected registry to support %q", openai.Name)
			}
		})
	}
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg == nil {
		t.Fatal("expected registry")
	}
	if !reg.Supports(openai.Name) {
		t.Fatalf("expected builtin registry to support %q", openai.Name)
	}
}

func TestNewRegistryBuildsRegisteredDriver(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = reg.Build(context.Background(), config.ResolvedProviderConfig{
		ProviderConfig: config.ProviderConfig{
			Driver: openai.Name,
		},
	})
	if err == nil {
		t.Fatal("expected build to fail without api key or required config")
	}
	if errors.Is(err, provider.ErrDriverNotFound) {
		t.Fatalf("expected registered driver error, got %v", err)
	}
}
