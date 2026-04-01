package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestExecuteLocalCommandAdditionalBranches(t *testing.T) {
	t.Run("setting reports and updates values", func(t *testing.T) {
		manager := newTestConfigManager(t)
		providerSvc := newTestProviderService(t, manager)

		notice, err := executeLocalCommand(context.Background(), manager, providerSvc, "/setting")
		if err != nil {
			t.Fatalf("unexpected /setting error: %v", err)
		}
		if !strings.Contains(notice, "Settings:") || !strings.Contains(notice, "Provider:") {
			t.Fatalf("expected settings summary, got %q", notice)
		}

		nextDir := filepath.Join(t.TempDir(), "nested")
		notice, err = executeLocalCommand(context.Background(), manager, providerSvc, "/setting workdir "+nextDir)
		if err != nil {
			t.Fatalf("unexpected /setting workdir error: %v", err)
		}
		if !strings.Contains(notice, nextDir) {
			t.Fatalf("expected workdir update notice, got %q", notice)
		}
		if got := manager.Get().Workdir; got != nextDir {
			t.Fatalf("expected workdir %q, got %q", nextDir, got)
		}

		notice, err = executeLocalCommand(context.Background(), manager, providerSvc, "/setting provider gemini")
		if err != nil {
			t.Fatalf("unexpected /setting provider error: %v", err)
		}
		if !strings.Contains(notice, "gemini") {
			t.Fatalf("expected provider switch notice, got %q", notice)
		}
	})

	t.Run("setting validates arguments", func(t *testing.T) {
		manager := newTestConfigManager(t)
		providerSvc := newTestProviderService(t, manager)

		if _, err := executeLocalCommand(context.Background(), manager, providerSvc, "/setting model"); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("expected /setting model usage error, got %v", err)
		}
		if _, err := executeLocalCommand(context.Background(), manager, providerSvc, "/setting nope value"); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("expected unsupported setting error, got %v", err)
		}
	})
}

func TestExecuteFileCommandBranches(t *testing.T) {
	manager := newTestConfigManager(t)
	providerSvc := newTestProviderService(t, manager)
	root := t.TempDir()
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.Workdir = root
		return nil
	}); err != nil {
		t.Fatalf("set temp workdir: %v", err)
	}

	notice, err := executeLocalCommand(context.Background(), manager, providerSvc, "/file write notes.txt hello world")
	if err != nil {
		t.Fatalf("unexpected /file write error: %v", err)
	}
	if !strings.Contains(notice, "notes.txt") {
		t.Fatalf("expected write notice, got %q", notice)
	}

	notice, err = executeLocalCommand(context.Background(), manager, providerSvc, "/file read notes.txt")
	if err != nil {
		t.Fatalf("unexpected /file read error: %v", err)
	}
	if !strings.Contains(notice, "hello world") {
		t.Fatalf("expected file contents, got %q", notice)
	}

	notice, err = executeLocalCommand(context.Background(), manager, providerSvc, "/file list .")
	if err != nil {
		t.Fatalf("unexpected /file list dir error: %v", err)
	}
	if !strings.Contains(notice, "notes.txt") {
		t.Fatalf("expected file listing, got %q", notice)
	}

	notice, err = executeLocalCommand(context.Background(), manager, providerSvc, "/file list notes.txt")
	if err != nil {
		t.Fatalf("unexpected /file list file error: %v", err)
	}
	if !strings.Contains(notice, "File:") {
		t.Fatalf("expected file metadata notice, got %q", notice)
	}

	if _, err := executeLocalCommand(context.Background(), manager, providerSvc, "/file nope"); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected /file usage error, got %v", err)
	}
	if _, err := executeLocalCommand(context.Background(), manager, providerSvc, "/file read ../outside.txt"); err == nil || !strings.Contains(err.Error(), "escapes workspace root") {
		t.Fatalf("expected workspace escape error, got %v", err)
	}
}

