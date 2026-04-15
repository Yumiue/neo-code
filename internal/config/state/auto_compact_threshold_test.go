package state

import (
	"context"
	"errors"
	"testing"

	configpkg "neo-code/internal/config"
	providertypes "neo-code/internal/provider/types"
)

func TestResolveAutoCompactThresholdDisabled(t *testing.T) {
	t.Parallel()

	cfg := configpkg.StaticDefaults().Clone()
	cfg.Context.AutoCompact.Enabled = false

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 0 || resolution.Source != AutoCompactThresholdSourceDisabled {
		t.Fatalf("expected disabled resolution, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdExplicitWins(t *testing.T) {
	t.Parallel()

	cfg := configpkg.StaticDefaults().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 42000

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 42000 || resolution.Source != AutoCompactThresholdSourceExplicit {
		t.Fatalf("expected explicit resolution, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdDerivedFromContextWindow(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.ReserveTokens = 13000
	cfg.CurrentModel = "deepseek-coder"
	cfg.Providers[0].Model = "deepseek-coder"
	cfg.Providers[0].Models = []providertypes.ModelDescriptor{{
		ID:            "deepseek-coder",
		ContextWindow: 131072,
	}}

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{
		snapshotModels: cfg.Providers[0].Models,
	})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 118072 || resolution.Source != AutoCompactThresholdSourceDerived {
		t.Fatalf("expected derived threshold, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdFallsBackWhenWindowTooSmall(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.ReserveTokens = 13000
	cfg.Context.AutoCompact.FallbackInputTokenThreshold = 88000
	cfg.CurrentModel = "small-model"
	cfg.Providers[0].Model = "small-model"

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{
		snapshotModels: []providertypes.ModelDescriptor{{
			ID:            "small-model",
			ContextWindow: 8000,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 88000 || resolution.Source != AutoCompactThresholdSourceFallback {
		t.Fatalf("expected fallback threshold, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdFallsBackWhenModelMissing(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.FallbackInputTokenThreshold = 88000
	cfg.CurrentModel = "missing-model"

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{
		snapshotModels: []providertypes.ModelDescriptor{{ID: "other-model", ContextWindow: 131072}},
	})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 88000 || resolution.Source != AutoCompactThresholdSourceFallback {
		t.Fatalf("expected missing model to use fallback, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdFallsBackWhenSelectedProviderInvalid(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.FallbackInputTokenThreshold = 88000
	cfg.SelectedProvider = "missing-provider"

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 88000 || resolution.Source != AutoCompactThresholdSourceFallback {
		t.Fatalf("expected invalid selection to use fallback, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdFallsBackWhenCatalogInputResolutionFails(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.FallbackInputTokenThreshold = 88000
	cfg.Providers[0].BaseURL = ""

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 88000 || resolution.Source != AutoCompactThresholdSourceFallback {
		t.Fatalf("expected invalid catalog input to use fallback, got %+v", resolution)
	}
}

func TestResolveAutoCompactThresholdFallsBackWhenSnapshotLookupFails(t *testing.T) {
	t.Parallel()

	cfg := testDefaultConfig().Clone()
	cfg.Context.AutoCompact.Enabled = true
	cfg.Context.AutoCompact.InputTokenThreshold = 0
	cfg.Context.AutoCompact.FallbackInputTokenThreshold = 88000

	resolution, err := ResolveAutoCompactThreshold(context.Background(), cfg, catalogMethodsStub{
		snapshotErr: errors.New("snapshot failed"),
	})
	if err != nil {
		t.Fatalf("ResolveAutoCompactThreshold() error = %v", err)
	}
	if resolution.Threshold != 88000 || resolution.Source != AutoCompactThresholdSourceFallback {
		t.Fatalf("expected snapshot error to use fallback, got %+v", resolution)
	}
}
