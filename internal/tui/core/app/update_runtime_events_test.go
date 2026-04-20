package tui

import (
	"errors"
	"strings"
	"testing"

	providertypes "neo-code/internal/provider/types"
	agentruntime "neo-code/internal/runtime"
	"neo-code/internal/runtime/controlplane"
	agentsession "neo-code/internal/session"
	tuiservices "neo-code/internal/tui/services"
)

func TestRuntimeEventPhaseChangedHandlerBranches(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)
	if handled := runtimeEventPhaseChangedHandler(&app, agentruntime.RuntimeEvent{Payload: "invalid"}); handled {
		t.Fatalf("expected invalid payload to return false")
	}

	cases := []struct {
		to        string
		wantValue float64
		wantLabel string
	}{
		{to: " plan ", wantValue: 0.3, wantLabel: "Planning"},
		{to: "execute", wantValue: 0.6, wantLabel: "Running tools"},
		{to: "VERIFY", wantValue: 0.82, wantLabel: "Verifying"},
	}
	for _, tc := range cases {
		app.clearRunProgress()
		handled := runtimeEventPhaseChangedHandler(&app, agentruntime.RuntimeEvent{
			Payload: agentruntime.PhaseChangedPayload{To: tc.to},
		})
		if handled {
			t.Fatalf("expected phase handler to return false")
		}
		if !app.runProgressKnown || app.runProgressValue != tc.wantValue || app.runProgressLabel != tc.wantLabel {
			t.Fatalf("unexpected progress for %q: known=%v value=%v label=%q", tc.to, app.runProgressKnown, app.runProgressValue, app.runProgressLabel)
		}
	}
}

func TestRuntimeEventStopReasonDecidedHandlerBranches(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)
	app.pendingPermission = &permissionPromptState{
		Request: agentruntime.PermissionRequestPayload{RequestID: "perm-1"},
	}
	app.state.IsAgentRunning = true
	app.state.StreamingReply = true
	app.state.CurrentTool = "bash"
	app.state.ActiveRunID = "run-1"
	app.state.ExecutionError = "should-clear"
	app.setRunProgress(0.8, "running")

	if handled := runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{Payload: 123}); handled {
		t.Fatalf("expected invalid payload to return false")
	}

	handled := runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason(" success ")},
	})
	if handled {
		t.Fatalf("expected handler to return false")
	}
	if app.state.IsAgentRunning || app.state.StreamingReply || app.state.CurrentTool != "" || app.state.ActiveRunID != "" {
		t.Fatalf("expected run flags to be reset")
	}
	if app.pendingPermission != nil {
		t.Fatalf("expected pending permission to be cleared")
	}
	if app.runProgressKnown {
		t.Fatalf("expected run progress to be cleared")
	}
	if app.state.StatusText != statusReady {
		t.Fatalf("expected success status %q, got %q", statusReady, app.state.StatusText)
	}

	app.state.ExecutionError = ""
	app.state.StatusText = "not-ready"
	runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason("success")},
	})
	if app.state.StatusText != statusReady {
		t.Fatalf("expected success with empty execution error to set ready status")
	}

	app.state.ExecutionError = "boom"
	app.state.StatusText = ""
	runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason("success")},
	})
	if app.state.StatusText == statusReady {
		t.Fatalf("expected success branch to keep status unchanged when execution error exists")
	}

	runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason("canceled")},
	})
	if app.state.ExecutionError != "" || app.state.StatusText != statusCanceled {
		t.Fatalf("expected canceled state to clear error and set canceled status")
	}

	runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason("error"), Detail: "  "},
	})
	if app.state.StatusText != "runtime stopped" || app.state.ExecutionError != "runtime stopped" {
		t.Fatalf("expected default stop detail, got status=%q err=%q", app.state.StatusText, app.state.ExecutionError)
	}

	runtimeEventStopReasonDecidedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.StopReasonDecidedPayload{Reason: controlplane.StopReason("error"), Detail: "explicit failure"},
	})
	if app.state.StatusText != "explicit failure" || app.state.ExecutionError != "explicit failure" {
		t.Fatalf("expected explicit stop detail to be surfaced")
	}
}

func TestRuntimeEventHandlerRegistryContainsRenamedEvents(t *testing.T) {
	t.Parallel()

	if _, ok := runtimeEventHandlerRegistry[agentruntime.EventPhaseChanged]; !ok {
		t.Fatalf("expected phase_changed handler to be registered")
	}
	if _, ok := runtimeEventHandlerRegistry[agentruntime.EventStopReasonDecided]; !ok {
		t.Fatalf("expected stop_reason_decided handler to be registered")
	}
	if _, ok := runtimeEventHandlerRegistry[agentruntime.EventPermissionRequested]; !ok {
		t.Fatalf("expected permission_requested handler to be registered")
	}
	if _, ok := runtimeEventHandlerRegistry[agentruntime.EventCompactApplied]; !ok {
		t.Fatalf("expected compact_applied handler to be registered")
	}
}

