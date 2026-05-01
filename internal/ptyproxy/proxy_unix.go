//go:build !windows

package ptyproxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"neo-code/internal/gateway"
	gatewayclient "neo-code/internal/gateway/client"
	"neo-code/internal/gateway/protocol"
	"neo-code/internal/tools"
)

const (
	diagSignalPayload      = "diagnose\n"
	diagSignalAckPayload   = "1"
	diagnoseCallTimeout    = 10 * time.Second
	diagSocketReadDeadline = 2 * time.Second
	proxyInitializedBanner = "[ NeoCode Proxy initialized ]"
	proxyExitedBanner      = "[ NeoCode Proxy exited ]"
)

var (
	hostTerminalInput = os.Stdin
	isTerminalFD      = term.IsTerminal
	makeRawTerminal   = term.MakeRaw
	restoreTerminal   = term.Restore

	proxyOutputLineEndingNormalizer = strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\n", "\r\n",
	)
)

type diagnoseToolArgs struct {
	ErrorLog    string            `json:"error_log"`
	OSEnv       map[string]string `json:"os_env"`
	CommandText string            `json:"command_text"`
	ExitCode    int               `json:"exit_code"`
}

type diagnoseToolResult struct {
	Confidence            float64  `json:"confidence"`
	RootCause             string   `json:"root_cause"`
	FixCommands           []string `json:"fix_commands"`
	InvestigationCommands []string `json:"investigation_commands"`
}

type diagSignalRequest struct {
	done chan struct{}
}

// RunManualShell 启动 Manual 模式终端代理，提供 PTY 透明透传与诊断触发能力。
func RunManualShell(ctx context.Context, options ManualShellOptions) error {
	normalized, err := NormalizeShellOptions(options)
	if err != nil {
		return err
	}

	shellPath := resolveShellPath(normalized.Shell)
	if shellPath == "" {
		return errors.New("ptyproxy: shell executable is empty")
	}

	listener, socketPath, err := listenDiagSocket(normalized.SocketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	var outputMu sync.Mutex
	synchronizedOutput := &serializedWriter{writer: normalized.Stdout, lock: &outputMu}
	buffer := NewUTF8RingBuffer(DefaultRingBufferCapacity)
	outputSink := io.MultiWriter(synchronizedOutput, buffer)

	triggerCh := make(chan diagSignalRequest, 1)
	acceptCtx, cancelAccept := context.WithCancel(context.Background())
	var acceptWG sync.WaitGroup
	acceptWG.Add(1)
	go func() {
		defer acceptWG.Done()
		serveDiagSocket(acceptCtx, listener, triggerCh, normalized.Stderr)
	}()

	diagCtx, cancelDiag := context.WithCancel(context.Background())
	var diagWG sync.WaitGroup
	diagWG.Add(1)
	go func() {
		defer diagWG.Done()
		consumeDiagSignals(diagCtx, triggerCh, synchronizedOutput, buffer, normalized, socketPath)
	}()

	restoreRawTerminal, err := enableHostTerminalRawMode()
	if err != nil {
		cancelAccept()
		cancelDiag()
		acceptWG.Wait()
		diagWG.Wait()
		return err
	}
	defer func() {
		_ = restoreRawTerminal()
	}()

	command := buildShellCommand(shellPath, normalized, socketPath)

	ptyFile, err := pty.Start(command)
	if err != nil {
		cancelAccept()
		cancelDiag()
		acceptWG.Wait()
		diagWG.Wait()
		return fmt.Errorf("ptyproxy: start pty shell: %w", err)
	}
	defer func() {
		_ = ptyFile.Close()
	}()

	if err := pty.InheritSize(os.Stdin, ptyFile); err != nil && normalized.Stderr != nil {
		writeProxyf(normalized.Stderr, "neocode shell: inherit terminal size failed: %v\n", err)
	}
	stopResizeWatcher := watchPTYWindowResize(normalized.Stderr, ptyFile)
	defer stopResizeWatcher()

	printProxyInitializedBanner(synchronizedOutput)

	go func() {
		_, _ = io.Copy(ptyFile, normalized.Stdin)
	}()
	go func() {
		_, _ = io.Copy(outputSink, ptyFile)
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()

	var waitErr error
	select {
	case <-ctx.Done():
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		waitErr = <-waitDone
	case waitErr = <-waitDone:
	}

	printProxyExitedBanner(synchronizedOutput)

	_ = ptyFile.Close()
	cancelAccept()
	_ = listener.Close()
	acceptWG.Wait()
	cancelDiag()
	diagWG.Wait()

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return fmt.Errorf("ptyproxy: shell exited with code %d", status.ExitStatus())
			}
		}
		return waitErr
	}
	return nil
}

