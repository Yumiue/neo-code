package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"neo-code/internal/config"
)

func TestExecuteLocalCommand(t *testing.T) {
	previousExecutor := shellCommandExecutor
	t.Cleanup(func() { shellCommandExecutor = previousExecutor })
	shellCommandExecutor = func(ctx context.Context, cfg config.Config, command string) (string, error) {
		return "stubbed: " + command, nil
	}

	tests := []struct {
		name      string
		command   string
		expectErr string
		assert    func(t *testing.T, manager *config.Manager, notice string)
	}{
		{
			name:    "help lists slash commands",
			command: "/help",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				if !strings.Contains(notice, slashUsageHelp) || !strings.Contains(notice, slashUsageExit) {
					t.Fatalf("expected help output to list slash commands, got %q", notice)
				}
			},
		},
		{
			name:    "status includes workspace details",
			command: "/status",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				if !strings.Contains(notice, "Workspace status:") || !strings.Contains(notice, "stubbed: git status --short --branch") {
					t.Fatalf("expected status output, got %q", notice)
				}
			},
		},
		{
			name:    "run executes shell command",
			command: "/run echo hi",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				if !strings.Contains(notice, "stubbed: echo hi") {
					t.Fatalf("expected run output, got %q", notice)
				}
			},
		},
		{
			name:    "git executes git command",
			command: "/git status",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				if !strings.Contains(notice, "stubbed: git status") {
					t.Fatalf("expected git output, got %q", notice)
				}
			},
		},
		{
			name:    "provider switches current provider",
			command: "/provider gemini",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				cfg := manager.Get()
				if cfg.SelectedProvider != config.GeminiName {
					t.Fatalf("expected selected provider gemini, got %q", cfg.SelectedProvider)
				}
				if !strings.Contains(notice, "Current provider switched") {
					t.Fatalf("expected provider switch notice, got %q", notice)
				}
			},
		},
		{
			name:    "set url updates selected provider",
			command: "/set url https://test.example/v1",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				cfg := manager.Get()
				selected, err := cfg.SelectedProviderConfig()
				if err != nil {
					t.Fatalf("SelectedProviderConfig() error = %v", err)
				}
				if selected.BaseURL != "https://test.example/v1" {
					t.Fatalf("expected updated base url, got %q", selected.BaseURL)
				}
				if !strings.Contains(notice, "Base URL updated") {
					t.Fatalf("expected update notice, got %q", notice)
				}
			},
		},
		{
			name:    "set model updates current model",
			command: "/set model gpt-4.1",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				cfg := manager.Get()
				if cfg.CurrentModel != "gpt-4.1" {
					t.Fatalf("expected current model gpt-4.1, got %q", cfg.CurrentModel)
				}
				if !strings.Contains(notice, "Current model switched") {
					t.Fatalf("expected model switch notice, got %q", notice)
				}
			},
		},
		{
			name:    "set key updates process env",
			command: "/set key secret-key",
			assert: func(t *testing.T, manager *config.Manager, notice string) {
				t.Helper()
				if got := strings.TrimSpace(os.Getenv(config.OpenAIDefaultAPIKeyEnv)); got != "secret-key" {
					t.Fatalf("expected env to be reloaded, got %q", got)
				}
				if !strings.Contains(notice, "updated for the current process") {
					t.Fatalf("expected key reload notice, got %q", notice)
				}
			},
		},
		{
			name:      "provider requires an argument",
			command:   "/provider",
			expectErr: "usage:",
		},
		{
			name:      "unknown command is rejected",
			command:   "/unknown",
			expectErr: `unknown command "/unknown"`,
		},
		{
			name:      "set usage requires enough arguments",
			command:   "/set url",
			expectErr: "usage:",
		},
		{
			name:      "invalid url is rejected",
			command:   "/set url not-a-url",
			expectErr: "invalid url",
		},
		{
			name:      "empty command is rejected",
			command:   "   ",
			expectErr: "empty command",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			manager := newTestConfigManager(t)
			providerSvc := newTestProviderService(t, manager)
			notice, err := executeLocalCommand(context.Background(), manager, providerSvc, tt.command)
			if tt.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectErr) {
					t.Fatalf("expected error containing %q, got %v", tt.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.assert != nil {
				tt.assert(t, manager, notice)
			}
		})
	}
}

func TestMatchingSlashCommands(t *testing.T) {
	t.Parallel()

	app := App{}
	tests := []struct {
		name        string
		input       string
		expectCount int
		expectUsage string
	}{
		{
			name:        "non slash input returns no suggestions",
			input:       "hello",
			expectCount: 0,
		},
		{
			name:        "bare slash returns all commands",
			input:       "/",
			expectCount: len(builtinSlashCommands),
			expectUsage: slashUsageHelp,
		},
		{
			name:        "prefix narrows suggestions",
			input:       "/mo",
			expectCount: 1,
			expectUsage: slashUsageModel,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := app.matchingSlashCommands(tt.input)
			if len(got) != tt.expectCount {
				t.Fatalf("expected %d suggestions, got %d", tt.expectCount, len(got))
			}
			if tt.expectUsage != "" && (len(got) == 0 || got[0].Command.Usage != tt.expectUsage && !containsUsage(got, tt.expectUsage)) {
				t.Fatalf("expected suggestions to contain %q, got %+v", tt.expectUsage, got)
			}
		})
	}
}

func containsUsage(suggestions []commandSuggestion, usage string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Command.Usage == usage {
			return true
		}
	}
	return false
}
