package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"neo-code/internal/config"
)

func TestModelCommand(t *testing.T) {
	cmd := newModelCommand()
	if cmd.Use != "model" {
		t.Errorf("expected use 'model', got %s", cmd.Use)
	}
}

func TestModelLsCommand(t *testing.T) {
	cmd := newModelLsCommand()
	if cmd.Use != "ls" {
		t.Errorf("expected use 'ls', got %s", cmd.Use)
	}

	called := false
	runModelLsCommand = func(c *cobra.Command) error {
		called = true
		return errors.New("mock error")
	}
	defer func() { runModelLsCommand = defaultModelLsCommandRunner }()

	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestModelSetCommand(t *testing.T) {
	cmd := newModelSetCommand()
	if cmd.Use != "set <model-id>" {
		t.Errorf("expected use 'set <model-id>', got %s", cmd.Use)
	}

	called := false
	runModelSetCommand = func(c *cobra.Command, modelID string) error {
		called = true
		if modelID != "gpt-5.4" {
			t.Errorf("expected 'gpt-5.4', got %s", modelID)
		}
		return errors.New("mock error")
	}
	defer func() { runModelSetCommand = defaultModelSetCommandRunner }()

	cmd.SetArgs([]string{"gpt-5.4"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestDefaultModelLsCommandRunner_BuiltinProvider(t *testing.T) {
	workdir := t.TempDir()

	// 加载默认配置并设置 selected_provider 为 builtin openai
	loader := config.NewLoader(workdir, config.StaticDefaults())
	manager := config.NewManager(loader)
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.SelectedProvider = "openai"
		cfg.CurrentModel = "gpt-5.4"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("workdir", workdir, "")
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	err := defaultModelLsCommandRunner(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "openai") {
		t.Errorf("expected output to contain 'openai', got %s", output)
	}
	if !strings.Contains(output, "gpt-5.4") {
		t.Errorf("expected output to contain 'gpt-5.4', got %s", output)
	}
	// 当前模型应有 * 标记
	if !strings.Contains(output, "* gpt-5.4") {
		t.Errorf("expected current model to be marked with *, got %s", output)
	}
}

func TestDefaultModelLsCommandRunner_NoSelectedProvider(t *testing.T) {
	workdir := t.TempDir()

	loader := config.NewLoader(workdir, config.StaticDefaults())
	manager := config.NewManager(loader)
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 清空 selected_provider
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.SelectedProvider = "openai"
		cfg.CurrentModel = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// 再次清空，写入一个空值是不行的（会被校验拒绝），所以测试 no-provider 场景用一个不同的方式
	// 实际上 selected_provider 被校验逻辑要求非空，所以我们测试正常的未设置 current_model 场景
	cmd := &cobra.Command{}
	cmd.Flags().String("workdir", workdir, "")
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	err := defaultModelLsCommandRunner(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "(未设置，将自动选择)") {
		t.Errorf("expected output to contain auto-select hint, got %s", output)
	}
}

func TestDefaultModelSetCommandRunner(t *testing.T) {
	workdir := t.TempDir()

	// 预创建配置
	loader := config.NewLoader(workdir, config.StaticDefaults())
	manager := config.NewManager(loader)
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Update(context.Background(), func(cfg *config.Config) error {
		cfg.SelectedProvider = "openai"
		cfg.CurrentModel = "gpt-5.4"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("workdir", workdir, "")
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	err := defaultModelSetCommandRunner(cmd, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证 current_model 已更新
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentModel != "gpt-4o" {
		t.Errorf("expected current model 'gpt-4o', got %s", cfg.CurrentModel)
	}

	// 空模型 ID 应报错
	err = defaultModelSetCommandRunner(cmd, "  ")
	if err == nil {
		t.Fatal("expected error for empty model ID, got nil")
	}
}

func TestDisplayCurrentModel(t *testing.T) {
	if got := displayCurrentModel(""); !strings.Contains(got, "未设置") {
		t.Errorf("expected empty model to show hint, got %s", got)
	}
	if got := displayCurrentModel("gpt-5.4"); got != "gpt-5.4" {
		t.Errorf("expected 'gpt-5.4', got %s", got)
	}
}
