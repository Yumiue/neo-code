package builtin

import (
	"testing"

	"neo-code/internal/provider"
	"neo-code/internal/provider/anthropic"
	"neo-code/internal/provider/gemini"
	"neo-code/internal/provider/openaicompat"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if !registry.Supports(openaicompat.DriverName) {
		t.Fatalf("expected registry to include %q driver", openaicompat.DriverName)
	}
	if !registry.Supports(gemini.DriverName) {
		t.Fatalf("expected registry to include %q driver", gemini.DriverName)
	}
	if !registry.Supports(anthropic.DriverName) {
		t.Fatalf("expected registry to include %q driver", anthropic.DriverName)
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("nil registry", func(t *testing.T) {
		t.Parallel()
		err := register(nil)
		if err == nil {
			t.Fatal("expected error for nil registry")
		}
		if err.Error() != "builtin provider registry is nil" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid registry", func(t *testing.T) {
		t.Parallel()
		registry := provider.NewRegistry()
		err := register(registry)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if !registry.Supports(openaicompat.DriverName) {
			t.Fatalf("expected registry to include %q driver", openaicompat.DriverName)
		}
		if !registry.Supports(gemini.DriverName) {
			t.Fatalf("expected registry to include %q driver", gemini.DriverName)
		}
		if !registry.Supports(anthropic.DriverName) {
			t.Fatalf("expected registry to include %q driver", anthropic.DriverName)
		}
	})
}
