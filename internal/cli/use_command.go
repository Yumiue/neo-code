package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"neo-code/internal/config"
)

var runUseCommand = defaultUseCommandRunner

// newUseCommand 创建 use 命令，用于全局切换选中的 provider。
func newUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use <provider>",
		Short: "Switch to a specific provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUseCommand(cmd, args[0])
		},
	}
}

// defaultUseCommandRunner 执行 provider 切换逻辑。
func defaultUseCommandRunner(cmd *cobra.Command, name string) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	manager := config.NewManager(loader)

	// 预加载校验配置是否存在
	if _, err := manager.Load(cmd.Context()); err != nil {
		return err
	}

	err := manager.Update(cmd.Context(), func(cfg *config.Config) error {
		provider, err := cfg.ProviderByName(name)
		if err != nil {
			return err
		}
		cfg.SelectedProvider = provider.Name
		return nil
	})

	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ 已全局切换到供应商: %s\n", name)
	return nil
}
