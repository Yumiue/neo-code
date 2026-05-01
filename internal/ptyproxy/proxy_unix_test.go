//go:build !windows

package ptyproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"neo-code/internal/gateway"
	"neo-code/internal/gateway/protocol"
	"neo-code/internal/tools"

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

func findTestShellPath(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/bash",
		"/usr/bin/bash",
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skip("no usable shell executable found for PTY test")
	return ""
}

func writeGatewayRPCResult(responseWriter *json.Encoder, id json.RawMessage, result any) error {
	response, rpcError := protocol.NewJSONRPCResultResponse(id, result)
	if rpcError != nil {
		return fmt.Errorf("build jsonrpc response failed: %s", strings.TrimSpace(rpcError.Message))
	}
	if err := responseWriter.Encode(response); err != nil {
		return fmt.Errorf("encode jsonrpc response failed: %w", err)
	}
	return nil
}

func writeGatewayTokenFile(t *testing.T, tokenPath string, token string) {
	t.Helper()
	payload := map[string]any{
		"version":    1,
		"token":      token,
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal token payload error = %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(tokenPath, raw, 0o600); err != nil {
		t.Fatalf("write token file error = %v", err)
	}
}

func withRawModeDisabledForTest(t *testing.T) {
	t.Helper()
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

	hostTerminalInput = nil
	isTerminalFD = func(int) bool { return false }
	makeRawTerminal = func(int) (*term.State, error) {
		return nil, errors.New("makeRawTerminal should not be called when hostTerminalInput is nil")
	}
	restoreTerminal = func(int, *term.State) error { return nil }
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

func TestRunManualShellSuccess(t *testing.T) {
	withRawModeDisabledForTest(t)

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	workdir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := RunManualShell(ctx, ManualShellOptions{
		Workdir:              workdir,
		Shell:                findTestShellPath(t),
		SocketPath:           filepath.Join(t.TempDir(), "diag.sock"),
		GatewayListenAddress: filepath.Join(t.TempDir(), "missing-gateway.sock"),
		GatewayTokenFile:     filepath.Join(t.TempDir(), "missing-auth.json"),
		Stdin:                strings.NewReader("exit 0\n"),
		Stdout:               stdoutBuffer,
		Stderr:               stderrBuffer,
	})
	if err != nil {
		t.Fatalf("RunManualShell() error = %v", err)
	}

	output := stdoutBuffer.String()
	if !strings.Contains(output, proxyInitializedBanner) {
		t.Fatalf("stdout = %q, want contains %q", output, proxyInitializedBanner)
	}
	if !strings.Contains(output, proxyExitedBanner) {
		t.Fatalf("stdout = %q, want contains %q", output, proxyExitedBanner)
	}
	assertNoBareLineFeed(t, output)
}

func TestRunManualShellFailsOnMissingShell(t *testing.T) {
	withRawModeDisabledForTest(t)

	err := RunManualShell(context.Background(), ManualShellOptions{
		Workdir:    t.TempDir(),
		Shell:      filepath.Join(t.TempDir(), "missing-shell"),
		SocketPath: filepath.Join(t.TempDir(), "diag.sock"),
		Stdin:      strings.NewReader("exit 0\n"),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected shell start error")
	}
	if !strings.Contains(err.Error(), "start pty shell") {
		t.Fatalf("error = %v, want contains %q", err, "start pty shell")
	}
}

func TestRunManualShellFailsOnInvalidSocketPath(t *testing.T) {
	withRawModeDisabledForTest(t)

	workspace := t.TempDir()
	blockingFile := filepath.Join(workspace, "blocking-file")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocking file error = %v", err)
	}

	err := RunManualShell(context.Background(), ManualShellOptions{
		Workdir:    workspace,
		Shell:      findTestShellPath(t),
		SocketPath: filepath.Join(blockingFile, "diag.sock"),
		Stdin:      strings.NewReader("exit 0\n"),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected socket path error")
	}
	if !strings.Contains(err.Error(), "create socket directory") {
		t.Fatalf("error = %v, want contains %q", err, "create socket directory")
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

func TestRunSingleDiagnosisGatewayHappyPath(t *testing.T) {
	tempDir := t.TempDir()
	gatewaySocketPath := filepath.Join(tempDir, "gateway.sock")
	listener, err := net.Listen("unix", gatewaySocketPath)
	if err != nil {
		t.Fatalf("listen gateway socket error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(gatewaySocketPath)
	})

	tokenFile := filepath.Join(tempDir, "auth.json")
	const expectedToken = "diag-test-token"
	writeGatewayTokenFile(t, tokenFile, expectedToken)

	serverErrCh := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- fmt.Errorf("accept gateway connection: %w", acceptErr)
			return
		}
		defer connection.Close()

		requestReader := json.NewDecoder(connection)
		responseWriter := json.NewEncoder(connection)

		var authRequest protocol.JSONRPCRequest
		if err := requestReader.Decode(&authRequest); err != nil {
			serverErrCh <- fmt.Errorf("decode auth request: %w", err)
			return
		}
		if authRequest.Method != protocol.MethodGatewayAuthenticate {
			serverErrCh <- fmt.Errorf("auth method = %q, want %q", authRequest.Method, protocol.MethodGatewayAuthenticate)
			return
		}

		var authParams protocol.AuthenticateParams
		if err := json.Unmarshal(authRequest.Params, &authParams); err != nil {
			serverErrCh <- fmt.Errorf("decode auth params: %w", err)
			return
		}
		if authParams.Token != expectedToken {
			serverErrCh <- fmt.Errorf("auth token = %q, want %q", authParams.Token, expectedToken)
			return
		}
		if err := writeGatewayRPCResult(responseWriter, authRequest.ID, gateway.MessageFrame{
			Type:   gateway.FrameTypeAck,
			Action: gateway.FrameActionAuthenticate,
		}); err != nil {
			serverErrCh <- err
			return
		}

		var executeRequest protocol.JSONRPCRequest
		if err := requestReader.Decode(&executeRequest); err != nil {
			serverErrCh <- fmt.Errorf("decode execute request: %w", err)
			return
		}
		if executeRequest.Method != protocol.MethodGatewayExecuteSystemTool {
			serverErrCh <- fmt.Errorf(
				"execute method = %q, want %q",
				executeRequest.Method,
				protocol.MethodGatewayExecuteSystemTool,
			)
			return
		}

		var executeParams protocol.ExecuteSystemToolParams
		if err := json.Unmarshal(executeRequest.Params, &executeParams); err != nil {
			serverErrCh <- fmt.Errorf("decode execute params: %w", err)
			return
		}
		if executeParams.ToolName != tools.ToolNameDiagnose {
			serverErrCh <- fmt.Errorf("tool_name = %q, want %q", executeParams.ToolName, tools.ToolNameDiagnose)
			return
		}

		responsePayload := tools.ToolResult{
			Content: `{"confidence":0.95,"root_cause":"disk full","investigation_commands":["df -h"],"fix_commands":["rm -rf /tmp/*"]}`,
			IsError: false,
		}
		if err := writeGatewayRPCResult(responseWriter, executeRequest.ID, gateway.MessageFrame{
			Type:    gateway.FrameTypeAck,
			Action:  gateway.FrameActionExecuteSystemTool,
			Payload: responsePayload,
		}); err != nil {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	logBuffer := NewUTF8RingBuffer(1024)
	_, _ = logBuffer.Write([]byte("诊断样本日志"))
	outputBuffer := &bytes.Buffer{}
	runSingleDiagnosis(outputBuffer, logBuffer, ManualShellOptions{
		Workdir:              tempDir,
		Shell:                findTestShellPath(t),
		GatewayListenAddress: gatewaySocketPath,
		GatewayTokenFile:     tokenFile,
	}, filepath.Join(tempDir, "diag.sock"))

	select {
	case serverErr := <-serverErrCh:
		if serverErr != nil {
			t.Fatalf("gateway mock server error = %v", serverErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for gateway mock server")
	}

	output := outputBuffer.String()
	if !strings.Contains(output, "NeoCode Diagnosis") {
		t.Fatalf("output = %q, want contains %q", output, "NeoCode Diagnosis")
	}
	if !strings.Contains(output, "根因: disk full") {
		t.Fatalf("output = %q, want contains %q", output, "根因: disk full")
	}
	if !strings.Contains(output, "建议排查命令") {
		t.Fatalf("output = %q, want contains %q", output, "建议排查命令")
	}
	if !strings.Contains(output, "建议修复命令") {
		t.Fatalf("output = %q, want contains %q", output, "建议修复命令")
	}
	assertNoBareLineFeed(t, output)
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

func TestResolveShellPathDefaultsToBinBash(t *testing.T) {
	t.Setenv("SHELL", "")
	path := resolveShellPath("")
	if path != "/bin/bash" {
		t.Fatalf("resolveShellPath() = %q, want %q", path, "/bin/bash")
	}
}

func TestResolveShellPathUsesShellEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	path := resolveShellPath("")
	if path != "/bin/zsh" {
		t.Fatalf("resolveShellPath() = %q, want %q", path, "/bin/zsh")
	}
}

func TestResolveShellPathPrefersExplicit(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	path := resolveShellPath("/bin/fish")
	if path != "/bin/fish" {
		t.Fatalf("resolveShellPath() = %q, want %q", path, "/bin/fish")
	}
}

func TestIsClosedNetworkErrorNil(t *testing.T) {
	if isClosedNetworkError(nil) {
		t.Fatal("isClosedNetworkError(nil) should be false")
	}
}

func TestIsClosedNetworkErrorDetectsNetErrClosed(t *testing.T) {
	if !isClosedNetworkError(net.ErrClosed) {
		t.Fatal("isClosedNetworkError(net.ErrClosed) should be true")
	}
}

func TestIsClosedNetworkErrorDetectsStringMatch(t *testing.T) {
	err := errors.New("use of closed network connection")
	if !isClosedNetworkError(err) {
		t.Fatal("isClosedNetworkError() should be true for closed connection string")
	}
}

func TestIsClosedNetworkErrorNonMatch(t *testing.T) {
	err := errors.New("some other error")
	if isClosedNetworkError(err) {
		t.Fatal("isClosedNetworkError() should be false for unrelated error")
	}
}

func TestSerializedWriterNilWriter(t *testing.T) {
	var w *serializedWriter
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() on nil receiver error = %v", err)
	}
	if n != len("hello") {
		t.Fatalf("Write() n = %d, want %d", n, len("hello"))
	}
}

func TestSerializedWriterNilLock(t *testing.T) {
	var buffer bytes.Buffer
	w := &serializedWriter{writer: &buffer, lock: nil}
	n, err := w.Write([]byte("data"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 4 {
		t.Fatalf("Write() n = %d, want 4", n)
	}
	if buffer.String() != "data" {
		t.Fatalf("buffer = %q, want %q", buffer.String(), "data")
	}
}

func TestSerializedWriterWithLock(t *testing.T) {
	var buffer bytes.Buffer
	var mu sync.Mutex
	w := &serializedWriter{writer: &buffer, lock: &mu}
	n, err := w.Write([]byte("locked-write"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len("locked-write") {
		t.Fatalf("Write() n = %d, want %d", n, len("locked-write"))
	}
	if buffer.String() != "locked-write" {
		t.Fatalf("buffer = %q, want %q", buffer.String(), "locked-write")
	}
}

func TestWriteProxyTextNilWriter(t *testing.T) {
	writeProxyText(nil, "text")
	writeProxyLine(nil, "text")
}

func TestWriteProxyTextEmptyText(t *testing.T) {
	var buffer bytes.Buffer
	writeProxyText(&buffer, "")
	if buffer.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buffer.String())
	}
}

func TestWriteProxyTextNormalizesLineEndings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\nworld", "hello\r\nworld"},
		{"hello\r\nworld", "hello\r\nworld"},
		{"hello\rworld", "hello\r\nworld"},
		{"line1\nline2\nline3", "line1\r\nline2\r\nline3"},
	}
	for _, tt := range tests {
		var buffer bytes.Buffer
		writeProxyText(&buffer, tt.input)
		if buffer.String() != tt.expected {
			t.Fatalf("writeProxyText(%q) = %q, want %q", tt.input, buffer.String(), tt.expected)
		}
	}
}

func TestWriteProxyLine(t *testing.T) {
	var buffer bytes.Buffer
	writeProxyLine(&buffer, "header")
	// writeProxyText with "\n" appended → normalized to "\r\n"
	if buffer.String() != "header\r\n" {
		t.Fatalf("writeProxyLine output = %q, want %q", buffer.String(), "header\\r\\n")
	}
}

func TestWriteProxyf(t *testing.T) {
	var buffer bytes.Buffer
	writeProxyf(&buffer, "count=%d, name=%s", 42, "test")
	expected := "count=42, name=test"
	// writeProxyf → writeProxyText which normalizes
	got := buffer.String()
	if got != expected {
		t.Fatalf("writeProxyf output = %q, want %q", got, expected)
	}
}

func TestSyncPTYWindowSizeNilPTY(t *testing.T) {
	syncPTYWindowSize(nil, nil)
}

func TestWatchPTYWindowResizeNilPTY(t *testing.T) {
	stop := watchPTYWindowResize(nil, nil)
	stop()
}

func TestDecodeToolResult(t *testing.T) {
	result, err := decodeToolResult(map[string]any{
		"content": "ok",
		"isError": false,
	})
	if err != nil {
		t.Fatalf("decodeToolResult() error = %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("Content = %q, want %q", result.Content, "ok")
	}
	if result.IsError {
		t.Fatal("IsError should be false")
	}
}

func TestDecodeToolResultInvalidPayload(t *testing.T) {
	_, err := decodeToolResult([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestDecodeToolResultFromStruct(t *testing.T) {
	result, err := decodeToolResult(map[string]any{"content": "from-map", "isError": true})
	if err != nil {
		t.Fatalf("decodeToolResult() error = %v", err)
	}
	if result.Content != "from-map" {
		t.Fatalf("Content = %q, want %q", result.Content, "from-map")
	}
	if !result.IsError {
		t.Fatal("IsError should be true")
	}
}

func TestRenderDiagnosisNilOutput(t *testing.T) {
	renderDiagnosis(nil, "some content", false)
}

func TestRenderDiagnosisEmptyContent(t *testing.T) {
	var buffer bytes.Buffer
	renderDiagnosis(&buffer, "", false)
	output := buffer.String()
	if !strings.Contains(output, "无可用诊断内容") {
		t.Fatalf("output = %q, want contains %q", output, "无可用诊断内容")
	}
	assertNoBareLineFeed(t, output)
}

func TestRenderDiagnosisErrorHeader(t *testing.T) {
	var buffer bytes.Buffer
	renderDiagnosis(&buffer, `{"confidence":0.9,"rootCause":"OOM","fixCommands":["free -h"]}`, true)
	output := buffer.String()
	assertNoBareLineFeed(t, output)
	if !strings.Contains(output, "\033[31m") {
		t.Fatalf("error output should use red color, got %q", output)
	}
}

func TestRenderDiagnosisFullContent(t *testing.T) {
	var buffer bytes.Buffer
	renderDiagnosis(&buffer, `{"confidence":0.95,"root_cause":"disk full","investigation_commands":["df -h"],"fix_commands":["rm -rf /tmp/*"]}`, false)
	output := buffer.String()
	assertNoBareLineFeed(t, output)
	if !strings.Contains(output, "置信度: 0.95") {
		t.Fatalf("output = %q, want contains %q", output, "置信度: 0.95")
	}
	if !strings.Contains(output, "建议排查命令") {
		t.Fatalf("output = %q, want contains %q", output, "建议排查命令")
	}
	if !strings.Contains(output, "建议修复命令") {
		t.Fatalf("output = %q, want contains %q", output, "建议修复命令")
	}
}

func TestCleanupStaleSocketNotExist(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "nonexistent.sock")
	err := cleanupStaleSocket(socketPath)
	if err != nil {
		t.Fatalf("cleanupStaleSocket() error = %v, want nil", err)
	}
}

func TestHandleDiagSocketConnectionNil(t *testing.T) {
	handleDiagSocketConnection(context.Background(), nil, make(chan diagSignalRequest, 1))
}

func TestHandleDiagSocketConnectionEmitsTriggerAndAck(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	defer clientConnection.Close()

	triggerCh := make(chan diagSignalRequest, 1)
	done := make(chan struct{})
	go func() {
		handleDiagSocketConnection(context.Background(), serverConnection, triggerCh)
		close(done)
	}()

	if _, err := clientConnection.Write([]byte(diagSignalPayload)); err != nil {
		t.Fatalf("write payload error = %v", err)
	}

	var request diagSignalRequest
	select {
	case request = <-triggerCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for trigger request")
	}
	if request.done == nil {
		t.Fatal("request.done should not be nil")
	}

	if err := clientConnection.SetReadDeadline(time.Now().Add(120 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline error = %v", err)
	}
	ackBuffer := make([]byte, len(diagSignalAckPayload))
	if _, err := io.ReadFull(clientConnection, ackBuffer); err == nil {
		t.Fatalf("ack should not be readable before diagnosis completion, got %q", string(ackBuffer))
	}
	_ = clientConnection.SetReadDeadline(time.Time{})

	close(request.done)
	if _, err := io.ReadFull(clientConnection, ackBuffer); err != nil {
		t.Fatalf("read ack error = %v", err)
	}
	if string(ackBuffer) != diagSignalAckPayload {
		t.Fatalf("ack payload = %q, want %q", string(ackBuffer), diagSignalAckPayload)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handleDiagSocketConnection to return")
	}
}

func TestServeDiagSocketHandlesUnexpectedAndNormalPayload(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "serve-diag.sock")
	listener, resolvedPath, err := listenDiagSocket(socketPath)
	if err != nil {
		t.Fatalf("listenDiagSocket() error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(resolvedPath)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	triggerCh := make(chan diagSignalRequest, 2)
	serveDone := make(chan struct{})
	go func() {
		serveDiagSocket(ctx, listener, triggerCh, nil)
		close(serveDone)
	}()

	sendAndVerify := func(payload string) {
		t.Helper()
		connection, err := net.Dial("unix", resolvedPath)
		if err != nil {
			t.Fatalf("dial diag socket error = %v", err)
		}
		defer connection.Close()

		if _, err := connection.Write([]byte(payload)); err != nil {
			t.Fatalf("write payload error = %v", err)
		}

		var request diagSignalRequest
		select {
		case request = <-triggerCh:
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for trigger, payload=%q", payload)
		}
		if request.done == nil {
			t.Fatal("request.done should not be nil")
		}
		close(request.done)

		ack := make([]byte, len(diagSignalAckPayload))
		if _, err := io.ReadFull(connection, ack); err != nil {
			t.Fatalf("read ack error = %v", err)
		}
		if string(ack) != diagSignalAckPayload {
			t.Fatalf("ack payload = %q, want %q", string(ack), diagSignalAckPayload)
		}
	}

	sendAndVerify("unexpected-signal\n")
	sendAndVerify(diagSignalPayload)

	cancel()
	_ = listener.Close()
	select {
	case <-serveDone:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for serveDiagSocket to stop")
	}
}

func TestConsumeDiagSignalsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	consumeDiagSignals(ctx, nil, nil, nil, ManualShellOptions{}, "")
}

func TestConsumeDiagSignalsChannelClosed(t *testing.T) {
	ch := make(chan diagSignalRequest)
	close(ch)
	consumeDiagSignals(context.Background(), ch, nil, nil, ManualShellOptions{}, "")
}

func TestPrintProxyFunctionsNilWriter(t *testing.T) {
	printProxyInitializedBanner(nil)
	printProxyExitedBanner(nil)
}

func TestRunSingleDiagnosisNilOutput(t *testing.T) {
	runSingleDiagnosis(nil, nil, ManualShellOptions{}, "")
}
