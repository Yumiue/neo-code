package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"neo-code/internal/ptyproxy"
)

func TestShellCommandUsesFlags(t *testing.T) {
	originalRunner := runShellCommand
	t.Cleanup(func() { runShellCommand = originalRunner })

	var captured shellCommandOptions
	runShellCommand = func(_ context.Context, options shellCommandOptions, _ io.Reader, _ io.Writer, _ io.Writer) error {
		captured = options
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{
		"--workdir", " /repo ",
		"shell",
		"--shell", " /bin/zsh ",
		"--socket", " /tmp/diag.sock ",
		"--gateway-listen", " /tmp/gateway.sock ",
		"--gateway-token-file", " /tmp/auth.json ",
	})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if captured.Workdir != "/repo" {
		t.Fatalf("workdir = %q, want %q", captured.Workdir, "/repo")
	}
	if captured.Shell != "/bin/zsh" {
		t.Fatalf("shell = %q, want %q", captured.Shell, "/bin/zsh")
	}
	if captured.SocketPath != "/tmp/diag.sock" {
		t.Fatalf("socket = %q, want %q", captured.SocketPath, "/tmp/diag.sock")
	}
	if captured.GatewayListenAddress != "/tmp/gateway.sock" {
		t.Fatalf("gateway-listen = %q, want %q", captured.GatewayListenAddress, "/tmp/gateway.sock")
	}
	if captured.GatewayTokenFile != "/tmp/auth.json" {
		t.Fatalf("gateway-token-file = %q, want %q", captured.GatewayTokenFile, "/tmp/auth.json")
	}
}

func TestShellCommandSkipsGlobalPreload(t *testing.T) {
	originalPreload := runGlobalPreload
	originalRunner := runShellCommand
	t.Cleanup(func() { runGlobalPreload = originalPreload })
	t.Cleanup(func() { runShellCommand = originalRunner })

	var preloadCalled bool
	runGlobalPreload = func(context.Context) error {
		preloadCalled = true
		return errors.New("should be skipped")
	}
	runShellCommand = func(context.Context, shellCommandOptions, io.Reader, io.Writer, io.Writer) error {
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{"shell"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if preloadCalled {
		t.Fatal("expected global preload to be skipped for shell command")
	}
}

func TestShellCommandSkipsSilentUpdateCheck(t *testing.T) {
	originalSilentCheck := runSilentUpdateCheck
	originalRunner := runShellCommand
	t.Cleanup(func() { runSilentUpdateCheck = originalSilentCheck })
	t.Cleanup(func() { runShellCommand = originalRunner })

	var checkCalled bool
	runSilentUpdateCheck = func(context.Context) {
		checkCalled = true
	}
	runShellCommand = func(context.Context, shellCommandOptions, io.Reader, io.Writer, io.Writer) error {
		return nil
	}

	command := NewRootCommand()
	command.SetArgs([]string{"shell"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if checkCalled {
		t.Fatal("expected silent update check to be skipped for shell command")
	}
}

func TestDiagCommandSocketPriority(t *testing.T) {
	originalRunner := runDiagCommand
	originalReadEnv := readDiagEnvValue
	t.Cleanup(func() { runDiagCommand = originalRunner })
	t.Cleanup(func() { readDiagEnvValue = originalReadEnv })

	var captured diagCommandOptions
	runDiagCommand = func(_ context.Context, options diagCommandOptions) error {
		captured = options
		return nil
	}
	readDiagEnvValue = func(key string) string {
		if key == ptyproxy.DiagSocketEnv {
			return "/tmp/from-env.sock"
		}
		return ""
	}

	command := NewRootCommand()
	command.SetArgs([]string{"diag", "--socket", " /tmp/from-flag.sock "})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if captured.SocketPath != "/tmp/from-flag.sock" {
		t.Fatalf("socket = %q, want %q", captured.SocketPath, "/tmp/from-flag.sock")
	}
}

func TestDiagCommandUsesEnvFallback(t *testing.T) {
	originalRunner := runDiagCommand
	originalReadEnv := readDiagEnvValue
	t.Cleanup(func() { runDiagCommand = originalRunner })
	t.Cleanup(func() { readDiagEnvValue = originalReadEnv })

	var captured diagCommandOptions
	runDiagCommand = func(_ context.Context, options diagCommandOptions) error {
		captured = options
		return nil
	}
	readDiagEnvValue = func(key string) string {
		if key == ptyproxy.DiagSocketEnv {
			return " /tmp/from-env.sock "
		}
		return ""
	}

	command := NewRootCommand()
	command.SetArgs([]string{"diag"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if captured.SocketPath != "/tmp/from-env.sock" {
		t.Fatalf("socket = %q, want %q", captured.SocketPath, "/tmp/from-env.sock")
	}
}

func TestDiagCommandSocketMissing(t *testing.T) {
	originalReadEnv := readDiagEnvValue
	t.Cleanup(func() { readDiagEnvValue = originalReadEnv })
	readDiagEnvValue = func(string) string { return "" }

	command := NewRootCommand()
	command.SetArgs([]string{"diag"})
	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected missing socket error")
	}
	if !strings.Contains(err.Error(), "--socket") {
		t.Fatalf("error = %v, want contains --socket", err)
	}
	if !strings.Contains(err.Error(), ptyproxy.DiagSocketEnv) {
		t.Fatalf("error = %v, want contains %s", err, ptyproxy.DiagSocketEnv)
	}
}

func TestDiagCommandSkipsGlobalPreload(t *testing.T) {
	originalPreload := runGlobalPreload
	originalRunner := runDiagCommand
	originalReadEnv := readDiagEnvValue
	t.Cleanup(func() { runGlobalPreload = originalPreload })
	t.Cleanup(func() { runDiagCommand = originalRunner })
	t.Cleanup(func() { readDiagEnvValue = originalReadEnv })

	var preloadCalled bool
	runGlobalPreload = func(context.Context) error {
		preloadCalled = true
		return errors.New("should be skipped")
	}
	runDiagCommand = func(context.Context, diagCommandOptions) error { return nil }
	readDiagEnvValue = func(string) string { return "/tmp/diag.sock" }

	command := NewRootCommand()
	command.SetArgs([]string{"diag"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if preloadCalled {
		t.Fatal("expected global preload to be skipped for diag command")
	}
}

func TestDefaultShellCommandRunnerForwardsUnsupportedError(t *testing.T) {
	originalManualShell := runManualShellProxy
	t.Cleanup(func() { runManualShellProxy = originalManualShell })
	runManualShellProxy = func(context.Context, ptyproxy.ManualShellOptions) error {
		return errors.New("manual shell mode is only supported on unix-like systems in phase1")
	}

	err := defaultShellCommandRunner(context.Background(), shellCommandOptions{}, os.Stdin, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected unsupported error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "only supported on unix-like systems") {
		t.Fatalf("error = %v", err)
	}
}

func TestDefaultDiagCommandRunnerForwardsUnsupportedError(t *testing.T) {
	originalSend := sendDiagnoseSignalFn
	t.Cleanup(func() { sendDiagnoseSignalFn = originalSend })
	sendDiagnoseSignalFn = func(context.Context, string) error {
		return errors.New("manual shell mode is only supported on unix-like systems in phase1")
	}

	err := defaultDiagCommandRunner(context.Background(), diagCommandOptions{SocketPath: "/tmp/diag.sock"})
	if err == nil {
		t.Fatal("expected unsupported error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "only supported on unix-like systems") {
		t.Fatalf("error = %v", err)
	}
}
