package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"neo-code/internal/config"
)

var runUseCommand = defaultUseCommandRunner

type useCommandOptions struct {
	Model string
}

// newUseCommand 创建 use 命令，用于全局切换选中的 provider，并可选指定模型。
func newUseCommand() *cobra.Command {
	var opts useCommandOptions
	cmd := &cobra.Command{
		Use:   "use <provider>",
		Short: "Switch to a specific provider (and optionally a model)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUseCommand(cmd, args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.Model, "model", "", "model to select for the provider")
	return cmd
}

// defaultUseCommandRunner 执行 provider 切换逻辑，并在指定 --model 时同步设置 current_model。
func defaultUseCommandRunner(cmd *cobra.Command, name string, opts useCommandOptions) error {
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

	model := strings.TrimSpace(opts.Model)

	err := manager.Update(cmd.Context(), func(cfg *config.Config) error {
		provider, err := cfg.ProviderByName(name)
		if err != nil {
			return err
		}
		cfg.SelectedProvider = provider.Name
		if model != "" {
			cfg.CurrentModel = model
		}
		return nil
	})

	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "✅ 已全局切换到供应商: %s\n", name)
	if model != "" {
		fmt.Fprintf(out, "✅ 已设置模型: %s\n", model)
	}
	return nil
}