func TestCommandHelperFunctions(t *testing.T) {
	t.Run("splitFirstWord handles empty and remainder", func(t *testing.T) {
		if first, rest := splitFirstWord("   "); first != "" || rest != "" {
			t.Fatalf("expected empty split, got %q / %q", first, rest)
		}
		if first, rest := splitFirstWord("alpha beta gamma"); first != "alpha" || rest != "beta gamma" {
			t.Fatalf("unexpected split result %q / %q", first, rest)
		}
	})

	t.Run("indentBlock handles empty output", func(t *testing.T) {
		if got := indentBlock("", "  "); got != "  (no output)" {
			t.Fatalf("unexpected empty indent output %q", got)
		}
	})

	t.Run("shell args cover known shells", func(t *testing.T) {
		if got := shellArgs("powershell", "echo hi"); len(got) < 3 || got[0] != "powershell" {
			t.Fatalf("unexpected powershell args %+v", got)
		}
		if got := shellArgs("bash", "echo hi"); len(got) != 3 || got[0] != "bash" {
			t.Fatalf("unexpected bash args %+v", got)
		}
		if got := shellArgs("sh", "echo hi"); len(got) != 3 || got[0] != "sh" {
			t.Fatalf("unexpected sh args %+v", got)
		}
		if got := shellArgs("unknown", "echo hi"); len(got) < 3 || got[0] != "powershell" {
			t.Fatalf("unexpected default args %+v", got)
		}
	})

	t.Run("resolve workspace and workdir paths", func(t *testing.T) {
		root := t.TempDir()
		inside, err := resolveWorkspacePath(root, "sub\\file.txt")
		if err != nil || !strings.HasSuffix(inside, filepath.Join("sub", "file.txt")) {
			t.Fatalf("unexpected inside path %q / %v", inside, err)
		}
		if _, err := resolveWorkspacePath(root, "..\\escape.txt"); err == nil {
			t.Fatalf("expected workspace escape error")
		}
		if _, err := resolveWorkspacePath(root, ""); err == nil {
			t.Fatalf("expected empty path error")
		}
		if _, err := resolveRequestedWorkdir(root, ""); err == nil {
			t.Fatalf("expected empty workdir error")
		}
		if got, err := resolveRequestedWorkdir(root, "child"); err != nil || !strings.HasSuffix(got, "child") {
			t.Fatalf("unexpected relative workdir %q / %v", got, err)
		}
	})
}

func TestDefaultShellCommandExecutorBranches(t *testing.T) {
	cfg := config.Config{
		Workdir:        t.TempDir(),
		Shell:          "powershell",
		ToolTimeoutSec: 1,
	}

	if _, err := defaultShellCommandExecutor(context.Background(), cfg, ""); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty command error, got %v", err)
	}

	output, err := defaultShellCommandExecutor(context.Background(), cfg, "Write-Output 'hello'")
	if err != nil {
		t.Fatalf("unexpected executor success error: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected hello output, got %q", output)
	}

	if _, err := defaultShellCommandExecutor(context.Background(), cfg, "throw 'boom'"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected command failure, got %v", err)
	}

	start := time.Now()
	if _, err := defaultShellCommandExecutor(context.Background(), cfg, "Start-Sleep -Seconds 2"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout test took too long")
	}
}

func TestLocalCommandCommandWrappers(t *testing.T) {
	manager := newTestConfigManager(t)
	providerSvc := newTestProviderService(t, manager)

	msg := runLocalCommand(manager, providerSvc, "/help")()
	result, ok := msg.(localCommandResultMsg)
	if !ok || result.err != nil || !strings.Contains(result.notice, "Available slash commands") {
		t.Fatalf("expected help command result, got %+v", msg)
	}

	msg = runProviderSelection(providerSvc, "missing-provider")()
	result, ok = msg.(localCommandResultMsg)
	if !ok || result.err == nil {
		t.Fatalf("expected provider selection error, got %+v", msg)
	}
}

func TestExecuteStatusRunAndGitErrors(t *testing.T) {
	previousExecutor := shellCommandExecutor
	t.Cleanup(func() { shellCommandExecutor = previousExecutor })
	shellCommandExecutor = func(ctx context.Context, cfg config.Config, command string) (string, error) {
		return "", errors.New("executor boom")
	}

	manager := newTestConfigManager(t)

	if notice, err := executeStatusCommand(context.Background(), manager); err != nil || !strings.Contains(notice, "Git: unavailable") {
		t.Fatalf("expected status fallback, got notice=%q err=%v", notice, err)
	}
	if _, err := executeRunCommand(context.Background(), manager, ""); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected /run usage error, got %v", err)
	}
	if _, err := executeRunCommand(context.Background(), manager, "echo hi"); err == nil || !strings.Contains(err.Error(), "executor boom") {
		t.Fatalf("expected /run executor error, got %v", err)
	}
	if _, err := executeGitCommand(context.Background(), manager, ""); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected /git usage error, got %v", err)
	}
	if _, err := executeGitCommand(context.Background(), manager, "status"); err == nil || !strings.Contains(err.Error(), "executor boom") {
		t.Fatalf("expected /git executor error, got %v", err)
	}
}

func TestExecuteFileCommandContextError(t *testing.T) {
	manager := newTestConfigManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := executeFileCommand(ctx, manager, "list ."); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", err)
	}
}

func TestExecuteProviderCommandErrors(t *testing.T) {
	manager := newTestConfigManager(t)
	providerSvc := newTestProviderService(t, manager)

	if _, err := executeProviderCommand(context.Background(), providerSvc, ""); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected provider usage error, got %v", err)
	}
}
