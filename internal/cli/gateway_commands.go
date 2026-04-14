package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"neo-code/internal/gateway"
)

const (
	defaultGatewayLogLevel = "info"
)

var (
	runGatewayCommand     = defaultGatewayCommandRunner
	runURLDispatchCommand = defaultURLDispatchCommandRunner
)

type gatewayCommandOptions struct {
	ListenAddress string
	LogLevel      string
}

type urlDispatchCommandOptions struct {
	URL           string
	ListenAddress string
}

// newGatewayCommand 创建并返回网关子命令，负责启动本地 Gateway 进程。
func newGatewayCommand() *cobra.Command {
	options := &gatewayCommandOptions{}

	cmd := &cobra.Command{
		Use:          "gateway",
		Short:        "Start local gateway server",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedLogLevel, err := normalizeGatewayLogLevel(options.LogLevel)
			if err != nil {
				return err
			}

			return runGatewayCommand(cmd.Context(), gatewayCommandOptions{
				ListenAddress: strings.TrimSpace(options.ListenAddress),
				LogLevel:      normalizedLogLevel,
			})
		},
	}

	cmd.Flags().StringVar(&options.ListenAddress, "listen", "", "gateway listen address (optional override)")
	cmd.Flags().StringVar(&options.LogLevel, "log-level", defaultGatewayLogLevel, "gateway log level: debug|info|warn|error")

	return cmd
}

// normalizeGatewayLogLevel 对网关日志级别做归一化并校验合法值。
func normalizeGatewayLogLevel(logLevel string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(logLevel))
	switch normalized {
	case "debug", "info", "warn", "error":
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid --log-level %q: must be debug|info|warn|error", logLevel)
	}
}

// defaultGatewayCommandRunner 使用网关服务骨架启动本地 IPC 监听并处理信号退出。
func defaultGatewayCommandRunner(ctx context.Context, options gatewayCommandOptions) error {
	logger := log.New(os.Stderr, "neocode-gateway: ", log.LstdFlags)
	logger.Printf("starting gateway (log-level=%s)", options.LogLevel)

	signalContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := gateway.NewServer(gateway.ServerOptions{
		ListenAddress: options.ListenAddress,
		Logger:        logger,
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = server.Close(context.Background())
	}()

	logger.Printf("gateway listen address: %s", server.ListenAddress())
	return server.Serve(signalContext, nil)
}

// newURLDispatchCommand 创建 URL Scheme 派发子命令骨架，仅做参数收敛与调用转发。
func newURLDispatchCommand() *cobra.Command {
	options := &urlDispatchCommandOptions{}

	cmd := &cobra.Command{
		Use:          "url-dispatch [url]",
		Short:        "Dispatch a neocode:// URL to gateway (skeleton)",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urlValue := strings.TrimSpace(options.URL)
			if urlValue == "" && len(args) == 1 {
				urlValue = strings.TrimSpace(args[0])
			}
			if urlValue == "" {
				return errors.New("missing required --url or positional <url>")
			}

			return runURLDispatchCommand(cmd.Context(), urlDispatchCommandOptions{
				URL:           urlValue,
				ListenAddress: strings.TrimSpace(options.ListenAddress),
			})
		},
	}

	cmd.Flags().StringVar(&options.URL, "url", "", "neocode:// URL to dispatch")
	cmd.Flags().StringVar(&options.ListenAddress, "listen", "", "gateway listen address override (reserved for EPIC-GW-02)")

	return cmd
}

// defaultURLDispatchCommandRunner 提供 url-dispatch 的默认骨架行为，明确告知后续步骤接管实现。
func defaultURLDispatchCommandRunner(_ context.Context, _ urlDispatchCommandOptions) error {
	return errors.New("url-dispatch is not implemented yet (planned in EPIC-GW-02)")
}
