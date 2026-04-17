package state

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "neo-code/internal/config"
	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

func TestCreateCustomProviderSuccess(t *testing.T) {
	restorePersist, restoreDelete, restoreLookup := stubUserEnvOpsForCreateProvider(t)
	defer restorePersist()
	defer restoreDelete()
	defer restoreLookup()

	manager := newSelectionTestManager(t, testDefaultConfig())
	service := NewService(manager, newDriverSupporterStub(), catalogMethodsStub{
		listModels: []providertypes.ModelDescriptor{
			{ID: "custom-model", Name: "custom-model"},
		},
	})

	input := CreateCustomProviderInput{
		Name:      "company-gateway",
		Driver:    provider.DriverOpenAICompat,
		BaseURL:   "https://llm.example.com/v1",
		APIKeyEnv: "COMPANY_GATEWAY_API_KEY",
		APIKey:    "test-key",
		APIStyle:  provider.OpenAICompatibleAPIStyleChatCompletions,
	}

	restore := captureEnvForCreateProvider(t, input.APIKeyEnv)
	defer restore()
	_ = os.Unsetenv(input.APIKeyEnv)

	selection, err := service.CreateCustomProvider(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateCustomProvider() error = %v", err)
	}
	if selection.ProviderID != input.Name {
		t.Fatalf("expected provider %q, got %+v", input.Name, selection)
	}
	if strings.TrimSpace(os.Getenv(input.APIKeyEnv)) != input.APIKey {
		t.Fatalf("expected process env %q to be set", input.APIKeyEnv)
	}

	providerPath := filepath.Join(manager.BaseDir(), "providers", input.Name, "provider.yaml")
	data, readErr := os.ReadFile(providerPath)
	if readErr != nil {
		t.Fatalf("read provider config: %v", readErr)
	}
	providerText := string(data)
	if !strings.Contains(providerText, "api_key_env: "+input.APIKeyEnv) {
		t.Fatalf("expected provider config to persist env name, got %q", providerText)
	}
	if strings.Contains(providerText, input.APIKey) {
		t.Fatalf("provider config should not persist api key, got %q", providerText)
	}
}

func TestCreateCustomProviderRollbackOnSelectFailure(t *testing.T) {
	restorePersist, restoreDelete, restoreLookup := stubUserEnvOpsForCreateProvider(t)
	defer restorePersist()
	defer restoreDelete()
	defer restoreLookup()

	manager := newSelectionTestManager(t, testDefaultConfig())
	service := NewService(manager, newDriverSupporterStub(), errorCatalogStub{err: context.DeadlineExceeded})

	input := CreateCustomProviderInput{
		Name:      "rollback-gateway",
		Driver:    provider.DriverOpenAICompat,
		BaseURL:   "https://llm.example.com/v1",
		APIKeyEnv: "ROLLBACK_GATEWAY_API_KEY",
		APIKey:    "new-key",
		APIStyle:  provider.OpenAICompatibleAPIStyleChatCompletions,
	}

	restore := captureEnvForCreateProvider(t, input.APIKeyEnv)
	defer restore()
	if err := os.Setenv(input.APIKeyEnv, "old-key"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}

	if _, err := service.CreateCustomProvider(context.Background(), input); err == nil {
		t.Fatal("expected CreateCustomProvider() to fail")
	}

	if got := os.Getenv(input.APIKeyEnv); got != "old-key" {
		t.Fatalf("expected process env rollback, got %q", got)
	}
	providerDir := filepath.Join(manager.BaseDir(), "providers", input.Name)
	if _, statErr := os.Stat(providerDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected provider dir rollback, stat err = %v", statErr)
	}
	cfgAfterRollback := manager.Get()
	if _, findErr := cfgAfterRollback.ProviderByName(input.Name); findErr == nil {
		t.Fatalf("expected provider %q to be absent from manager snapshot after rollback", input.Name)
	}
}

