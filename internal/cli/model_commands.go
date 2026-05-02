package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"neo-code/internal/config"
)

var (
	runModelLsCommand  = defaultModelLsCommandRunner
	runModelSetCommand = defaultModelSetCommandRunner
)

// newModelCommand 创建 model 命令组，管理当前 provider 下的模型选择。
func newModelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "model",
		Short:        "Manage model selection for the current provider",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newModelLsCommand(),
		newModelSetCommand(),
	)

	return cmd
}

// newModelLsCommand 创建 model ls 子命令，列出当前选中 provider 可用的模型。
func newModelLsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List available models for the current provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelLsCommand(cmd)
		},
	}
}

// newModelSetCommand 创建 model set 子命令，切换当前使用的模型。
func newModelSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <model-id>",
		Short: "Switch to a specific model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelSetCommand(cmd, args[0])
		},
	}
}

// defaultModelLsCommandRunner 列出当前选中 provider 的所有可用模型。
func defaultModelLsCommandRunner(cmd *cobra.Command) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	cfg, err := loader.Load(cmd.Context())
	if err != nil {
		return err
	}

	selectedName := strings.TrimSpace(cfg.SelectedProvider)
	if selectedName == "" {
		return fmt.Errorf("尚未选择任何供应商，请先运行 neocode use <provider>")
	}

	providerCfg, err := cfg.ProviderByName(selectedName)
	if err != nil {
		return fmt.Errorf("当前选中的供应商 %q 不存在: %w", selectedName, err)
	}

	currentModel := strings.TrimSpace(cfg.CurrentModel)
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "供应商: %s\n", providerCfg.Name)
	fmt.Fprintf(out, "当前模型: %s\n", displayCurrentModel(currentModel))
	fmt.Fprintln(out, "可用模型:")

	if len(providerCfg.Models) == 0 {
		fmt.Fprintln(out, "  (无静态模型列表，该供应商使用动态发现)")
		return nil
	}

	for _, model := range providerCfg.Models {
		marker := "  "
		if strings.EqualFold(strings.TrimSpace(model.ID), currentModel) {
			marker = "* "
		}
		line := fmt.Sprintf("%s%s", marker, strings.TrimSpace(model.ID))
		if name := strings.TrimSpace(model.Name); name != "" && name != model.ID {
			line += fmt.Sprintf(" (%s)", name)
		}
		fmt.Fprintln(out, line)
	}

	return nil
}

// defaultModelSetCommandRunner 切换当前模型，将 current_model 写入配置。
func defaultModelSetCommandRunner(cmd *cobra.Command, modelID string) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	manager := config.NewManager(loader)

	if _, err := manager.Load(cmd.Context()); err != nil {
		return err
	}

	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("模型 ID 不能为空")
	}

	err := manager.Update(cmd.Context(), func(cfg *config.Config) error {
		cfg.CurrentModel = modelID
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ 已切换模型: %s\n", modelID)
	return nil
}

// displayCurrentModel 格式化当前模型名称，未设置时显示占位文案。
func displayCurrentModel(model string) string {
	if model == "" {
		return "(未设置，将自动选择)"
	}
	return model
}
