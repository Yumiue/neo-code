package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"neo-code/internal/config"
	agentruntime "neo-code/internal/runtime"
)

type stubRuntime struct {
	events chan agentruntime.RuntimeEvent
}

func newStubRuntime() *stubRuntime {
	return &stubRuntime{events: make(chan agentruntime.RuntimeEvent)}
}

func (s *stubRuntime) Run(ctx context.Context, input agentruntime.UserInput) error {
	return nil
}

func (s *stubRuntime) Compact(ctx context.Context, input agentruntime.CompactInput) (agentruntime.CompactResult, error) {
	return agentruntime.CompactResult{}, nil
}

func (s *stubRuntime) CancelActiveRun() bool {
	return false
}

func (s *stubRuntime) Events() <-chan agentruntime.RuntimeEvent {
	return s.events
}

func (s *stubRuntime) ListSessions(ctx context.Context) ([]agentruntime.SessionSummary, error) {
	return nil, nil
}

func (s *stubRuntime) LoadSession(ctx context.Context, id string) (agentruntime.Session, error) {
	return agentruntime.Session{}, nil
}

func (s *stubRuntime) SetSessionWorkdir(ctx context.Context, sessionID string, workdir string) (agentruntime.Session, error) {
	return agentruntime.Session{}, nil
}

type stubProviderService struct{}

func (s *stubProviderService) ListProviders(ctx context.Context) ([]config.ProviderCatalogItem, error) {
	return nil, nil
}

func (s *stubProviderService) SelectProvider(ctx context.Context, providerID string) (config.ProviderSelection, error) {
	return config.ProviderSelection{}, nil
}

func (s *stubProviderService) ListModels(ctx context.Context) ([]config.ModelDescriptor, error) {
	return nil, nil
}

func (s *stubProviderService) ListModelsSnapshot(ctx context.Context) ([]config.ModelDescriptor, error) {
	return nil, nil
}

func (s *stubProviderService) SetCurrentModel(ctx context.Context, modelID string) (config.ProviderSelection, error) {
	return config.ProviderSelection{}, nil
}

type spyFactory struct {
	runtimeOut   agentruntime.Runtime
	providerOut  ProviderService
	err          error
	modeSeen     Mode
	runtimeHits  int
	providerHits int
}

func (s *spyFactory) BuildRuntime(mode Mode, current agentruntime.Runtime) (agentruntime.Runtime, error) {
	s.modeSeen = mode
	s.runtimeHits++
	if s.err != nil {
		return nil, s.err
	}
	if s.runtimeOut != nil {
		return s.runtimeOut, nil
	}
	return current, nil
}

func (s *spyFactory) BuildProvider(mode Mode, current ProviderService) (ProviderService, error) {
	s.modeSeen = mode
	s.providerHits++
	if s.err != nil {
		return nil, s.err
	}
	if s.providerOut != nil {
		return s.providerOut, nil
	}
	return current, nil
}

func TestBuildValidatesDependencies(t *testing.T) {
	manager := newTestConfigManager(t)
	runtimeSvc := newStubRuntime()
	providerSvc := &stubProviderService{}

	cases := []struct {
		name    string
		options Options
		wantErr string
	}{
		{
			name: "nil manager",
			options: Options{
				Runtime:         runtimeSvc,
				ProviderService: providerSvc,
			},
			wantErr: "config manager is nil",
		},
		{
			name: "nil runtime",
			options: Options{
				ConfigManager:   manager,
				ProviderService: providerSvc,
			},
			wantErr: "runtime is nil",
		},
		{
			name: "nil provider",
			options: Options{
				ConfigManager: manager,
				Runtime:       runtimeSvc,
			},
			wantErr: "provider service is nil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Build() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestBuildUsesManagerSnapshotWhenConfigNil(t *testing.T) {
	manager := newTestConfigManager(t)
	runtimeSvc := newStubRuntime()
	providerSvc := &stubProviderService{}

	container, err := Build(Options{
		ConfigManager:   manager,
		Runtime:         runtimeSvc,
		ProviderService: providerSvc,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if container.Mode != ModeLive {
		t.Fatalf("expected default mode %q, got %q", ModeLive, container.Mode)
	}
	if container.Config.SelectedProvider != manager.Get().SelectedProvider {
		t.Fatalf("expected config snapshot from manager, got %+v", container.Config)
	}
}

func TestBuildUsesConfigSnapshotAndFactory(t *testing.T) {
	manager := newTestConfigManager(t)
	runtimeSvc := newStubRuntime()
	providerSvc := &stubProviderService{}

	override := manager.Get()
	override.CurrentModel = "custom-model"

	altRuntime := newStubRuntime()
	altProvider := &stubProviderService{}
	factory := &spyFactory{
		runtimeOut:  altRuntime,
		providerOut: altProvider,
	}

	container, err := Build(Options{
		Config:          &override,
		ConfigManager:   manager,
		Runtime:         runtimeSvc,
		ProviderService: providerSvc,
		Mode:            ModeMock,
		Factory:         factory,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	override.CurrentModel = "mutated-after-build"
	if container.Config.CurrentModel != "custom-model" {
		t.Fatalf("expected config clone snapshot, got %q", container.Config.CurrentModel)
	}
	if container.Runtime != altRuntime || container.ProviderService != altProvider {
		t.Fatalf("expected factory outputs to be injected")
	}
	if factory.modeSeen != ModeMock || factory.runtimeHits != 1 || factory.providerHits != 1 {
		t.Fatalf("factory was not invoked as expected: %+v", factory)
	}
}

func TestBuildFactoryError(t *testing.T) {
	manager := newTestConfigManager(t)
	factory := &spyFactory{err: errors.New("boom")}

	_, err := Build(Options{
		ConfigManager:   manager,
		Runtime:         newStubRuntime(),
		ProviderService: &stubProviderService{},
		Factory:         factory,
	})
	if err == nil || !strings.Contains(err.Error(), "build runtime") {
		t.Fatalf("Build() error = %v, want runtime factory error", err)
	}
}

// newTestConfigManager 创建隔离配置目录，返回可用于 bootstrap 单测的配置管理器。
func newTestConfigManager(t *testing.T) *config.Manager {
	t.Helper()

	loader := config.NewLoader(t.TempDir(), config.DefaultConfig())
	manager := config.NewManager(loader)
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatalf("manager.Load() error = %v", err)
	}
	return manager
}