func TestCreateCustomProviderRejectsEnvConflicts(t *testing.T) {
	restorePersist, restoreDelete, restoreLookup := stubUserEnvOpsForCreateProvider(t)
	defer restorePersist()
	defer restoreDelete()
	defer restoreLookup()

	manager := newSelectionTestManager(t, testDefaultConfig())
	service := NewService(manager, newDriverSupporterStub(), catalogMethodsStub{
		listModels: []providertypes.ModelDescriptor{{ID: "m1", Name: "m1"}},
	})

	_, err := service.CreateCustomProvider(context.Background(), CreateCustomProviderInput{
		Name:      "conflict-provider",
		Driver:    provider.DriverOpenAICompat,
		BaseURL:   "https://llm.example.com/v1",
		APIKeyEnv: configpkg.OpenAIDefaultAPIKeyEnv,
		APIKey:    "key",
		APIStyle:  provider.OpenAICompatibleAPIStyleChatCompletions,
	})
	if err == nil || !strings.Contains(err.Error(), "duplicates provider") {
		t.Fatalf("expected duplicate env error, got %v", err)
	}
}

func TestCreateCustomProviderRejectsProtectedEnvName(t *testing.T) {
	restorePersist, restoreDelete, restoreLookup := stubUserEnvOpsForCreateProvider(t)
	defer restorePersist()
	defer restoreDelete()
	defer restoreLookup()

	manager := newSelectionTestManager(t, testDefaultConfig())
	service := NewService(manager, newDriverSupporterStub(), catalogMethodsStub{
		listModels: []providertypes.ModelDescriptor{{ID: "m1", Name: "m1"}},
	})

	_, err := service.CreateCustomProvider(context.Background(), CreateCustomProviderInput{
		Name:      "protected-env-provider",
		Driver:    provider.DriverOpenAICompat,
		BaseURL:   "https://llm.example.com/v1",
		APIKeyEnv: "PATH",
		APIKey:    "key",
		APIStyle:  provider.OpenAICompatibleAPIStyleChatCompletions,
	})
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("expected protected env error, got %v", err)
	}
}

func TestCreateCustomProviderRejectsInvalidProviderName(t *testing.T) {
	restorePersist, restoreDelete, restoreLookup := stubUserEnvOpsForCreateProvider(t)
	defer restorePersist()
	defer restoreDelete()
	defer restoreLookup()

	manager := newSelectionTestManager(t, testDefaultConfig())
	service := NewService(manager, newDriverSupporterStub(), catalogMethodsStub{
		listModels: []providertypes.ModelDescriptor{{ID: "m1", Name: "m1"}},
	})

	_, err := service.CreateCustomProvider(context.Background(), CreateCustomProviderInput{
		Name:      "../invalid-provider",
		Driver:    provider.DriverOpenAICompat,
		BaseURL:   "https://llm.example.com/v1",
		APIKeyEnv: "INVALID_PROVIDER_NAME_API_KEY",
		APIKey:    "key",
		APIStyle:  provider.OpenAICompatibleAPIStyleChatCompletions,
	})
	if err == nil || !strings.Contains(err.Error(), "provider name") {
		t.Fatalf("expected invalid provider name error, got %v", err)
	}
}

func captureEnvForCreateProvider(t *testing.T, key string) func() {
	t.Helper()

	value, exists := os.LookupEnv(key)
	return func() {
		if exists {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	}
}

func stubUserEnvOpsForCreateProvider(t *testing.T) (func(), func(), func()) {
	t.Helper()

	prevPersist := persistUserEnvVarForCreate
	prevDelete := deleteUserEnvVarForCreate
	prevLookup := lookupUserEnvVarForCreate

	persistUserEnvVarForCreate = func(key string, value string) error { return nil }
	deleteUserEnvVarForCreate = func(key string) error { return nil }
	lookupUserEnvVarForCreate = func(key string) (string, bool, error) { return "", false, nil }

	return func() { persistUserEnvVarForCreate = prevPersist },
		func() { deleteUserEnvVarForCreate = prevDelete },
		func() { lookupUserEnvVarForCreate = prevLookup }
}