// printProxyInitializedBanner 在 PTY 会话启动前输出代理已就绪提示。
func printProxyInitializedBanner(writer io.Writer) {
	if writer == nil {
		return
	}
	writeProxyLine(writer, proxyInitializedBanner)
}

// printProxyExitedBanner 在 PTY 会话结束后输出代理退出提示。
func printProxyExitedBanner(writer io.Writer) {
	if writer == nil {
		return
	}
	_, _ = fmt.Fprint(writer, "\r\n[ NeoCode Proxy exited ]\r\n")
}

// enableHostTerminalRawMode 将宿主终端切换到原始模式，并返回可恢复终端状态的函数。
func enableHostTerminalRawMode() (func() error, error) {
	if hostTerminalInput == nil {
		return func() error { return nil }, nil
	}

	fd := int(hostTerminalInput.Fd())
	if !isTerminalFD(fd) {
		return func() error { return nil }, nil
	}

	originalState, err := makeRawTerminal(fd)
	if err != nil {
		return nil, fmt.Errorf("ptyproxy: set host terminal raw mode: %w", err)
	}
	return func() error {
		if restoreErr := restoreTerminal(fd, originalState); restoreErr != nil {
			return fmt.Errorf("ptyproxy: restore host terminal state: %w", restoreErr)
		}
		return nil
	}, nil
}

// syncPTYWindowSize 将宿主终端窗口尺寸同步到 PTY，避免默认 80 列导致的提示符错位。
func syncPTYWindowSize(errWriter io.Writer, ptyFile *os.File) {
	if ptyFile == nil {
		return
	}
	if err := pty.InheritSize(os.Stdin, ptyFile); err != nil && errWriter != nil {
		writeProxyf(errWriter, "neocode shell: inherit terminal size failed: %v\n", err)
	}
}

// watchPTYWindowResize 监听 SIGWINCH 并持续同步 PTY 尺寸，返回停止监听函数。
func watchPTYWindowResize(errWriter io.Writer, ptyFile *os.File) func() {
	if ptyFile == nil {
		return func() {}
	}

	winchSignals := make(chan os.Signal, 1)
	signal.Notify(winchSignals, syscall.SIGWINCH)

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	var watcherWG sync.WaitGroup
	watcherWG.Add(1)
	go func() {
		defer watcherWG.Done()
		for {
			select {
			case <-stopCh:
				return
			case _, ok := <-winchSignals:
				if !ok {
					return
				}
				syncPTYWindowSize(errWriter, ptyFile)
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			signal.Stop(winchSignals)
			close(stopCh)
			watcherWG.Wait()
		})
	}
}

// writeProxyText 统一将代理自输出的换行规范化为 CRLF，适配 Raw Mode 下的终端显示。
func writeProxyText(writer io.Writer, text string) {
	if writer == nil || text == "" {
		return
	}
	_, _ = io.WriteString(writer, proxyOutputLineEndingNormalizer.Replace(text))
}

// writeProxyLine 向代理输出写入单行文本，并追加 CRLF 换行。
func writeProxyLine(writer io.Writer, text string) {
	writeProxyText(writer, text+"\n")
}

// writeProxyf 使用格式化字符串输出代理信息，并统一换行为 CRLF。
func writeProxyf(writer io.Writer, format string, args ...any) {
	writeProxyText(writer, fmt.Sprintf(format, args...))
}

