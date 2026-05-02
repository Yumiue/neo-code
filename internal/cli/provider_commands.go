package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"neo-code/internal/config"
)

type providerAddOptions struct {
	Driver                string
	URL                   string
	APIKeyEnv             string
	DiscoveryEndpointPath string
}

var (
	runProviderAddCommand = defaultProviderAddCommandRunner
	runProviderLsCommand  = defaultProviderLsCommandRunner
	runProviderRmCommand  = defaultProviderRmCommandRunner
)

// newProviderCommand 创建 provider 命令组，管理自定义供应商配置。
func newProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "provider",
		Short:        "Manage custom providers",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newProviderAddCommand(),
		newProviderLsCommand(),
		newProviderRmCommand(),
	)

	return cmd
}

func newProviderAddCommand() *cobra.Command {
	var opts providerAddOptions
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderAddCommand(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Driver, "driver", "", "Provider driver (e.g., openaicompat)")
	cmd.Flags().StringVar(&opts.URL, "url", "", "Provider API base URL")
	cmd.Flags().StringVar(&opts.APIKeyEnv, "api-key-env", "", "Environment variable for API key")
	cmd.Flags().StringVar(&opts.DiscoveryEndpointPath, "discovery-endpoint", "", "Discovery endpoint path (optional)")

	_ = cmd.MarkFlagRequired("driver")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("api-key-env")

	return cmd
}

func defaultProviderAddCommandRunner(cmd *cobra.Command, name string, opts providerAddOptions) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	baseDir := loader.BaseDir()

	discoveryPath := opts.DiscoveryEndpointPath
	if discoveryPath == "" && opts.Driver == "openaicompat" {
		discoveryPath = "/v1/models"
	}

	input := config.SaveCustomProviderInput{
		Name:                  name,
		Driver:                opts.Driver,
		BaseURL:               opts.URL,
		APIKeyEnv:             opts.APIKeyEnv,
		ModelSource:           "discover",
		DiscoveryEndpointPath: discoveryPath,
	}

	if err := config.SaveCustomProviderWithModels(baseDir, input); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ 供应商 %s 添加成功！请记得在终端配置: export %s=\"sk-...\"\n", name, opts.APIKeyEnv)
	return nil
}



func newProviderLsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderLsCommand(cmd)
		},
	}
}

func defaultProviderLsCommandRunner(cmd *cobra.Command) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	cfg, err := loader.Load(cmd.Context())
	if err != nil {
		return err
	}

	for _, p := range cfg.Providers {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s (Driver: %s, Source: %s)\n", p.Name, p.Driver, p.Source)
	}
	return nil
}

func newProviderRmCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a custom provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderRmCommand(cmd, args[0])
		},
	}
}

func defaultProviderRmCommandRunner(cmd *cobra.Command, name string) error {
	var workdir string
	if f := cmd.Flag("workdir"); f != nil {
		workdir = f.Value.String()
	}
	loader := config.NewLoader(workdir, config.StaticDefaults())
	baseDir := loader.BaseDir()

	if err := config.DeleteCustomProvider(baseDir, name); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ 供应商 %s 已删除\n", name)
	return nil
}
