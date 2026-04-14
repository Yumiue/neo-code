package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"neo-code/internal/app"
	"neo-code/internal/gateway"
)

func TestNewRootCommandPassesWorkdirFlagToLauncher(t *testing.T) {
	originalLauncher := launchRootProgram
	t.Cleanup(func() { launchRootProgram = originalLauncher })

	var captured app.BootstrapOptions
	launchRootProgram = func(ctx context.Context, opts app.BootstrapOptions) error {
		captured = opts
		return nil
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--workdir", `D:\项目\中文目录`})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if captured.Workdir != `D:\项目\中文目录` {
		t.Fatalf("expected workdir to be forwarded, got %q", captured.Workdir)
	}
}

func TestNewRootCommandAllowsEmptyWorkdir(t *testing.T) {
	originalLauncher := launchRootProgram
	t.Cleanup(func() { launchRootProgram = originalLauncher })

	var captured app.BootstrapOptions
	launchRootProgram = func(ctx context.Context, opts app.BootstrapOptions) error {
		captured = opts
		return nil
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if captured.Workdir != "" {
		t.Fatalf("expected empty workdir override, got %q", captured.Workdir)
	}
}

func TestNewRootCommandReturnsLauncherError(t *testing.T) {
	originalLauncher := launchRootProgram
	t.Cleanup(func() { launchRootProgram = originalLauncher })

	expected := errors.New("launch failed")
	launchRootProgram = func(ctx context.Context, opts app.BootstrapOptions) error {
		return expected
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected launcher error %v, got %v", expected, err)
	}
}

func TestExecuteUsesOSArgs(t *testing.T) {
	originalLauncher := launchRootProgram
	originalArgs := os.Args
	t.Cleanup(func() {
		launchRootProgram = originalLauncher
		os.Args = originalArgs
	})

	var captured app.BootstrapOptions
	launchRootProgram = func(ctx context.Context, opts app.BootstrapOptions) error {
		captured = opts
		return nil
	}
	os.Args = []string{"neocode", "--workdir", `D:\项目\中文目录`}

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if captured.Workdir != `D:\项目\中文目录` {
		t.Fatalf("expected Execute to forward workdir, got %q", captured.Workdir)
	}
}

func TestDefaultRootProgramLauncherRunsProgram(t *testing.T) {
	originalNewProgram := newRootProgram
	t.Cleanup(func() { newRootProgram = originalNewProgram })

	cleanedUp := false
	newRootProgram = func(ctx context.Context, opts app.BootstrapOptions) (*tea.Program, func() error, error) {
		model := quitModel{}
		return tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard)), func() error { cleanedUp = true; return nil }, nil
	}

	if err := defaultRootProgramLauncher(context.Background(), app.BootstrapOptions{Workdir: `D:\项目\中文目录`}); err != nil {
		t.Fatalf("defaultRootProgramLauncher() error = %v", err)
	}
	if !cleanedUp {
		t.Fatalf("expected cleanup to be called")
	}
}

func TestDefaultRootProgramLauncherReturnsNewProgramError(t *testing.T) {
	originalNewProgram := newRootProgram
	t.Cleanup(func() { newRootProgram = originalNewProgram })

	expected := errors.New("new program failed")
	newRootProgram = func(ctx context.Context, opts app.BootstrapOptions) (*tea.Program, func() error, error) {
		return nil, nil, expected
	}

	err := defaultRootProgramLauncher(context.Background(), app.BootstrapOptions{})
	if !errors.Is(err, expected) {
		t.Fatalf("expected new program error %v, got %v", expected, err)
	}
}

func TestDefaultRootProgramLauncherReturnsCleanupErrorWhenRunSucceeds(t *testing.T) {
	originalNewProgram := newRootProgram
	t.Cleanup(func() { newRootProgram = originalNewProgram })

	cleanupErr := errors.New("cleanup failed")
	newRootProgram = func(ctx context.Context, opts app.BootstrapOptions) (*tea.Program, func() error, error) {
		model := quitModel{}
		return tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard)), func() error {
			return cleanupErr
		}, nil
	}

	err := defaultRootProgramLauncher(context.Background(), app.BootstrapOptions{})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error %v, got %v", cleanupErr, err)
	}
}