// SendDiagnoseSignal 连接 shell 代理暴露的本地 socket，并发送一次诊断触发信令。
func SendDiagnoseSignal(ctx context.Context, socketPath string) error {
	normalizedSocket := strings.TrimSpace(socketPath)
	if normalizedSocket == "" {
		return errors.New("ptyproxy: diagnose socket path is empty")
	}

	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", normalizedSocket)
	if err != nil {
		return fmt.Errorf("ptyproxy: connect diagnose socket: %w", err)
	}
	defer connection.Close()

	_ = connection.SetWriteDeadline(time.Now().Add(diagSocketReadDeadline))
	_, err = connection.Write([]byte(diagSignalPayload))
	if err != nil {
		return fmt.Errorf("ptyproxy: send diagnose signal: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		_, readErr := io.ReadAll(connection)
		waitDone <- readErr
	}()

	select {
	case <-ctx.Done():
		_ = connection.SetReadDeadline(time.Now())
		<-waitDone
		return fmt.Errorf("ptyproxy: wait diagnose completion: %w", ctx.Err())
	case readErr := <-waitDone:
		if readErr != nil && !isClosedNetworkError(readErr) {
			return fmt.Errorf("ptyproxy: wait diagnose completion: %w", readErr)
		}
	}
	return nil
}

// buildShellCommand 构建真实 shell 进程，并在子进程环境中注入诊断 socket 变量。
func buildShellCommand(shellPath string, options ManualShellOptions, socketPath string) *exec.Cmd {
	command := exec.Command(shellPath)
	command.Dir = options.Workdir
	command.Env = MergeEnvVar(os.Environ(), DiagSocketEnv, socketPath)
	return command
}

// resolveShellPath 按“显式参数 -> SHELL 环境变量 -> /bin/bash”顺序选择实际 shell。
func resolveShellPath(shellOption string) string {
	if trimmed := strings.TrimSpace(shellOption); trimmed != "" {
		return trimmed
	}
	if envShell := strings.TrimSpace(os.Getenv("SHELL")); envShell != "" {
		return envShell
	}
	return "/bin/bash"
}

// listenDiagSocket 创建并监听 Unix socket；路径为空时按进程生成默认地址。
func listenDiagSocket(socketOption string) (net.Listener, string, error) {
	socketPath := strings.TrimSpace(socketOption)
	if socketPath == "" {
		socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("neocode-diag-%d.sock", os.Getpid()))
	}
	socketPath = filepath.Clean(socketPath)
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil, "", fmt.Errorf("ptyproxy: create socket directory: %w", err)
	}
	if err := cleanupStaleSocket(socketPath); err != nil {
		return nil, "", err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, "", fmt.Errorf("ptyproxy: listen diagnose socket: %w", err)
	}
	if chmodErr := os.Chmod(socketPath, 0o600); chmodErr != nil {
		_ = listener.Close()
		return nil, "", fmt.Errorf("ptyproxy: set diagnose socket permission: %w", chmodErr)
	}
	return listener, socketPath, nil
}

// cleanupStaleSocket 删除历史遗留 socket，防止异常退出后下次监听失败。
func cleanupStaleSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ptyproxy: stat diagnose socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("ptyproxy: diagnose socket path exists and is not socket: %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("ptyproxy: remove stale diagnose socket: %w", err)
	}
	return nil
}

// serveDiagSocket 接收本地诊断触发连接，并向消费协程投递触发信号。
func serveDiagSocket(ctx context.Context, listener net.Listener, triggerCh chan<- diagSignalRequest, errWriter io.Writer) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || isClosedNetworkError(err) {
				return
			}
			if errWriter != nil {
				writeProxyf(errWriter, "neocode diag: accept signal error: %v\n", err)
			}
			continue
		}
		handleDiagSocketConnection(ctx, connection, triggerCh)
	}
}

// handleDiagSocketConnection 处理单个诊断请求连接，并在对应诊断渲染结束后返回 ACK 后断开连接。
func handleDiagSocketConnection(ctx context.Context, connection net.Conn, triggerCh chan<- diagSignalRequest) {
	if connection == nil {
		return
	}
	defer connection.Close()

	_ = connection.SetReadDeadline(time.Now().Add(diagSocketReadDeadline))
	reader := bufio.NewReader(io.LimitReader(connection, 1024))
	_, _ = reader.ReadString('\n')
	_ = connection.SetReadDeadline(time.Time{})

	request := diagSignalRequest{
		done: make(chan struct{}),
	}
	select {
	case <-ctx.Done():
		return
	case triggerCh <- request:
	}

	select {
	case <-ctx.Done():
		return
	case <-request.done:
		_, _ = connection.Write([]byte(diagSignalAckPayload))
	}
}

// consumeDiagSignals 串行消费触发信号并执行诊断调用，避免并发请求互相覆盖终端输出。
func consumeDiagSignals(
	ctx context.Context,
	triggerCh <-chan diagSignalRequest,
	output io.Writer,
	buffer *UTF8RingBuffer,
	options ManualShellOptions,
	socketPath string,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-triggerCh:
			if !ok {
				return
			}
			runSingleDiagnosis(output, buffer, options, socketPath)
			if request.done != nil {
				close(request.done)
			}
		}
	}
}

