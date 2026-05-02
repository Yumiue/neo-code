package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProviderCommand(t *testing.T) {
	cmd := newProviderCommand()
	if cmd.Use != "provider" {
		t.Errorf("expected use 'provider', got %s", cmd.Use)
	}
}

func TestProviderAddCommand(t *testing.T) {
	cmd := newProviderAddCommand()
	if cmd.Use != "add <name>" {
		t.Errorf("expected use 'add <name>', got %s", cmd.Use)
	}

	called := false
	runProviderAddCommand = func(c *cobra.Command, name string, opts providerAddOptions) error {
		called = true
		if name != "my-provider" {
			t.Errorf("expected 'my-provider', got %s", name)
		}
		if opts.Driver != "openaicompat" {
			t.Errorf("expected 'openaicompat', got %s", opts.Driver)
		}
		return errors.New("mock error")
	}
	defer func() { runProviderAddCommand = defaultProviderAddCommandRunner }()

	cmd.SetArgs([]string{"my-provider", "--driver", "openaicompat", "--url", "http://mock", "--api-key-env", "MOCK_KEY"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestProviderLsCommand(t *testing.T) {
	cmd := newProviderLsCommand()
	if cmd.Use != "ls" {
		t.Errorf("expected use 'ls', got %s", cmd.Use)
	}

	called := false
	runProviderLsCommand = func(c *cobra.Command) error {
		called = true
		return errors.New("mock error")
	}
	defer func() { runProviderLsCommand = defaultProviderLsCommandRunner }()

	cmd.SetArgs([]string{})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestProviderRmCommand(t *testing.T) {
	cmd := newProviderRmCommand()
	if cmd.Use != "rm <name>" {
		t.Errorf("expected use 'rm <name>', got %s", cmd.Use)
	}

	called := false
	runProviderRmCommand = func(c *cobra.Command, name string) error {
		called = true
		if name != "my-provider" {
			t.Errorf("expected 'my-provider', got %s", name)
		}
		return errors.New("mock error")
	}
	defer func() { runProviderRmCommand = defaultProviderRmCommandRunner }()

	cmd.SetArgs([]string{"my-provider"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !called {
		t.Fatal("runner not called")
	}
}

func TestDefaultProviderRunners(t *testing.T) {
	workdir := t.TempDir()

	cmd := &cobra.Command{}
	cmd.Flags().String("workdir", workdir, "")
	out := new(bytes.Buffer)
	cmd.SetOut(out)

	// Add
	opts := providerAddOptions{
		Driver:                "openaicompat",
		URL:                   "http://mock",
		APIKeyEnv:             "MOCK_KEY",
		DiscoveryEndpointPath: "/v1/models",
	}
	err := defaultProviderAddCommandRunner(cmd, "my-provider", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ls
	out.Reset()
	cmd.SetContext(context.Background())
	err = defaultProviderLsCommandRunner(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "my-provider") {
		t.Errorf("expected output to contain 'my-provider', got %s", out.String())
	}

	// Rm
	err = defaultProviderRmCommandRunner(cmd, "my-provider")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ls again
	out.Reset()
	err = defaultProviderLsCommandRunner(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "my-provider") {
		t.Errorf("expected output to not contain 'my-provider', got %s", out.String())
	}
}
