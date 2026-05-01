//go:build !windows

package ptyproxy

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
)

func assertNoBareLineFeed(t *testing.T, text string) {
	t.Helper()
	for index := 0; index < len(text); index++ {
		if text[index] == '\n' && (index == 0 || text[index-1] != '\r') {
			t.Fatalf("output contains bare LF at index %d: %q", index, text)
		}
	}
}

func TestBuildShellCommandInjectsDiagEnv(t *testing.T) {
	t.Setenv(DiagSocketEnv, "/tmp/old.sock")
	command := buildShellCommand("/bin/bash", ManualShellOptions{
		Workdir: "/tmp",
	}, "/tmp/new.sock")

	if command.Path != "/bin/bash" {
		t.Fatalf("command.Path = %q, want %q", command.Path, "/bin/bash")
	}
	if command.Dir != "/tmp" {
		t.Fatalf("command.Dir = %q, want %q", command.Dir, "/tmp")
	}

	var entries []string
	for _, item := range command.Env {
		if strings.HasPrefix(item, DiagSocketEnv+"=") {
			entries = append(entries, item)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("diag env entries len = %d, want 1", len(entries))
	}
	if entries[0] != DiagSocketEnv+"=/tmp/new.sock" {
		t.Fatalf("diag env entry = %q, want %q", entries[0], DiagSocketEnv+"=/tmp/new.sock")
	}
}

func TestSendDiagnoseSignal(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "diag.sock")
	listener, resolvedPath, err := listenDiagSocket(socketPath)
	if err != nil {
		t.Fatalf("listenDiagSocket() error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(resolvedPath)
	})

	payloadCh := make(chan string, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			payloadCh <- "accept-error:" + acceptErr.Error()
			return
		}
		defer connection.Close()
		buffer := make([]byte, 128)
		n, readErr := connection.Read(buffer)
		if readErr != nil {
			payloadCh <- "read-error:" + readErr.Error()
			return
		}
		payloadCh <- string(buffer[:n])
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := SendDiagnoseSignal(ctx, resolvedPath); err != nil {
		t.Fatalf("SendDiagnoseSignal() error = %v", err)
	}

	select {
	case payload := <-payloadCh:
		if payload != diagSignalPayload {
			t.Fatalf("payload = %q, want %q", payload, diagSignalPayload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for payload")
	}
}

func TestSendDiagnoseSignalWaitsForServerCompletion(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "diag-wait.sock")
	listener, resolvedPath, err := listenDiagSocket(socketPath)
	if err != nil {
		t.Fatalf("listenDiagSocket() error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(resolvedPath)
	})

	const serverDelay = 120 * time.Millisecond
	serverErrCh := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer connection.Close()
		buffer := make([]byte, 128)
		n, readErr := connection.Read(buffer)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if string(buffer[:n]) != diagSignalPayload {
			serverErrCh <- errors.New("unexpected payload")
			return
		}
		time.Sleep(serverDelay)
		_, writeErr := connection.Write([]byte(diagSignalAckPayload))
		serverErrCh <- writeErr
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if err := SendDiagnoseSignal(ctx, resolvedPath); err != nil {
		t.Fatalf("SendDiagnoseSignal() error = %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < serverDelay {
		t.Fatalf("SendDiagnoseSignal returned too early: elapsed=%s, want >= %s", elapsed, serverDelay)
	}

	select {
	case serverErr := <-serverErrCh:
		if serverErr != nil {
			t.Fatalf("server error = %v", serverErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server completion")
	}
}

func TestListenDiagSocketRecoversStaleSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "stale.sock")
	staleListener, resolvedPath, err := listenDiagSocket(socketPath)
	if err != nil {
		t.Fatalf("prepare stale listener error = %v", err)
	}
	_ = staleListener.Close()

	listener, _, err := listenDiagSocket(resolvedPath)
	if err != nil {
		t.Fatalf("listenDiagSocket() with stale socket error = %v", err)
	}
	_ = listener.Close()
	_ = os.Remove(resolvedPath)
}

func TestCleanupStaleSocketRejectsRegularFile(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "not-socket.sock")
	if err := os.WriteFile(socketPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file error = %v", err)
	}
	err := cleanupStaleSocket(socketPath)
	if err == nil {
		t.Fatal("expected non-socket error")
	}
	if !strings.Contains(err.Error(), "not socket") {
		t.Fatalf("error = %v, want contains %q", err, "not socket")
	}
}

func TestRunSingleDiagnosisGatewayUnavailableDoesNotPanic(t *testing.T) {
	buffer := NewUTF8RingBuffer(1024)
	_, _ = buffer.Write([]byte("中文日志 + \u001b[31merror\u001b[0m"))

	output := &bytes.Buffer{}
	runSingleDiagnosis(output, buffer, ManualShellOptions{
		Workdir:              t.TempDir(),
		Shell:                "/bin/bash",
		GatewayListenAddress: filepath.Join(t.TempDir(), "missing-gateway.sock"),
		GatewayTokenFile:     filepath.Join(t.TempDir(), "missing-auth.json"),
	}, filepath.Join(t.TempDir(), "diag.sock"))

	if !strings.Contains(output.String(), "NeoCode Diagnosis") {
		t.Fatalf("output = %q, want contains %q", output.String(), "NeoCode Diagnosis")
	}
	assertNoBareLineFeed(t, output.String())
}

func TestPrintProxyInitializedBanner(t *testing.T) {
	buffer := &bytes.Buffer{}
	printProxyInitializedBanner(buffer)
	if buffer.String() != proxyInitializedBanner+"\r\n" {
		t.Fatalf("banner output = %q, want %q", buffer.String(), proxyInitializedBanner+"\\r\\n")
	}
}

func TestPrintProxyExitedBanner(t *testing.T) {
	buffer := &bytes.Buffer{}
	printProxyExitedBanner(buffer)
	if buffer.String() != "\r\n"+proxyExitedBanner+"\r\n" {
		t.Fatalf("banner output = %q, want %q", buffer.String(), "\\r\\n"+proxyExitedBanner+"\\r\\n")
	}
}

func TestRenderDiagnosisUsesCRLFLineEndings(t *testing.T) {
	buffer := &bytes.Buffer{}
	renderDiagnosis(buffer, "", false)
	output := buffer.String()
	assertNoBareLineFeed(t, output)
	if !strings.Contains(output, "\r\n") {
		t.Fatalf("renderDiagnosis output = %q, want contains CRLF", output)
	}
}

func TestEnableHostTerminalRawModeSkipsNonTerminal(t *testing.T) {
	originalInput := hostTerminalInput
	originalIsTerminal := isTerminalFD
	originalMakeRaw := makeRawTerminal
	originalRestore := restoreTerminal
	t.Cleanup(func() {
		hostTerminalInput = originalInput
		isTerminalFD = originalIsTerminal
		makeRawTerminal = originalMakeRaw
		restoreTerminal = originalRestore
	})

	filePath := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(filePath, []byte("stdin"), 0o600); err != nil {
		t.Fatalf("write stdin file error = %v", err)
	}
	inputFile, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open stdin file error = %v", err)
	}
	defer inputFile.Close()

	hostTerminalInput = inputFile
	isTerminalFD = func(int) bool { return false }

	makeRawCalled := false
	makeRawTerminal = func(int) (*term.State, error) {
		makeRawCalled = true
		return &term.State{}, nil
	}
	restoreFn, err := enableHostTerminalRawMode()
	if err != nil {
		t.Fatalf("enableHostTerminalRawMode() error = %v", err)
	}
	if makeRawCalled {
		t.Fatal("makeRawTerminal should not be called for non-terminal input")
	}
	if restoreErr := restoreFn(); restoreErr != nil {
		t.Fatalf("restoreFn() error = %v", restoreErr)
	}
}