func TestShouldHandleRuntimeEventFiltersBySessionAndRun(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)
	app.state.ActiveSessionID = "session-active"
	app.state.ActiveRunID = "run-active"

	if app.shouldHandleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventAgentChunk,
		SessionID: "session-other",
		RunID:     "run-active",
	}) {
		t.Fatalf("expected mismatched session event to be ignored")
	}
	if app.shouldHandleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventAgentChunk,
		SessionID: "session-active",
		RunID:     "run-other",
	}) {
		t.Fatalf("expected mismatched run event to be ignored")
	}
	if !app.shouldHandleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventAgentChunk,
		SessionID: "session-active",
		RunID:     "run-active",
	}) {
		t.Fatalf("expected matched event to be handled")
	}
}

func TestRuntimeEventMultimodalHandlers(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)

	if handled := runtimeEventInputNormalizedHandler(&app, agentruntime.RuntimeEvent{Payload: "bad"}); handled {
		t.Fatalf("expected invalid normalized payload to return false")
	}
	runtimeEventInputNormalizedHandler(&app, agentruntime.RuntimeEvent{
		RunID: "run-1",
		Payload: agentruntime.InputNormalizedPayload{
			TextLength: 12,
			ImageCount: 2,
		},
	})
	if app.state.ActiveRunID != "run-1" {
		t.Fatalf("expected active run id to be updated, got %q", app.state.ActiveRunID)
	}
	if len(app.activities) == 0 {
		t.Fatalf("expected input normalized activity to be appended")
	}
	last := app.activities[len(app.activities)-1]
	if last.Title != "Input normalized" || !strings.Contains(last.Detail, "images=2") {
		t.Fatalf("unexpected normalized activity: %+v", last)
	}

	before := len(app.activities)
	runtimeEventAssetSavedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.AssetSavedPayload{
			AssetID: "asset-1",
			Path:    "/tmp/chart.png",
		},
	})
	if len(app.activities) != before+1 {
		t.Fatalf("expected saved attachment activity appended")
	}
	last = app.activities[len(app.activities)-1]
	if last.Title != "Saved attachment" || !strings.Contains(last.Detail, "chart.png") {
		t.Fatalf("unexpected asset saved activity: %+v", last)
	}
	if handled := runtimeEventAssetSavedHandler(&app, agentruntime.RuntimeEvent{Payload: 123}); handled {
		t.Fatalf("expected invalid asset_saved payload to return false")
	}

	runtimeEventAssetSaveFailedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.AssetSaveFailedPayload{Message: " failed "},
	})
	if app.state.ExecutionError != "failed" || app.state.StatusText != "failed" {
		t.Fatalf("expected failed status to be surfaced, got status=%q err=%q", app.state.StatusText, app.state.ExecutionError)
	}
	last = app.activities[len(app.activities)-1]
	if !last.IsError || last.Title != "Failed to save attachment" {
		t.Fatalf("unexpected asset save failed activity: %+v", last)
	}
	runtimeEventAssetSaveFailedHandler(&app, agentruntime.RuntimeEvent{
		Payload: agentruntime.AssetSaveFailedPayload{},
	})
	if app.state.ExecutionError != "failed to save attachment" || app.state.StatusText != "failed to save attachment" {
		t.Fatalf("expected default failed message, got status=%q err=%q", app.state.StatusText, app.state.ExecutionError)
	}
	if handled := runtimeEventAssetSaveFailedHandler(&app, agentruntime.RuntimeEvent{Payload: true}); handled {
		t.Fatalf("expected invalid asset_save_failed payload to return false")
	}
}

func TestHandleRuntimeEventRoutesByRegistryWithoutBindingTransientSession(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)
	handled := app.handleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventAssetSaved,
		SessionID: "session-1",
		Payload:   agentruntime.AssetSavedPayload{AssetID: "asset-1"},
	})
	if handled {
		t.Fatalf("expected asset_saved handler to return false")
	}
	if app.state.ActiveSessionID != "" {
		t.Fatalf("expected active session to stay empty for non-stable event, got %q", app.state.ActiveSessionID)
	}
	if len(app.activities) == 0 || app.activities[len(app.activities)-1].Title != "Saved attachment" {
		t.Fatalf("expected saved attachment activity")
	}

	if app.handleRuntimeEvent(agentruntime.RuntimeEvent{Type: "unknown_event", SessionID: "session-1"}) {
		t.Fatalf("expected unknown event handler result to be false")
	}
}

