package runtime

import (
	"context"
	"testing"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/controlplane"
	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
)

func TestCollectCompletionStateDoesNotAutoVerifyWithoutClosureResponse(t *testing.T) {
	t.Parallel()

	state := newRunState("run-verify-silent", newRuntimeSession("session-verify-silent"))
	state.completion = controlplane.CompletionState{
		RequiresVerification: true,
		HasUnverifiedWrites:  true,
	}

	got := collectCompletionState(&state, providertypes.Message{Role: providertypes.RoleAssistant}, false)
	if got.HasUnverifiedWrites != true {
		t.Fatalf("expected unverified writes to remain blocked, got %+v", got)
	}
	if got.LastTurnVerifyPassed {
		t.Fatalf("expected silent turn to not count as verify passed")
	}
}

func TestCollectCompletionStateKeepsExplicitVerifyPassedState(t *testing.T) {
	t.Parallel()

	state := newRunState("run-verify-closure", newRuntimeSession("session-verify-closure"))
	state.completion = controlplane.CompletionState{
		RequiresVerification: true,
		LastTurnVerifyPassed: true,
	}

	got := collectCompletionState(&state, providertypes.Message{
		Role:  providertypes.RoleAssistant,
		Parts: []providertypes.ContentPart{providertypes.NewTextPart("done")},
	}, false)
	if got.HasUnverifiedWrites {
		t.Fatalf("expected explicit verify state to remain cleared, got %+v", got)
	}
	if !got.LastTurnVerifyPassed {
		t.Fatalf("expected explicit verify passed state to be preserved")
	}
}

func TestApplyToolExecutionCompletionTracksWriteAndVerification(t *testing.T) {
	t.Parallel()

	written := applyToolExecutionCompletion(controlplane.CompletionState{}, toolExecutionSummary{
		HasSuccessfulWorkspaceWrite: true,
	})
	if !written.RequiresVerification || !written.HasUnverifiedWrites {
		t.Fatalf("expected successful write to require verification, got %+v", written)
	}
	if written.LastTurnVerifyPassed {
		t.Fatalf("expected write-only turn to keep verify pending")
	}

	verified := applyToolExecutionCompletion(written, toolExecutionSummary{
		HasSuccessfulVerification: true,
	})
	if verified.HasUnverifiedWrites {
		t.Fatalf("expected explicit verification to clear pending write, got %+v", verified)
	}
	if !verified.LastTurnVerifyPassed {
		t.Fatalf("expected explicit verification to mark verify passed")
	}
}

func TestHasPendingAgentTodosBlocksOnAnyNonTerminalTodo(t *testing.T) {
	t.Parallel()

	todos := []agentsession.TodoItem{
		{
			ID:       "subagent-1",
			Content:  "delegate",
			Status:   agentsession.TodoStatusPending,
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}
	if !hasPendingAgentTodos(todos) {
		t.Fatalf("expected pending subagent todo to block completion")
	}

	completed := []agentsession.TodoItem{
		{
			ID:       "subagent-2",
			Content:  "done",
			Status:   agentsession.TodoStatusCompleted,
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}
	if hasPendingAgentTodos(completed) {
		t.Fatalf("expected terminal todo to not block completion")
	}
}

func TestTransitionRunPhaseInvalidTransitionReturnsError(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 4)}
	state := newRunState("run-invalid-phase", newRuntimeSession("session-invalid-phase"))
	state.lifecycle = controlplane.RunStatePlan

	err := service.transitionRunState(context.Background(), &state, controlplane.RunStateVerify)
	if err == nil {
		t.Fatalf("expected invalid transition to return error")
	}
	if state.lifecycle != controlplane.RunStatePlan {
		t.Fatalf("expected lifecycle to remain unchanged, got %q", state.lifecycle)
	}
	if events := collectRuntimeEvents(service.Events()); len(events) != 0 {
		t.Fatalf("expected no phase events on invalid transition, got %+v", events)
	}
}

func TestHasSuccessfulVerificationResultRequiresExplicitVerificationCall(t *testing.T) {
	t.Parallel()

	bashVerifyCall := providertypes.ToolCall{
		ID:        "verify-1",
		Name:      tools.ToolNameBash,
		Arguments: `{"command":"go test ./..."}`,
	}
	successfulResults := []tools.ToolResult{
		{ToolCallID: "verify-1", Name: tools.ToolNameBash, Content: "ok"},
	}
	if !hasSuccessfulVerificationResult([]providertypes.ToolCall{bashVerifyCall}, successfulResults) {
		t.Fatalf("expected explicit verification bash command to count as verify passed")
	}

	readCall := providertypes.ToolCall{
		ID:        "read-1",
		Name:      tools.ToolNameFilesystemReadFile,
		Arguments: `{"path":"README.md"}`,
	}
	readResults := []tools.ToolResult{
		{ToolCallID: "read-1", Name: tools.ToolNameFilesystemReadFile, Content: "docs"},
	}
	if hasSuccessfulVerificationResult([]providertypes.ToolCall{readCall}, readResults) {
		t.Fatalf("expected successful read to not count as verify passed")
	}

	bashNonVerifyCall := providertypes.ToolCall{
		ID:        "bash-1",
		Name:      tools.ToolNameBash,
		Arguments: `{"command":"pwd"}`,
	}
	bashResults := []tools.ToolResult{
		{ToolCallID: "bash-1", Name: tools.ToolNameBash, Content: "C:/repo"},
	}
	if hasSuccessfulVerificationResult([]providertypes.ToolCall{bashNonVerifyCall}, bashResults) {
		t.Fatalf("expected non-verification bash command to not count as verify passed")
	}
}
