package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"neo-code/internal/config"
	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/acceptance"
	"neo-code/internal/runtime/controlplane"
	runtimehooks "neo-code/internal/runtime/hooks"
	"neo-code/internal/runtime/verify"
	agentsession "neo-code/internal/session"
)

func TestBeforeCompletionDecisionAcceptanceHooksOnOffParity(t *testing.T) {
	t.Parallel()

	snapshot := TurnBudgetSnapshot{Config: config.StaticDefaults().Clone(), Workdir: t.TempDir()}
	assistant := providertypes.Message{
		Role:  providertypes.RoleAssistant,
		Parts: []providertypes.ContentPart{providertypes.NewTextPart("done")},
	}

	offService := &Service{events: make(chan RuntimeEvent, 16)}
	offState := newRunState("run-hooks-off", agentsession.New("hooks-off"))
	offState.session.TaskState.VerificationProfile = agentsession.VerificationProfileTaskOnly
	offDecision, err := offService.runBeforeCompletionDecisionAcceptance(
		context.Background(),
		&offState,
		snapshot,
		assistant,
		snapshot.Workdir,
		true,
		false,
		providertypes.RoleAssistant,
	)
	if err != nil {
		t.Fatalf("hooks-off decision error = %v", err)
	}

	onService := &Service{events: make(chan RuntimeEvent, 16)}
	baseRegistry := runtimehooks.NewRegistry()
	userRegistry := runtimehooks.NewRegistry()
	repoRegistry := runtimehooks.NewRegistry()
	if err := userRegistry.Register(runtimehooks.HookSpec{
		ID:     "user-note",
		Point:  runtimehooks.HookPointBeforeCompletionDecision,
		Scope:  runtimehooks.HookScopeUser,
		Source: runtimehooks.HookSourceUser,
		Handler: func(_ context.Context, _ runtimehooks.HookContext) runtimehooks.HookResult {
			return runtimehooks.HookResult{Status: runtimehooks.HookResultPass, Message: "note"}
		},
	}); err != nil {
		t.Fatalf("register user hook: %v", err)
	}
	onService.SetHookExecutor(composeRuntimeHookExecutors(
		runtimehooks.NewExecutor(baseRegistry, nil, time.Second),
		runtimehooks.NewExecutor(userRegistry, nil, time.Second),
		runtimehooks.NewExecutor(repoRegistry, nil, time.Second),
	))
	onState := newRunState("run-hooks-on", agentsession.New("hooks-on"))
	onState.session.TaskState.VerificationProfile = agentsession.VerificationProfileTaskOnly
	onDecision, err := onService.runBeforeCompletionDecisionAcceptance(
		context.Background(),
		&onState,
		snapshot,
		assistant,
		snapshot.Workdir,
		true,
		false,
		providertypes.RoleAssistant,
	)
	if err != nil {
		t.Fatalf("hooks-on decision error = %v", err)
	}

	if offDecision.Status != onDecision.Status || offDecision.StopReason != onDecision.StopReason {
		t.Fatalf("hooks parity mismatch: off=%+v on=%+v", offDecision, onDecision)
	}
}

func TestAcceptanceDecisionRequiresCompletionAndVerification(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 16)}
	snapshot := TurnBudgetSnapshot{Config: config.StaticDefaults().Clone(), Workdir: t.TempDir()}
	assistant := providertypes.Message{Role: providertypes.RoleAssistant, Parts: []providertypes.ContentPart{providertypes.NewTextPart("done")}}

	t.Run("completion_pass_but_verification_fail_not_accepted", func(t *testing.T) {
		state := newRunState("run-verify-fail", agentsession.New("verify-fail"))
		state.session.TaskState.VerificationProfile = agentsession.VerificationProfileCreateFile
		state.session.TaskState.KeyArtifacts = []string{"missing.txt"}
		decision, err := service.beforeAcceptFinal(context.Background(), &state, snapshot, assistant, true, beforeCompletionHookSignals{})
		if err != nil {
			t.Fatalf("beforeAcceptFinal error = %v", err)
		}
		if decision.Status == acceptance.AcceptanceAccepted {
			t.Fatalf("unexpected accepted decision: %+v", decision)
		}
		if !decision.CompletionPassed || decision.VerificationPassed {
			t.Fatalf("expected completion=true verification=false, got %+v", decision)
		}
		if len(decision.VerifierResults) == 0 {
			t.Fatalf("expected verification trace in decision")
		}
	})

	t.Run("completion_fail_not_accepted_even_if_task_only", func(t *testing.T) {
		state := newRunState("run-completion-fail", agentsession.New("completion-fail"))
		state.session.TaskState.VerificationProfile = agentsession.VerificationProfileTaskOnly
		decision, err := service.beforeAcceptFinal(context.Background(), &state, snapshot, assistant, false, beforeCompletionHookSignals{})
		if err != nil {
			t.Fatalf("beforeAcceptFinal error = %v", err)
		}
		if decision.Status == acceptance.AcceptanceAccepted {
			t.Fatalf("unexpected accepted decision: %+v", decision)
		}
		if decision.CompletionPassed {
			t.Fatalf("expected completion=false, got %+v", decision)
		}
	})

	t.Run("accepted_requires_both_true", func(t *testing.T) {
		state := newRunState("run-accepted", agentsession.New("accepted"))
		state.session.TaskState.VerificationProfile = agentsession.VerificationProfileTaskOnly
		decision, err := service.beforeAcceptFinal(context.Background(), &state, snapshot, assistant, true, beforeCompletionHookSignals{})
		if err != nil {
			t.Fatalf("beforeAcceptFinal error = %v", err)
		}
		if decision.Status != acceptance.AcceptanceAccepted {
			t.Fatalf("status=%q want accepted", decision.Status)
		}
		if !decision.CompletionPassed || !decision.VerificationPassed {
			t.Fatalf("accepted must satisfy completion+verification, got %+v", decision)
		}
	})
}