func TestSubAgentEventPayloadParsers(t *testing.T) {
	t.Parallel()

	payload, ok := parseSubAgentEventPayload(agentruntime.SubAgentEventPayload{
		TaskID:    "task-1",
		Reason:    "ok",
		Error:     "none",
		Attempts:  2,
		QueueSize: 3,
		Running:   1,
	})
	if !ok || payload.TaskID != "task-1" || payload.Attempts != 2 {
		t.Fatalf("parseSubAgentEventPayload(struct) = %+v, %v", payload, ok)
	}

	payload, ok = parseSubAgentEventPayload(&agentruntime.SubAgentEventPayload{TaskID: "task-2"})
	if !ok || payload.TaskID != "task-2" {
		t.Fatalf("parseSubAgentEventPayload(pointer) = %+v, %v", payload, ok)
	}

	if _, ok := parseSubAgentEventPayload((*agentruntime.SubAgentEventPayload)(nil)); ok {
		t.Fatalf("parseSubAgentEventPayload(nil pointer) should fail")
	}

	payload, ok = parseSubAgentEventPayload(map[string]any{
		"task_id":    " task-3 ",
		"reason":     " blocked ",
		"error":      " denied ",
		"attempts":   int64(4),
		"queue_size": float64(5),
		"running":    "bad",
	})
	if !ok {
		t.Fatalf("parseSubAgentEventPayload(map) should succeed")
	}
	if payload.TaskID != "task-3" || payload.Reason != "blocked" || payload.Error != "denied" {
		t.Fatalf("unexpected parsed payload: %+v", payload)
	}
	if payload.Attempts != 4 || payload.QueueSize != 5 || payload.Running != 0 {
		t.Fatalf("unexpected numeric parsing: %+v", payload)
	}

	if _, ok := parseSubAgentEventPayload(123); ok {
		t.Fatalf("parseSubAgentEventPayload(non-map) should fail")
	}
	if got := parsePayloadInt(true); got != 0 {
		t.Fatalf("parsePayloadInt(bool) = %d, want 0", got)
	}
}

func TestRuntimeEventSubAgentTaskLifecycleHandlerBranches(t *testing.T) {
	t.Parallel()

	app, runtime := newTestApp(t)
	app.state.ActiveSessionID = "s1"
	runtime.loadSessions = map[string]agentsession.Session{
		"s1": agentsession.New("s1"),
	}

	if handled := runtimeEventSubAgentTaskLifecycleHandler(&app, agentruntime.RuntimeEvent{Payload: "bad"}); handled {
		t.Fatalf("expected invalid payload to return false")
	}

	tests := []struct {
		name       string
		eventType  agentruntime.EventType
		payload    any
		wantTitle  string
		wantError  bool
		wantLabel  string
		wantKnown  bool
		sessionID  string
		loadErr    error
		wantDetail string
	}{
		{
			name:      "started sets progress",
			eventType: agentruntime.EventSubAgentTaskStarted,
			payload: map[string]any{
				"task_id":  "task-start",
				"reason":   "boot",
				"attempts": 1,
			},
			wantTitle:  "Subagent task started",
			wantLabel:  "Running subagent",
			wantKnown:  true,
			wantDetail: "task=task-start attempt=1 reason=boot",
		},
		{
			name:      "progress defaults task id and reason",
			eventType: agentruntime.EventSubAgentTaskProgress,
			payload: map[string]any{
				"task_id":  "",
				"attempts": 0,
			},
			wantTitle:  "Subagent task progress",
			wantLabel:  "Subagent progressing",
			wantKnown:  true,
			wantDetail: "task=unknown-task attempt=0 reason=ok",
		},
		{
			name:      "retried uses error fallback reason",
			eventType: agentruntime.EventSubAgentTaskRetried,
			payload: map[string]any{
				"task_id":    "task-retry",
				"attempts":   2,
				"reason":     "",
				"error":      "timeout",
				"queue_size": 1,
			},
			wantTitle:  "Subagent task retried",
			wantDetail: "task=task-retry attempt=2 reason=timeout",
		},
		{
			name:      "blocked",
			eventType: agentruntime.EventSubAgentTaskBlocked,
			payload: map[string]any{
				"task_id":  "task-blocked",
				"reason":   "deps_unmet",
				"attempts": 3,
			},
			wantTitle:  "Subagent task blocked",
			wantDetail: "task=task-blocked attempt=3 reason=deps_unmet",
		},
		{
			name:      "completed sets progress",
			eventType: agentruntime.EventSubAgentTaskCompleted,
			payload: map[string]any{
				"task_id":  "task-done",
				"attempts": 1,
				"reason":   "ok",
			},
			wantTitle:  "Subagent task completed",
			wantLabel:  "Subagent completed",
			wantKnown:  true,
			wantDetail: "task=task-done attempt=1 reason=ok",
		},
		{
			name:      "failed marks error and falls back active session id",
			eventType: agentruntime.EventSubAgentTaskFailed,
			payload: map[string]any{
				"task_id":  "task-failed",
				"attempts": 2,
				"reason":   "boom",
			},
			sessionID:  "",
			wantTitle:  "Subagent task failed",
			wantError:  true,
			wantDetail: "task=task-failed attempt=2 reason=boom",
		},
		{
			name:      "canceled refresh failure still emits activity",
			eventType: agentruntime.EventSubAgentTaskCanceled,
			payload: map[string]any{
				"task_id":  "task-canceled",
				"attempts": 5,
				"reason":   "stopped",
			},
			sessionID:  "s1",
			loadErr:    errors.New("load failed"),
			wantTitle:  "Subagent task canceled",
			wantError:  true,
			wantDetail: "task=task-canceled attempt=5 reason=stopped",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			runtime.loadSessionErr = tt.loadErr
			before := len(app.activities)

			sessionID := tt.sessionID
			if sessionID == "" {
				sessionID = "s1"
			}
			runtimeEventSubAgentTaskLifecycleHandler(&app, agentruntime.RuntimeEvent{
				Type:      tt.eventType,
				SessionID: sessionID,
				Payload:   tt.payload,
			})

			if len(app.activities) <= before {
				t.Fatalf("expected activity appended")
			}
			last := app.activities[len(app.activities)-1]
			if last.Title != tt.wantTitle {
				t.Fatalf("activity title = %q, want %q", last.Title, tt.wantTitle)
			}
			if last.IsError != tt.wantError {
				t.Fatalf("activity IsError = %v, want %v", last.IsError, tt.wantError)
			}
			if !strings.Contains(last.Detail, tt.wantDetail) {
				t.Fatalf("activity detail = %q, want contains %q", last.Detail, tt.wantDetail)
			}
			if tt.wantKnown && (!app.runProgressKnown || app.runProgressLabel != tt.wantLabel) {
				t.Fatalf("run progress = known:%v label:%q, want known true label %q", app.runProgressKnown, app.runProgressLabel, tt.wantLabel)
			}
		})
	}
}