// runSingleDiagnosis 读取最近日志并调用 gateway.executeSystemTool(diagnose) 获取诊断结果。
func runSingleDiagnosis(output io.Writer, buffer *UTF8RingBuffer, options ManualShellOptions, socketPath string) {
	if output == nil {
		return
	}

	logSnapshot := buffer.SnapshotString()
	requestArgs := diagnoseToolArgs{
		ErrorLog: logSnapshot,
		OSEnv: map[string]string{
			"os":     runtime.GOOS,
			"shell":  resolveShellPath(options.Shell),
			"cwd":    options.Workdir,
			"socket": socketPath,
		},
		CommandText: "",
		ExitCode:    0,
	}

	requestPayload, err := json.Marshal(requestArgs)
	if err != nil {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 请求构建失败: %v\n", err)
		return
	}

	rpcClient, err := gatewayclient.NewGatewayRPCClient(gatewayclient.GatewayRPCClientOptions{
		ListenAddress: options.GatewayListenAddress,
		TokenFile:     options.GatewayTokenFile,
	})
	if err != nil {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 网关客户端初始化失败: %v\n", err)
		return
	}
	defer rpcClient.Close()

	callContext, cancel := context.WithTimeout(context.Background(), diagnoseCallTimeout)
	defer cancel()

	if err := rpcClient.Authenticate(callContext); err != nil {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 网关认证失败: %v\n", err)
		return
	}

	var frame gateway.MessageFrame
	if err := rpcClient.CallWithOptions(
		callContext,
		protocol.MethodGatewayExecuteSystemTool,
		protocol.ExecuteSystemToolParams{
			Workdir:   options.Workdir,
			ToolName:  tools.ToolNameDiagnose,
			Arguments: requestPayload,
		},
		&frame,
		gatewayclient.GatewayRPCCallOptions{
			Timeout: diagnoseCallTimeout,
			Retries: 0,
		},
	); err != nil {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 诊断调用失败: %v\n", err)
		return
	}

	if frame.Type == gateway.FrameTypeError && frame.Error != nil {
		writeProxyf(
			output,
			"\n\033[31m[NeoCode Diagnosis]\033[0m 网关返回错误 (%s): %s\n",
			strings.TrimSpace(frame.Error.Code),
			strings.TrimSpace(frame.Error.Message),
		)
		return
	}
	if frame.Type != gateway.FrameTypeAck {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 网关返回未知帧: %s\n", frame.Type)
		return
	}

	toolResult, err := decodeToolResult(frame.Payload)
	if err != nil {
		writeProxyf(output, "\n\033[31m[NeoCode Diagnosis]\033[0m 诊断结果解析失败: %v\n", err)
		return
	}
	renderDiagnosis(output, toolResult.Content, toolResult.IsError)
}

// decodeToolResult 将网关 payload 反序列化为统一的 ToolResult 结构。
func decodeToolResult(payload any) (tools.ToolResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("encode tool payload: %w", err)
	}
	var result tools.ToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return tools.ToolResult{}, fmt.Errorf("decode tool payload: %w", err)
	}
	return result, nil
}

// renderDiagnosis 将工具返回渲染成终端可读格式，并在失败时降级展示原始文本。
func renderDiagnosis(output io.Writer, content string, isError bool) {
	headerColor := "\033[36m"
	if isError {
		headerColor = "\033[31m"
	}
	writeProxyf(output, "\n%s[NeoCode Diagnosis]\033[0m\n", headerColor)

	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		writeProxyLine(output, "- 无可用诊断内容")
		return
	}

	var parsed diagnoseToolResult
	if err := json.Unmarshal([]byte(trimmedContent), &parsed); err != nil || strings.TrimSpace(parsed.RootCause) == "" {
		writeProxyLine(output, trimmedContent)
		return
	}

	writeProxyf(output, "置信度: %.2f\n", parsed.Confidence)
	writeProxyf(output, "根因: %s\n", strings.TrimSpace(parsed.RootCause))
	if len(parsed.InvestigationCommands) > 0 {
		writeProxyLine(output, "建议排查命令:")
		for _, command := range parsed.InvestigationCommands {
			writeProxyf(output, "- %s\n", strings.TrimSpace(command))
		}
	}
	if len(parsed.FixCommands) > 0 {
		writeProxyLine(output, "建议修复命令:")
		for _, command := range parsed.FixCommands {
			writeProxyf(output, "- %s\n", strings.TrimSpace(command))
		}
	}
}

// isClosedNetworkError 识别网络连接已关闭类错误，避免重复打印无效噪声。
func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "use of closed network connection")
}

// serializedWriter 在并发写入场景下串行化输出，避免诊断内容与 shell 输出交错。
type serializedWriter struct {
	writer io.Writer
	lock   *sync.Mutex
}

// Write 实现 io.Writer 并在写入前加锁。
func (w *serializedWriter) Write(payload []byte) (int, error) {
	if w == nil || w.writer == nil {
		return len(payload), nil
	}
	if w.lock == nil {
		return w.writer.Write(payload)
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.writer.Write(payload)
}