func TestBeforeCompletionDecisionUserRepoCannotDirectlyTerminal(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 16)}
	baseRegistry := runtimehooks.NewRegistry()
	userRegistry := runtimehooks.NewRegistry()
	if err := userRegistry.Register(runtimehooks.HookSpec{
		ID:     "user-guard",
		Point:  runtimehooks.HookPointBeforeCompletionDecision,
		Scope:  runtimehooks.HookScopeUser,
		Source: runtimehooks.HookSourceUser,
		Handler: func(_ context.Context, _ runtimehooks.HookContext) runtimehooks.HookResult {
			return runtimehooks.HookResult{Status: runtimehooks.HookResultFailed, Message: "guard"}
		},
	}); err != nil {
		t.Fatalf("register user guard hook: %v", err)
	}
	service.SetHookExecutor(composeRuntimeHookExecutors(
		runtimehooks.NewExecutor(baseRegistry, nil, time.Second),
		runtimehooks.NewExecutor(userRegistry, nil, time.Second),
		nil,
	))

	state := newRunState("run-user-guard", agentsession.New("user-guard"))
	state.session.TaskState.VerificationProfile = agentsession.VerificationProfileTaskOnly
	snapshot := TurnBudgetSnapshot{Config: config.StaticDefaults().Clone(), Workdir: t.TempDir()}
	decision, err := service.runBeforeCompletionDecisionAcceptance(
		context.Background(),
		&state,
		snapshot,
		providertypes.Message{Role: providertypes.RoleAssistant, Parts: []providertypes.ContentPart{providertypes.NewTextPart("done")}},
		snapshot.Workdir,
		true,
		false,
		providertypes.RoleAssistant,
	)
	if err != nil {
		t.Fatalf("runBeforeCompletionDecisionAcceptance error = %v", err)
	}
	if decision.Status != acceptance.AcceptanceAccepted {
		t.Fatalf("user guard should not directly terminal-block acceptance path, got %+v", decision)
	}
	if !strings.Contains(decision.InternalSummary, "hook signals consumed") {
		t.Fatalf("expected hook signal to be consumed by acceptance input, got %q", decision.InternalSummary)
	}
}

func TestVerificationTraceEmitsStageEvents(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 32)}
	state := newRunState("run-verify-stage-events", agentsession.New("verify-stage-events"))
	decision := acceptance.AcceptanceDecision{
		Status:     acceptance.AcceptanceContinue,
		StopReason: controlplane.StopReasonVerificationFailed,
		ErrorClass: "content_mismatch",
		VerifierResults: []verify.VerificationResult{
			{
				Name:       "content_match",
				Status:     verify.VerificationSoftBlock,
				Summary:    "missing expected token",
				Reason:     "content mismatch",
				ErrorClass: "content_mismatch",
			},
		},
	}
	service.emitAcceptanceDecisionEvents(&state, decision)
	events := collectRuntimeEvents(service.Events())
	stageCount := 0
	for _, evt := range events {
		if evt.Type == EventVerificationStageFinished {
			stageCount++
		}
	}
	if stageCount == 0 {
		t.Fatal("expected verification_stage_finished events from acceptance decision trace")
	}
}

func TestVerificationFailureProducesStopReasonAndErrorClass(t *testing.T) {
	t.Parallel()

	service := &Service{events: make(chan RuntimeEvent, 16)}
	state := newRunState("run-invalid-profile", agentsession.New("invalid-profile"))
	state.session.TaskState.VerificationProfile = "bad_profile"
	snapshot := TurnBudgetSnapshot{Config: config.StaticDefaults().Clone(), Workdir: t.TempDir()}
	decision, err := service.beforeAcceptFinal(
		context.Background(),
		&state,
		snapshot,
		providertypes.Message{Role: providertypes.RoleAssistant, Parts: []providertypes.ContentPart{providertypes.NewTextPart("done")}},
		true,
		beforeCompletionHookSignals{},
	)
	if err != nil {
		t.Fatalf("beforeAcceptFinal error = %v", err)
	}
	if decision.Status != acceptance.AcceptanceFailed {
		t.Fatalf("status=%q want failed", decision.Status)
	}
	if decision.StopReason != controlplane.StopReasonVerificationConfigMissing {
		t.Fatalf("stop reason=%q want verification_config_missing", decision.StopReason)
	}
	if decision.ErrorClass == "" {
		t.Fatalf("verification failure must keep non-empty error_class: %+v", decision)
	}
}
