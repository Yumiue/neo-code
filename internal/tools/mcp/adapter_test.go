package mcp

import (
	"context"
	"errors"
	"testing"

	"neo-code/internal/tools"
)

func TestAdapterFactoryBuildTools(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	client := &stubServerClient{
		tools: []ToolDescriptor{
			{
				Name:        "search",
				Description: "search docs",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	if err := registry.RegisterServer("docs", "stdio", "v1", client); err != nil {
		t.Fatalf("register server: %v", err)
	}
	if err := registry.RefreshServerTools(context.Background(), "docs"); err != nil {
		t.Fatalf("refresh tools: %v", err)
	}

	factory := NewAdapterFactory(registry)
	toolsList, err := factory.BuildTools(context.Background())
	if err != nil {
		t.Fatalf("BuildTools() error = %v", err)
	}
	if len(toolsList) != 1 {
		t.Fatalf("expected one adapter tool, got %d", len(toolsList))
	}
	if toolsList[0].Name() != "mcp.docs.search" {
		t.Fatalf("unexpected adapter tool name: %q", toolsList[0].Name())
	}
}

func TestAdapterExecute(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	client := &stubServerClient{
		tools: []ToolDescriptor{
			{Name: "search", InputSchema: map[string]any{"type": "object"}},
		},
		callResult: CallResult{
			Content: "result body",
			Metadata: map[string]any{
				"latency_ms": 20,
			},
		},
	}
	if err := registry.RegisterServer("docs", "stdio", "v1", client); err != nil {
		t.Fatalf("register server: %v", err)
	}
	if err := registry.RefreshServerTools(context.Background(), "docs"); err != nil {
		t.Fatalf("refresh tools: %v", err)
	}

	adapter, err := NewAdapter(registry, "docs", ToolDescriptor{
		Name:        "search",
		Description: "search docs",
		InputSchema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	result, err := adapter.Execute(context.Background(), tools.ToolCallInput{
		ID:        "tool-call-1",
		Name:      adapter.Name(),
		Arguments: []byte(`{"q":"mcp"}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ToolCallID != "tool-call-1" {
		t.Fatalf("expected tool call id tool-call-1, got %q", result.ToolCallID)
	}
	if result.Name != "mcp.docs.search" {
		t.Fatalf("expected tool name mcp.docs.search, got %q", result.Name)
	}
	if result.Content != "result body" {
		t.Fatalf("expected result content, got %q", result.Content)
	}
	if result.Metadata["mcp_server_id"] != "docs" || result.Metadata["mcp_tool_name"] != "search" {
		t.Fatalf("unexpected metadata: %+v", result.Metadata)
	}
}

func TestAdapterExecuteErrorMapping(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	client := &stubServerClient{
		tools: []ToolDescriptor{
			{Name: "search", InputSchema: map[string]any{"type": "object"}},
		},
		callErr: errors.New("transport timeout"),
	}
	if err := registry.RegisterServer("docs", "stdio", "v1", client); err != nil {
		t.Fatalf("register server: %v", err)
	}
	if err := registry.RefreshServerTools(context.Background(), "docs"); err != nil {
		t.Fatalf("refresh tools: %v", err)
	}

	adapter, err := NewAdapter(registry, "docs", ToolDescriptor{
		Name: "search",
	})
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	result, execErr := adapter.Execute(context.Background(), tools.ToolCallInput{
		ID:        "tool-call-error",
		Name:      adapter.Name(),
		Arguments: []byte(`{"q":"mcp"}`),
	})
	if execErr == nil {
		t.Fatalf("expected execute error")
	}
	if !result.IsError {
		t.Fatalf("expected error result, got %+v", result)
	}
	if result.Metadata["mcp_server_id"] != "docs" {
		t.Fatalf("unexpected metadata for error result: %+v", result.Metadata)
	}
}
