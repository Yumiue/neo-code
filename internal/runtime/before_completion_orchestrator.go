package runtime

import (
	"context"
	"strings"

	"neo-code/internal/runtime/decider"
	runtimehooks "neo-code/internal/runtime/hooks"
)

// beforeCompletionHookSignals 收敛 before_completion_decision 阶段 user/repo hook 的可消费信号。
type beforeCompletionHookSignals struct {
	Annotations []string
	Guards      []decider.HookGuardSignal
}

// runBeforeCompletionDecisionOrchestrator 执行 before_completion_decision 专用编排：
// 1) 先执行 user/repo hooks 收集 signal；2) 再执行 internal hooks；3) 仅返回信号，不直接裁决终态。
func (s *Service) runBeforeCompletionDecisionOrchestrator(
	ctx context.Context,
	state *runState,
	workdir string,
	completionPassed bool,
	hasToolCalls bool,
	assistantRole string,
) beforeCompletionHookSignals {
	if s == nil || s.hookExecutor == nil {
		return beforeCompletionHookSignals{}
	}

	point := runtimehooks.HookPointBeforeCompletionDecision
	hookInput := s.buildRunHookContext(
		state,
		runtimehooks.HookContext{
			Metadata: map[string]any{
				"completion_passed": completionPassed,
				"has_tool_calls":    hasToolCalls,
				"assistant_role":    strings.TrimSpace(assistantRole),
				"workdir":           strings.TrimSpace(workdir),
			},
		},
	)
	scopedCtx := withRuntimeHookEnvelope(ctx, hookRuntimeEnvelope{
		RunID:     firstNonBlank(hookRunIDFromState(state), hookInput.RunID),
		SessionID: firstNonBlank(hookSessionIDFromState(state), hookInput.SessionID),
		Turn:      hookTurnFromState(state),
		Phase:     hookPhaseFromState(state),
	})

	baseExecutor, userExecutor, repoExecutor := splitHookExecutors(s.hookExecutor)
	signals := beforeCompletionHookSignals{}

	for _, item := range []struct {
		executor HookExecutor
		source   runtimehooks.HookSource
	}{
		{executor: userExecutor, source: runtimehooks.HookSourceUser},
		{executor: repoExecutor, source: runtimehooks.HookSourceRepo},
	} {
		if item.executor == nil {
			continue
		}
		output := item.executor.Run(scopedCtx, point, hookInput.Clone())
		annotations, guards := collectBeforeCompletionSignals(output, item.source)
		signals.Annotations = append(signals.Annotations, annotations...)
		signals.Guards = append(signals.Guards, guards...)
		s.recordUserHookAnnotations(state, output)
	}

	// internal hooks 在该点位最后执行；其结果仅用于观测，不参与 user/repo signal 收集。
	if baseExecutor != nil {
		output := baseExecutor.Run(scopedCtx, point, hookInput.Clone())
		s.recordUserHookAnnotations(state, output)
	}
	return signals
}

// buildRunHookContext 构造带 run/session 元数据的 hook 输入。
func (s *Service) buildRunHookContext(state *runState, input runtimehooks.HookContext) runtimehooks.HookContext {
	runID := firstNonBlank(hookRunIDFromState(state), input.RunID)
	sessionID := firstNonBlank(hookSessionIDFromState(state), input.SessionID)
	input.RunID = firstNonBlank(input.RunID, runID)
	input.SessionID = firstNonBlank(input.SessionID, sessionID)
	if input.Metadata == nil {
		input.Metadata = make(map[string]any, 8)
	}
	input.Metadata["run_id"] = input.RunID
	input.Metadata["session_id"] = input.SessionID
	if state != nil {
		input.Metadata["runtime_run_token"] = state.runToken
		if _, exists := input.Metadata["phase"]; !exists {
			input.Metadata["phase"] = hookPhaseFromState(state)
		}
		input.Metadata["turn"] = hookTurnFromState(state)
	}
	return input
}

// splitHookExecutors 拆解 composeRuntimeHookExecutors 形成的链，恢复 internal/user/repo 三段执行器。
func splitHookExecutors(executor HookExecutor) (base HookExecutor, user HookExecutor, repo HookExecutor) {
	switch typed := executor.(type) {
	case *repoComposedHookExecutor:
		subBase, subUser, _ := splitHookExecutors(typed.base)
		return subBase, subUser, typed.repo
	case *userComposedHookExecutor:
		subBase, _, subRepo := splitHookExecutors(typed.base)
		return subBase, typed.user, subRepo
	default:
		return executor, nil, nil
	}
}

// collectBeforeCompletionSignals 从 user/repo hook 结果提取 annotation 与 guard 信号。
func collectBeforeCompletionSignals(
	output runtimehooks.RunOutput,
	defaultSource runtimehooks.HookSource,
) ([]string, []decider.HookGuardSignal) {
	if len(output.Results) == 0 {
		return nil, nil
	}
	annotations := make([]string, 0, len(output.Results))
	guards := make([]decider.HookGuardSignal, 0, len(output.Results))
	for _, result := range output.Results {
		source := strings.TrimSpace(string(result.Source))
		if source == "" {
			source = strings.TrimSpace(string(defaultSource))
		}
		message := strings.TrimSpace(result.Message)
		errText := strings.TrimSpace(result.Error)
		if message != "" {
			annotations = append(annotations, message)
		}

		isGuard := result.Status == runtimehooks.HookResultFailed
		if !isGuard && strings.Contains(strings.ToLower(errText), "block downgraded") {
			isGuard = true
		}
		if !isGuard && strings.Contains(strings.ToLower(message), "block downgraded") {
			isGuard = true
		}
		if !isGuard {
			continue
		}
		guard := decider.HookGuardSignal{
			HookID:  strings.TrimSpace(result.HookID),
			Source:  source,
			Message: firstNonBlank(message, errText),
		}
		guards = append(guards, guard)
	}
	return annotations, guards
}
