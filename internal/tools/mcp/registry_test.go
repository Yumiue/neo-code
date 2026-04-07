package mcp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type stubServerClient struct {
	mu            sync.Mutex
	tools         []ToolDescriptor
	callResult    CallResult
	listErr       error
	callErr       error
	healthErr     error
	lastToolName  string
	lastArguments []byte
}

func (s *stubServerClient) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return cloneToolDescriptors(s.tools), nil
}

func (s *stubServerClient) CallTool(ctx context.Context, toolName string, arguments []byte) (CallResult, error) {
	s.mu.Lock()
	s.lastToolName = toolName
	s.lastArguments = append([]byte(nil), arguments...)
	s.mu.Unlock()
	if s.callErr != nil {
		return CallResult{}, s.callErr
	}
	result := s.callResult
	result.Metadata = cloneSchema(result.Metadata)
	return result, nil
}

func (s *stubServerClient) HealthCheck(ctx context.Context) error {
	return s.healthErr
}

func TestRegistryRegisterRefreshSnapshotCall(t *testing.T) {
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
		callResult: CallResult{
			Content: "ok",
			Metadata: map[string]any{
				"latency_ms": 18,
			},
		},
	}

	if err := registry.RegisterServer("Docs", "stdio", "v1", client); err != nil {
		t.Fatalf("RegisterServer() error = %v", err)
	}
	if err := registry.RefreshServerTools(context.Background(), "docs"); err != nil {
		t.Fatalf("RefreshServerTools() error = %v", err)
	}

	snapshots := registry.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	snapshot := snapshots[0]
	if snapshot.ServerID != "docs" || snapshot.Status != ServerStatusReady {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(snapshot.Tools) != 1 || snapshot.Tools[0].Name != "search" {
		t.Fatalf("unexpected tools in snapshot: %+v", snapshot.Tools)
	}

	result, err := registry.Call(context.Background(), "docs", "search", []byte(`{"q":"mcp"}`))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("expected call content ok, got %q", result.Content)
	}
}

func TestRegistryStatusTransitions(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	client := &stubServerClient{
		tools: []ToolDescriptor{
			{Name: "search", InputSchema: map[string]any{"type": "object"}},
		},
	}
	if err := registry.RegisterServer("server-1", "stdio", "v1", client); err != nil {
		t.Fatalf("register server: %v", err)
	}

	client.healthErr = errors.New("offline")
	if err := registry.HealthCheck(context.Background(), "server-1"); err == nil {
		t.Fatalf("expected health check failure")
	}
	if snapshots := registry.Snapshot(); snapshots[0].Status != ServerStatusOffline {
		t.Fatalf("expected offline status, got %+v", snapshots[0].Status)
	}

	client.healthErr = nil
	if err := registry.HealthCheck(context.Background(), "server-1"); err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}
	if snapshots := registry.Snapshot(); snapshots[0].Status != ServerStatusReady {
		t.Fatalf("expected ready status, got %+v", snapshots[0].Status)
	}
}

func TestRegistryConcurrentSnapshotAndRefresh(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	client := &stubServerClient{
		tools: []ToolDescriptor{
			{Name: "search", InputSchema: map[string]any{"type": "object"}},
		},
	}
	if err := registry.RegisterServer("server-1", "stdio", "v1", client); err != nil {
		t.Fatalf("register server: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.RefreshServerTools(context.Background(), "server-1")
		}()
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.Snapshot()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("concurrent registry operations timed out")
	}
}