func TestRuntimeEventSubAgentDispatchFinishedHandler(t *testing.T) {
	t.Parallel()

	app, runtime := newTestApp(t)
	app.state.ActiveSessionID = "active"
	runtime.loadSessions = map[string]agentsession.Session{
		"active": agentsession.New("active"),
	}

	if handled := runtimeEventSubAgentDispatchFinishedHandler(&app, agentruntime.RuntimeEvent{Payload: 1}); handled {
		t.Fatalf("expected invalid payload to return false")
	}

	before := len(app.activities)
	runtimeEventSubAgentDispatchFinishedHandler(&app, agentruntime.RuntimeEvent{
		Type:      agentruntime.EventSubAgentDispatchFinished,
		SessionID: "active",
		Payload: map[string]any{
			"queue_size": 3,
			"running":    1,
			"reason":     "dispatch_round_finished",
		},
	})
	if len(app.activities) != before+1 {
		t.Fatalf("expected one dispatch activity appended")
	}
	last := app.activities[len(app.activities)-1]
	if last.Title != "Subagent dispatch finished" || last.IsError {
		t.Fatalf("unexpected dispatch activity: %+v", last)
	}
	if !strings.Contains(last.Detail, "queue=3 running=1 reason=dispatch_round_finished") {
		t.Fatalf("dispatch detail = %q", last.Detail)
	}

	runtime.loadSessionErr = errors.New("load failed")
	before = len(app.activities)
	runtimeEventSubAgentDispatchFinishedHandler(&app, agentruntime.RuntimeEvent{
		SessionID: "active",
		Payload: map[string]any{
			"queue_size": 0,
			"running":    0,
			"reason":     "none",
		},
	})
	if len(app.activities) != before+2 {
		t.Fatalf("expected refresh error + dispatch activities, got delta=%d", len(app.activities)-before)
	}
}

func TestHandleRuntimeEventBindsSessionFromStableEvents(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)

	app.handleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventUserMessage,
		SessionID: "session-user",
		RunID:     "run-1",
		Payload: providertypes.Message{
			Role:  providertypes.RoleUser,
			Parts: []providertypes.ContentPart{providertypes.NewTextPart("hi")},
		},
	})
	if app.state.ActiveSessionID != "session-user" {
		t.Fatalf("expected active session from user_message, got %q", app.state.ActiveSessionID)
	}

	app.state.ActiveSessionID = ""
	app.handleRuntimeEvent(agentruntime.RuntimeEvent{
		Type:      agentruntime.EventType(tuiservices.RuntimeEventRunContext),
		SessionID: "session-context",
		Payload: tuiservices.RuntimeRunContextPayload{
			Provider: "openai",
			Model:    "gpt-5.4",
		},
	})
	if app.state.ActiveSessionID != "session-context" {
		t.Fatalf("expected active session from run_context, got %q", app.state.ActiveSessionID)
	}
}