func TestDefaultRootProgramLauncherJoinsRunAndCleanupErrors(t *testing.T) {
	originalNewProgram := newRootProgram
	t.Cleanup(func() { newRootProgram = originalNewProgram })

	runErr := context.Canceled
	cleanupErr := errors.New("cleanup failed")
	newRootProgram = func(ctx context.Context, opts app.BootstrapOptions) (*tea.Program, func() error, error) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		return tea.NewProgram(quitModel{}, tea.WithContext(cancelledCtx), tea.WithInput(nil), tea.WithOutput(io.Discard)), func() error {
			return cleanupErr
		}, nil
	}

	err := defaultRootProgramLauncher(context.Background(), app.BootstrapOptions{})
	if !errors.Is(err, runErr) {
		t.Fatalf("expected joined error to include run error %v, got %v", runErr, err)
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected joined error to include cleanup error %v, got %v", cleanupErr, err)
	}
}

func TestGatewaySubcommandPassesFlagsToRunner(t *testing.T) {
	originalRunner := runGatewayCommand
	t.Cleanup(func() { runGatewayCommand = originalRunner })

	var captured gatewayCommandOptions
	runGatewayCommand = func(ctx context.Context, options gatewayCommandOptions) error {
		captured = options
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{"gateway", "--listen", "  /tmp/gateway.sock  ", "--log-level", " WARN "})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if captured.ListenAddress != "/tmp/gateway.sock" {
		t.Fatalf("listen address = %q, want %q", captured.ListenAddress, "/tmp/gateway.sock")
	}
	if captured.LogLevel != "warn" {
		t.Fatalf("log level = %q, want %q", captured.LogLevel, "warn")
	}
}

