package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	agentruntime "neo-code/internal/runtime"
	agentsession "neo-code/internal/session"
	tuistate "neo-code/internal/tui/state"
)

type permissionTestRuntime struct {
	resolveErr   error
	lastResolved agentruntime.PermissionResolutionInput
}

func (r *permissionTestRuntime) Run(ctx context.Context, input agentruntime.UserInput) error {
	return nil
}

func (r *permissionTestRuntime) Compact(ctx context.Context, input agentruntime.CompactInput) (agentruntime.CompactResult, error) {
	return agentruntime.CompactResult{}, nil
}

func (r *permissionTestRuntime) ResolvePermission(ctx context.Context, input agentruntime.PermissionResolutionInput) error {
	r.lastResolved = input
	return r.resolveErr
}

func (r *permissionTestRuntime) CancelActiveRun() bool {
	return false
}

func (r *permissionTestRuntime) Events() <-chan agentruntime.RuntimeEvent {
	ch := make(chan agentruntime.RuntimeEvent)
	close(ch)
	return ch
}

func (r *permissionTestRuntime) ListSessions(ctx context.Context) ([]agentsession.Summary, error) {
	return nil, nil
}

func (r *permissionTestRuntime) LoadSession(ctx context.Context, id string) (agentsession.Session, error) {
	return agentsession.Session{}, nil
}

func (r *permissionTestRuntime) SetSessionWorkdir(ctx context.Context, sessionID string, workdir string) (agentsession.Session, error) {
	return agentsession.Session{}, nil
}

func newPermissionTestApp(runtime agentruntime.Runtime) *App {
	input := textarea.New()
	spin := spinner.New()
	sessionList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	app := &App{
		state: tuistate.UIState{
			Focus: panelInput,
		},
		appServices: appServices{
			runtime: runtime,
		},
		appComponents: appComponents{
			keys:       newKeyMap(),
			spinner:    spin,
			sessions:   sessionList,
			input:      input,
			transcript: viewport.New(0, 0),
			activity:   viewport.New(0, 0),
		},
		appRuntimeState: appRuntimeState{
			nowFn:          time.Now,
			codeCopyBlocks: map[int]string{},
			focus:          panelInput,
			activities: []tuistate.ActivityEntry{
				{Kind: "test", Title: "seed"},
			},
		},
	}
	return app
}

func TestUpdatePendingPermissionInputSelectAndSubmit(t *testing.T) {
	runtime := &permissionTestRuntime{}
	app := newPermissionTestApp(runtime)
	app.pendingPermission = &permissionPromptState{
		Request:  agentruntime.PermissionRequestPayload{RequestID: "perm-1"},
		Selected: 0,
	}

	cmd, handled := app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || cmd != nil {
		t.Fatalf("expected handled down key without cmd, handled=%v cmd=%v", handled, cmd)
	}
	if app.pendingPermission.Selected != 1 {
		t.Fatalf("expected selection moved to 1, got %d", app.pendingPermission.Selected)
	}

	cmd, handled = app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyUp})
	if !handled || cmd != nil {
		t.Fatalf("expected handled up key without cmd, handled=%v cmd=%v", handled, cmd)
	}
	if app.pendingPermission.Selected != 0 {
		t.Fatalf("expected selection moved back to 0, got %d", app.pendingPermission.Selected)
	}

	cmd, handled = app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !handled || cmd != nil {
		t.Fatalf("expected unknown shortcut to be consumed without cmd, handled=%v cmd=%v", handled, cmd)
	}

	cmd, handled = app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatalf("expected enter key to submit permission decision, handled=%v cmd=%v", handled, cmd)
	}

	msg := cmd()
	done, ok := msg.(permissionResolutionFinishedMsg)
	if !ok {
		t.Fatalf("expected permissionResolutionFinishedMsg, got %T", msg)
	}
	if done.RequestID != "perm-1" || done.Decision != agentruntime.PermissionResolutionAllowOnce {
		t.Fatalf("unexpected submitted decision: %+v", done)
	}
	if runtime.lastResolved.Decision != agentruntime.PermissionResolutionAllowOnce {
		t.Fatalf("runtime decision mismatch: %+v", runtime.lastResolved)
	}
}

func TestUpdatePendingPermissionInputWithoutPendingState(t *testing.T) {
	app := newPermissionTestApp(&permissionTestRuntime{})
	cmd, handled := app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyEnter})
	if handled || cmd != nil {
		t.Fatalf("expected no handling when pending permission is nil, handled=%v cmd=%v", handled, cmd)
	}
}

func TestUpdatePendingPermissionInputShortcut(t *testing.T) {
	runtime := &permissionTestRuntime{}
	app := newPermissionTestApp(runtime)
	app.pendingPermission = &permissionPromptState{
		Request:  agentruntime.PermissionRequestPayload{RequestID: "perm-2"},
		Selected: 0,
	}

	cmd, handled := app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !handled || cmd == nil {
		t.Fatalf("expected shortcut n to trigger submit, handled=%v cmd=%v", handled, cmd)
	}
	msg := cmd()
	done, ok := msg.(permissionResolutionFinishedMsg)
	if !ok {
		t.Fatalf("expected permissionResolutionFinishedMsg, got %T", msg)
	}
	if done.Decision != agentruntime.PermissionResolutionReject {
		t.Fatalf("expected reject decision, got %q", done.Decision)
	}
}