func TestEnableHostTerminalRawModeCallsMakeRawAndRestore(t *testing.T) {
	originalInput := hostTerminalInput
	originalIsTerminal := isTerminalFD
	originalMakeRaw := makeRawTerminal
	originalRestore := restoreTerminal
	t.Cleanup(func() {
		hostTerminalInput = originalInput
		isTerminalFD = originalIsTerminal
		makeRawTerminal = originalMakeRaw
		restoreTerminal = originalRestore
	})

	filePath := filepath.Join(t.TempDir(), "stdin-terminal.txt")
	if err := os.WriteFile(filePath, []byte("stdin"), 0o600); err != nil {
		t.Fatalf("write stdin file error = %v", err)
	}
	inputFile, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open stdin file error = %v", err)
	}
	defer inputFile.Close()
	hostTerminalInput = inputFile

	expectedFD := int(inputFile.Fd())
	state := &term.State{}
	makeRawCalled := false
	restoreCalled := false

	isTerminalFD = func(fd int) bool {
		if fd != expectedFD {
			t.Fatalf("isTerminal fd = %d, want %d", fd, expectedFD)
		}
		return true
	}
	makeRawTerminal = func(fd int) (*term.State, error) {
		if fd != expectedFD {
			t.Fatalf("makeRaw fd = %d, want %d", fd, expectedFD)
		}
		makeRawCalled = true
		return state, nil
	}
	restoreTerminal = func(fd int, restored *term.State) error {
		if fd != expectedFD {
			t.Fatalf("restore fd = %d, want %d", fd, expectedFD)
		}
		if restored != state {
			t.Fatalf("restore state pointer mismatch")
		}
		restoreCalled = true
		return nil
	}

	restoreFn, err := enableHostTerminalRawMode()
	if err != nil {
		t.Fatalf("enableHostTerminalRawMode() error = %v", err)
	}
	if !makeRawCalled {
		t.Fatal("expected makeRawTerminal to be called")
	}
	if restoreErr := restoreFn(); restoreErr != nil {
		t.Fatalf("restoreFn() error = %v", restoreErr)
	}
	if !restoreCalled {
		t.Fatal("expected restoreTerminal to be called")
	}
}

func TestEnableHostTerminalRawModeMakeRawError(t *testing.T) {
	originalInput := hostTerminalInput
	originalIsTerminal := isTerminalFD
	originalMakeRaw := makeRawTerminal
	originalRestore := restoreTerminal
	t.Cleanup(func() {
		hostTerminalInput = originalInput
		isTerminalFD = originalIsTerminal
		makeRawTerminal = originalMakeRaw
		restoreTerminal = originalRestore
	})

	filePath := filepath.Join(t.TempDir(), "stdin-error.txt")
	if err := os.WriteFile(filePath, []byte("stdin"), 0o600); err != nil {
		t.Fatalf("write stdin file error = %v", err)
	}
	inputFile, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open stdin file error = %v", err)
	}
	defer inputFile.Close()
	hostTerminalInput = inputFile

	isTerminalFD = func(int) bool { return true }
	makeRawTerminal = func(int) (*term.State, error) {
		return nil, errors.New("raw mode failed")
	}

	_, err = enableHostTerminalRawMode()
	if err == nil {
		t.Fatal("expected raw mode error")
	}
	if !strings.Contains(err.Error(), "set host terminal raw mode") {
		t.Fatalf("error = %v, want contains %q", err, "set host terminal raw mode")
	}
}
