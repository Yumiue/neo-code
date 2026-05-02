package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"neo-code/internal/config"
)

func TestUseCommand(t *testing.T) {
	cmd := newUseCommand()
	if cmd.Use != "use <provider>" {
		t.Errorf("expected use 'use <provider>', got %s", cmd.Use)
	}

	called := false
	runUseCommand = func(c *cobra.Command, name string, opts useCommandOptions) error {
		called = true
		if name != "my-provider" {
			t.Errorf("expected 'my-provider', got %s", name)
		}
		return errors.New("mock error")
	}
	defer func() { runUseCommand = defaultUseCommandRunner }()

	cmd.SetArgs([]string{"my-provider"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestUseCommandWithModelFlag(t *testing.T) {
	called := false
	runUseCommand = func(c *cobra.Command, name string, opts useCommandOptions) error {
		called = true
		if name != "my-provider" {
			t.Errorf("expected 'my-provider', got %s", name)
		}
		if opts.Model != "gpt-5.4" {
			t.Errorf("expected model 'gpt-5.4', got %s", opts.Model)
		}
		return nil
	}
	defer func() { runUseCommand = defaultUseCommandRunner }()

	cmd := newUseCommand()
	cmd.SetArgs([]string{"my-provider", "--model", "gpt-5.4"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestDefaultUseCommandRunner(t *testing.T) {
	workdir := t.TempDir()

	// 预先创建一个 custom provider 用于测试
	input := config.SaveCustomProviderInput{
		Name:                  "my-provider",
		Driver:                "openaicompat",
		BaseURL:               "http://mock",
		APIKeyEnv:             "MOCK_KEY",
		ModelSource:           "discover",
		DiscoveryEndpointPath: "/v1/models",
	}
	if err := config.SaveCustomProviderWithModels(workdir, input); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("workdir", workdir, "")
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	// 仅切换 provider
	err := defaultUseCommandRunner(cmd, "my-provider", useCommandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loader := config.NewLoader(workdir, config.StaticDefaults())
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SelectedProvider != "my-provider" {
		t.Errorf("expected selected provider 'my-provider', got %s", cfg.SelectedProvider)
	}

	// 切换 provider + model
	out.Reset()
	err = defaultUseCommandRunner(cmd, "my-provider", useCommandOptions{Model: "deepseek-chat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err = loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentModel != "deepseek-chat" {
		t.Errorf("expected current model 'deepseek-chat', got %s", cfg.CurrentModel)
	}

	// 不存在的 provider 应报错
	err = defaultUseCommandRunner(cmd, "not-found", useCommandOptions{})
	if err == nil {
		t.Fatal("expected error for non-existent provider, got nil")
	}
}