func TestGatewaySubcommandRejectsInvalidLogLevel(t *testing.T) {
	command := NewRootCommand()
	command.SetArgs([]string{"gateway", "--log-level", "trace"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected invalid log level error")
	}
	if !strings.Contains(err.Error(), "invalid --log-level") {
		t.Fatalf("error = %v, want contains %q", err, "invalid --log-level")
	}
}

func TestDefaultGatewayCommandRunnerSuccess(t *testing.T) {
	originalNewGatewayServer := newGatewayServer
	t.Cleanup(func() { newGatewayServer = originalNewGatewayServer })

	server := &stubGatewayServer{listenAddress: "stub://gateway"}
	newGatewayServer = func(options gateway.ServerOptions) (gatewayServer, error) {
		return server, nil
	}

	err := defaultGatewayCommandRunner(context.Background(), gatewayCommandOptions{
		ListenAddress: "stub://gateway",
		LogLevel:      "info",
	})
	if err != nil {
		t.Fatalf("defaultGatewayCommandRunner() error = %v", err)
	}
	if !server.serveCalled {
		t.Fatal("expected server Serve to be called")
	}
	if !server.closeCalled {
		t.Fatal("expected server Close to be called")
	}
}

func TestDefaultGatewayCommandRunnerReturnsConstructorError(t *testing.T) {
	originalNewGatewayServer := newGatewayServer
	t.Cleanup(func() { newGatewayServer = originalNewGatewayServer })

	expected := errors.New("new gateway server failed")
	newGatewayServer = func(options gateway.ServerOptions) (gatewayServer, error) {
		return nil, expected
	}

	err := defaultGatewayCommandRunner(context.Background(), gatewayCommandOptions{
		ListenAddress: "stub://gateway",
		LogLevel:      "info",
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected constructor error %v, got %v", expected, err)
	}
}

func TestDefaultGatewayCommandRunnerReturnsServeError(t *testing.T) {
	originalNewGatewayServer := newGatewayServer
	t.Cleanup(func() { newGatewayServer = originalNewGatewayServer })

	expected := errors.New("serve failed")
	server := &stubGatewayServer{
		listenAddress: "stub://gateway",
		serveErr:      expected,
	}
	newGatewayServer = func(options gateway.ServerOptions) (gatewayServer, error) {
		return server, nil
	}

	err := defaultGatewayCommandRunner(context.Background(), gatewayCommandOptions{
		ListenAddress: "stub://gateway",
		LogLevel:      "info",
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected serve error %v, got %v", expected, err)
	}
	if !server.closeCalled {
		t.Fatal("expected server Close to be called")
	}
}

func TestDefaultNewGatewayServer(t *testing.T) {
	server, err := defaultNewGatewayServer(gateway.ServerOptions{
		ListenAddress: "stub://gateway",
	})
	if err != nil {
		t.Fatalf("defaultNewGatewayServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("defaultNewGatewayServer() returned nil server")
	}
}

func TestURLDispatchSubcommandUsesURLFlag(t *testing.T) {
	originalRunner := runURLDispatchCommand
	t.Cleanup(func() { runURLDispatchCommand = originalRunner })

	var captured urlDispatchCommandOptions
	runURLDispatchCommand = func(ctx context.Context, options urlDispatchCommandOptions) error {
		captured = options
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{
		"url-dispatch",
		"--url", "  neocode://review?path=README.md  ",
		"--listen", "  /tmp/gateway.sock  ",
	})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if captured.URL != "neocode://review?path=README.md" {
		t.Fatalf("url = %q, want %q", captured.URL, "neocode://review?path=README.md")
	}
	if captured.ListenAddress != "/tmp/gateway.sock" {
		t.Fatalf("listen address = %q, want %q", captured.ListenAddress, "/tmp/gateway.sock")
	}
}

func TestURLDispatchSubcommandUsesPositionalURL(t *testing.T) {
	originalRunner := runURLDispatchCommand
	t.Cleanup(func() { runURLDispatchCommand = originalRunner })

	var captured urlDispatchCommandOptions
	runURLDispatchCommand = func(ctx context.Context, options urlDispatchCommandOptions) error {
		captured = options
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{"url-dispatch", "neocode://review?path=README.md"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if captured.URL != "neocode://review?path=README.md" {
		t.Fatalf("url = %q, want %q", captured.URL, "neocode://review?path=README.md")
	}
}

func TestURLDispatchSubcommandRejectsInvalidScheme(t *testing.T) {
	command := NewRootCommand()
	command.SetArgs([]string{"url-dispatch", "--url", "http://example.com"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if !strings.Contains(err.Error(), `invalid --url scheme "http"`) {
		t.Fatalf("error = %v, want invalid scheme message", err)
	}
}

func TestURLDispatchSubcommandRejectsMissingActionHost(t *testing.T) {
	command := NewRootCommand()
	command.SetArgs([]string{"url-dispatch", "--url", "neocode://"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected missing action host error")
	}
	if !strings.Contains(err.Error(), "missing action host") {
		t.Fatalf("error = %v, want missing action host message", err)
	}
}

func TestURLDispatchSubcommandRejectsMissingURL(t *testing.T) {
	command := NewRootCommand()
	command.SetArgs([]string{"url-dispatch"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected missing url error")
	}
	if !strings.Contains(err.Error(), "missing required --url or positional <url>") {
		t.Fatalf("error = %v, want missing url message", err)
	}
}

func TestURLDispatchSubcommandDefaultRunnerError(t *testing.T) {
	originalRunner := runURLDispatchCommand
	t.Cleanup(func() { runURLDispatchCommand = originalRunner })
	runURLDispatchCommand = defaultURLDispatchCommandRunner

	command := NewRootCommand()
	command.SetArgs([]string{"url-dispatch", "--url", "neocode://review?path=README.md"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected default runner error")
	}
	if !strings.Contains(err.Error(), "planned in EPIC-GW-02") {
		t.Fatalf("error = %v, want planned message", err)
	}
}

func TestNormalizeDispatchURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		normalized, err := normalizeDispatchURL("  neocode://review?path=README.md  ")
		if err != nil {
			t.Fatalf("normalizeDispatchURL() error = %v", err)
		}
		if normalized != "neocode://review?path=README.md" {
			t.Fatalf("normalized = %q, want %q", normalized, "neocode://review?path=README.md")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := normalizeDispatchURL("://bad-url")
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "invalid --url") {
			t.Fatalf("error = %v, want invalid url message", err)
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		_, err := normalizeDispatchURL("https://example.com")
		if err == nil {
			t.Fatal("expected scheme error")
		}
		if !strings.Contains(err.Error(), "must be neocode") {
			t.Fatalf("error = %v, want scheme message", err)
		}
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := normalizeDispatchURL("neocode://")
		if err == nil {
			t.Fatal("expected missing host error")
		}
		if !strings.Contains(err.Error(), "missing action host") {
			t.Fatalf("error = %v, want missing host message", err)
		}
	})
}

type quitModel struct{}

type stubGatewayServer struct {
	listenAddress string
	serveErr      error
	closeErr      error
	serveCalled   bool
	closeCalled   bool
}

func (s *stubGatewayServer) ListenAddress() string {
	return s.listenAddress
}

func (s *stubGatewayServer) Serve(_ context.Context, _ gateway.RuntimePort) error {
	s.serveCalled = true
	return s.serveErr
}

func (s *stubGatewayServer) Close(_ context.Context) error {
	s.closeCalled = true
	return s.closeErr
}

func (quitModel) Init() tea.Cmd {
	return tea.Quit
}

func (quitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return quitModel{}, nil
}

func (quitModel) View() string {
	return ""
}
