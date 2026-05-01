package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"neo-code/internal/ptyproxy"
)

var (
	runShellCommand      = defaultShellCommandRunner
	runDiagCommand       = defaultDiagCommandRunner
	runManualShellProxy  = ptyproxy.RunManualShell
	sendDiagnoseSignalFn = ptyproxy.SendDiagnoseSignal
	readDiagEnvValue     = os.Getenv
)

type shellCommandOptions struct {
	Workdir              string
	Shell                string
	SocketPath           string
	GatewayListenAddress string
	GatewayTokenFile     string
}

type diagCommandOptions struct {
	SocketPath string
}

// newShellCommand 创建手动诊断入口：拉起 PTY 代理 shell 并在后台监听本地诊断信令。
func newShellCommand() *cobra.Command {
	options := &shellCommandOptions{}
	command := &cobra.Command{
		Use:          "shell",
		Short:        "Start manual terminal proxy shell for neocode diag",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Annotations: map[string]string{
			commandAnnotationSkipGlobalPreload:     "true",
			commandAnnotationSkipSilentUpdateCheck: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellCommand(
				cmd.Context(),
				shellCommandOptions{
					Workdir:              strings.TrimSpace(mustReadInheritedWorkdir(cmd)),
					Shell:                strings.TrimSpace(options.Shell),
					SocketPath:           strings.TrimSpace(options.SocketPath),
					GatewayListenAddress: strings.TrimSpace(options.GatewayListenAddress),
					GatewayTokenFile:     strings.TrimSpace(options.GatewayTokenFile),
				},
				cmd.InOrStdin(),
				cmd.OutOrStdout(),
				cmd.ErrOrStderr(),
			)
		},
	}

	command.Flags().StringVar(&options.Shell, "shell", "", "shell executable path (default $SHELL or /bin/bash)")
	command.Flags().StringVar(&options.SocketPath, "socket", "", "diagnose unix socket path override")
	command.Flags().StringVar(&options.GatewayListenAddress, "gateway-listen", "", "gateway listen address override")
	command.Flags().StringVar(&options.GatewayTokenFile, "gateway-token-file", "", "gateway token file override")
	return command
}

// newDiagCommand 创建手动触发入口：向代理 shell 发送一次诊断信号。
func newDiagCommand() *cobra.Command {
	options := &diagCommandOptions{}
	command := &cobra.Command{
		Use:          "diag",
		Short:        "Trigger terminal diagnose in current neocode shell",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Annotations: map[string]string{
			commandAnnotationSkipGlobalPreload:     "true",
			commandAnnotationSkipSilentUpdateCheck: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath, err := resolveDiagSocketPath(options.SocketPath)
			if err != nil {
				return err
			}
			return runDiagCommand(cmd.Context(), diagCommandOptions{
				SocketPath: socketPath,
			})
		},
	}

	command.Flags().StringVar(&options.SocketPath, "socket", "", "diagnose unix socket path override")
	return command
}

// defaultShellCommandRunner 组装 PTY 代理参数并启动手动诊断 shell。
func defaultShellCommandRunner(
	ctx context.Context,
	options shellCommandOptions,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return runManualShellProxy(ctx, ptyproxy.ManualShellOptions{
		Workdir:              strings.TrimSpace(options.Workdir),
		Shell:                strings.TrimSpace(options.Shell),
		SocketPath:           strings.TrimSpace(options.SocketPath),
		GatewayListenAddress: strings.TrimSpace(options.GatewayListenAddress),
		GatewayTokenFile:     strings.TrimSpace(options.GatewayTokenFile),
		Stdin:                stdin,
		Stdout:               stdout,
		Stderr:               stderr,
	})
}

// defaultDiagCommandRunner 向 shell 代理发送一次诊断触发信令。
func defaultDiagCommandRunner(ctx context.Context, options diagCommandOptions) error {
	return sendDiagnoseSignalFn(ctx, strings.TrimSpace(options.SocketPath))
}

// resolveDiagSocketPath 按“--socket > NEOCODE_DIAG_SOCKET”优先级解析诊断 socket 路径。
func resolveDiagSocketPath(socketFlag string) (string, error) {
	if socketPath := strings.TrimSpace(socketFlag); socketPath != "" {
		return socketPath, nil
	}
	if envValue := strings.TrimSpace(readDiagEnvValue(ptyproxy.DiagSocketEnv)); envValue != "" {
		return envValue, nil
	}
	return "", errors.New(
		fmt.Sprintf(
			"diagnose socket is empty: use --socket or set %s in your shell environment",
			ptyproxy.DiagSocketEnv,
		),
	)
}