func TestUpdatePendingPermissionInputSubmittingConsumesInput(t *testing.T) {
	app := newPermissionTestApp(&permissionTestRuntime{})
	app.pendingPermission = &permissionPromptState{
		Request:    agentruntime.PermissionRequestPayload{RequestID: "perm-3"},
		Selected:   0,
		Submitting: true,
	}
	cmd, handled := app.updatePendingPermissionInput(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || cmd != nil {
		t.Fatalf("expected submitting state to consume key without cmd, handled=%v cmd=%v", handled, cmd)
	}
}

func TestSubmitPermissionDecisionValidation(t *testing.T) {
	app := newPermissionTestApp(&permissionTestRuntime{})
	if cmd := app.submitPermissionDecision(agentruntime.PermissionResolutionAllowOnce); cmd != nil {
		t.Fatalf("expected nil cmd when no pending permission")
	}

	app.pendingPermission = &permissionPromptState{
		Request:  agentruntime.PermissionRequestPayload{RequestID: "  "},
		Selected: 0,
	}
	if cmd := app.submitPermissionDecision(agentruntime.PermissionResolutionAllowOnce); cmd != nil {
		t.Fatalf("expected nil cmd for empty request id")
	}
}

func TestRuntimePermissionEventHandlers(t *testing.T) {
	app := newPermissionTestApp(&permissionTestRuntime{})
	requestEvent := agentruntime.RuntimeEvent{
		Type: agentruntime.EventPermissionRequest,
		Payload: agentruntime.PermissionRequestPayload{
			RequestID: "perm-4",
			ToolName:  "bash",
			Target:    "git status",
		},
	}
	if dirty := runtimeEventPermissionRequestHandler(app, requestEvent); dirty {
		t.Fatalf("permission request should not mark transcript dirty")
	}
	if app.pendingPermission == nil || app.pendingPermission.Request.RequestID != "perm-4" {
		t.Fatalf("expected pending permission to be recorded")
	}

	resolvedEvent := agentruntime.RuntimeEvent{
		Type: agentruntime.EventPermissionResolved,
		Payload: agentruntime.PermissionResolvedPayload{
			RequestID:     "perm-4",
			Decision:      "allow",
			RememberScope: "once",
			ResolvedAs:    "approved",
		},
	}
	if dirty := runtimeEventPermissionResolvedHandler(app, resolvedEvent); dirty {
		t.Fatalf("permission resolved should not mark transcript dirty")
	}
	if app.pendingPermission != nil {
		t.Fatalf("expected pending permission to be cleared after resolved")
	}
}

func TestUpdatePermissionResolutionFinishedMessage(t *testing.T) {
	runtime := &permissionTestRuntime{}
	app := newPermissionTestApp(runtime)
	app.pendingPermission = &permissionPromptState{
		Request:    agentruntime.PermissionRequestPayload{RequestID: "perm-5"},
		Selected:   0,
		Submitting: true,
	}
	app.state.IsAgentRunning = true
	app.state.IsCompacting = true
	app.state.StatusText = "busy"

	model, _ := app.Update(permissionResolutionFinishedMsg{
		RequestID: "perm-5",
		Decision:  agentruntime.PermissionResolutionAllowOnce,
		Err:       errors.New("network"),
	})
	next := model.(App)
	if next.pendingPermission == nil || next.pendingPermission.Submitting {
		t.Fatalf("expected pending permission to remain and reset submitting on error")
	}
	if next.state.ExecutionError == "" {
		t.Fatalf("expected execution error after failed permission submit")
	}
}

func TestUpdateRuntimeClosedClearsPendingPermission(t *testing.T) {
	app := newPermissionTestApp(&permissionTestRuntime{})
	app.pendingPermission = &permissionPromptState{
		Request: agentruntime.PermissionRequestPayload{RequestID: "perm-6"},
	}
	model, _ := app.Update(RuntimeClosedMsg{})
	next := model.(App)
	if next.pendingPermission != nil {
		t.Fatalf("expected runtime closed to clear pending permission")
	}
}

func TestRunResolvePermissionForwardsRuntimeError(t *testing.T) {
	runtime := &permissionTestRuntime{resolveErr: errors.New("resolve failed")}
	cmd := runResolvePermission(runtime, "perm-7", agentruntime.PermissionResolutionReject)
	msg := cmd()
	done, ok := msg.(permissionResolutionFinishedMsg)
	if !ok {
		t.Fatalf("expected permissionResolutionFinishedMsg, got %T", msg)
	}
	if done.Err == nil || done.Err.Error() != "resolve failed" {
		t.Fatalf("expected forwarded resolve error, got %#v", done.Err)
	}
	if runtime.lastResolved.RequestID != "perm-7" || runtime.lastResolved.Decision != agentruntime.PermissionResolutionReject {
		t.Fatalf("unexpected runtime resolve input: %+v", runtime.lastResolved)
	}
}
