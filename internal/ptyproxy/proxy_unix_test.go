//go:build !windows

package ptyproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func assertNoBareLineFeed(t *testing.T, text string) {
	t.Helper()
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' && (i == 0 || text[i-1] != '\r') {
			t.Fatalf("output contains bare LF at index %d: %q", i, text)
		}
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

func TestHandleDiagSocketConnectionAutoMode(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantEnabled bool
	}{
		{name: "auto on", command: diagCommandAutoOn, wantEnabled: true},
		{name: "auto off", command: diagCommandAutoOff, wantEnabled: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()

			jobCh := make(chan diagnoseJob, 1)
			autoState := &autoRuntimeState{}
			autoState.Enabled.Store(!tc.wantEnabled)
			done := make(chan struct{})
			go func() {
				handleDiagSocketConnection(context.Background(), serverConn, jobCh, autoState)
				close(done)
			}()

			req := diagIPCRequest{Cmd: tc.command}
			raw, _ := json.Marshal(req)
			_, _ = clientConn.Write(append(raw, '\n'))

			line, err := bufio.NewReader(clientConn).ReadBytes('\n')
			if err != nil {
				t.Fatalf("read response error = %v", err)
			}
			var resp diagIPCResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				t.Fatalf("unmarshal response error = %v", err)
			}
			if !resp.OK {
				t.Fatalf("response not ok: %#v", resp)
			}
			if autoState.Enabled.Load() != tc.wantEnabled {
				t.Fatalf("enabled = %v, want %v", autoState.Enabled.Load(), tc.wantEnabled)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("timeout waiting handler return")
			}
		})
	}
}

func TestHandleDiagSocketConnectionDiagnose(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	jobCh := make(chan diagnoseJob, 1)
	autoState := &autoRuntimeState{}
	done := make(chan struct{})
	go func() {
		handleDiagSocketConnection(context.Background(), serverConn, jobCh, autoState)
		close(done)
	}()

	raw, _ := json.Marshal(diagIPCRequest{Cmd: diagCommandDiagnose})
	_, _ = clientConn.Write(append(raw, '\n'))

	var job diagnoseJob
	select {
	case job = <-jobCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting diagnose job")
	}
	if job.Done == nil {
		t.Fatal("job.Done should not be nil")
	}
	job.Done <- diagIPCResponse{OK: true, Message: "done"}

	line, err := bufio.NewReader(clientConn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("read response error = %v", err)
	}
	var resp diagIPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("unmarshal response error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("response not ok: %#v", resp)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting handler return")
	}
}

func TestSendDiagIPCCommandToPath(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "ipc.sock")
	listener, resolvedPath, err := listenDiagSocket(socketPath)
	if err != nil {
		t.Fatalf("listenDiagSocket() error = %v", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(resolvedPath)
	}()

	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer conn.Close()
		line, readErr := bufio.NewReader(conn).ReadBytes('\n')
		if readErr != nil {
			serverDone <- readErr
			return
		}
		var req diagIPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			serverDone <- err
			return
		}
		if req.Cmd != diagCommandDiagnose {
			serverDone <- io.ErrUnexpectedEOF
			return
		}
		resp, _ := json.Marshal(diagIPCResponse{OK: true, Message: "ok"})
		_, writeErr := conn.Write(append(resp, '\n'))
		serverDone <- writeErr
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := sendDiagIPCCommandToPath(ctx, resolvedPath, diagIPCRequest{Cmd: diagCommandDiagnose})
	if err != nil {
		t.Fatalf("sendDiagIPCCommandToPath() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("resp.OK = false")
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatalf("server error = %v", serverErr)
	}
}

func TestRunSingleDiagnosisGatewayUnavailableDoesNotPanic(t *testing.T) {
	buffer := NewUTF8RingBuffer(1024)
	_, _ = buffer.Write([]byte("中文日志 + \u001b[31merror\u001b[0m"))

	output := &bytes.Buffer{}
	runSingleDiagnosis(
		output,
		buffer,
		ManualShellOptions{
			Workdir:              t.TempDir(),
			Shell:                "/bin/bash",
			GatewayListenAddress: filepath.Join(t.TempDir(), "missing-gateway.sock"),
			GatewayTokenFile:     filepath.Join(t.TempDir(), "missing-auth.json"),
		},
		filepath.Join(t.TempDir(), "diag.sock"),
		diagnoseTrigger{
			CommandText: "go test ./...",
			ExitCode:    1,
			OutputText:  "fatal: missing module",
		},
		false,
		&autoRuntimeState{},
	)

	if !strings.Contains(output.String(), "NeoCode Diagnosis") {
		t.Fatalf("output = %q, want contains %q", output.String(), "NeoCode Diagnosis")
	}
	assertNoBareLineFeed(t, output.String())
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
