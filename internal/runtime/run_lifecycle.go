package runtime

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/controlplane"
)

// transitionRunState 在生命周期变化时校验迁移并发出 phase_changed 事件。
func (s *Service) transitionRunState(ctx context.Context, state *runState, next controlplane.RunState) error {
	if state == nil || state.lifecycle == next {
		return nil
	}

	from := state.lifecycle
	if err := controlplane.ValidateRunStateTransition(from, next); err != nil {
		return err
	}

	state.lifecycle = next
	_ = s.emitRunScoped(ctx, EventPhaseChanged, state, PhaseChangedPayload{
		From: string(from),
		To:   string(next),
	})
	return nil
}

// withTemporaryRunState 在短生命周期治理态内执行回调，随后恢复到进入前的运行态。
func (s *Service) withTemporaryRunState(
	ctx context.Context,
	state *runState,
	temporary controlplane.RunState,
	fn func() error,
) error {
	if state == nil {
		return fn()
	}

	previous := state.lifecycle
	if err := s.transitionRunState(ctx, state, temporary); err != nil {
		return err
	}

	runErr := fn()
	restoreState := previous
	if runErr != nil && restoreState == "" {
		restoreState = temporary
	}
	if restoreState != "" && restoreState != state.lifecycle {
		if err := s.transitionRunState(ctx, state, restoreState); err != nil && runErr == nil {
			runErr = err
		}
	}
	return runErr
}

// emitRunTermination 在 Run 退出时决议并发出唯一的 stop_reason_decided 事件。
func (s *Service) emitRunTermination(ctx context.Context, input UserInput, state *runState, err error) {
	runID := strings.TrimSpace(input.RunID)
	sessionID := strings.TrimSpace(input.SessionID)
	if state != nil {
		if strings.TrimSpace(state.runID) != "" {
			runID = state.runID
		}
		if strings.TrimSpace(state.session.ID) != "" {
			sessionID = state.session.ID
		}
		if state.stopEmitted {
			return
		}
		state.stopEmitted = true
		if state.lifecycle != "" && state.lifecycle != controlplane.RunStateStopped {
			state.lifecycle = controlplane.RunStateStopped
		}
	}

	in := controlplane.StopInput{}
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			in.UserInterrupted = true
		default:
			in.FatalError = err
		}
	} else {
		in.Completed = true
	}

	reason, detail := controlplane.DecideStopReason(in)
	turn := turnUnspecified
	phase := ""
	if state != nil {
		turn = state.turn
		if state.lifecycle != "" {
			phase = string(state.lifecycle)
		}
	}

	emitCtx, cancel := stopReasonEmitContext(ctx)
	defer cancel()
	_ = s.emitWithEnvelope(emitCtx, RuntimeEvent{
		Type:           EventStopReasonDecided,
		RunID:          runID,
		SessionID:      sessionID,
		Turn:           turn,
		Phase:          phase,
		Timestamp:      time.Now(),
		PayloadVersion: controlplane.PayloadVersion,
		Payload:        StopReasonDecidedPayload{Reason: reason, Detail: detail},
	})
}

// stopReasonEmitContext 为终止事件提供可用发送窗口，避免继承已取消上下文导致事件丢失。
func stopReasonEmitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil && ctx.Err() == nil {
		return context.WithTimeout(ctx, terminationEventEmitTimeout)
	}
	return context.WithTimeout(context.Background(), terminationEventEmitTimeout)
}

// handleRunError 统一转换 runtime 终止错误，保证取消语义收敛到同一路径。
func (s *Service) handleRunError(ctx context.Context, runID string, sessionID string, err error) error {
	_ = ctx
	_ = runID
	_ = sessionID
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return err
}

// isRetryableProviderError 判断 provider 错误是否允许 runtime 级重试。
func isRetryableProviderError(err error) bool {
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	return providerErr.Retryable
}

// providerRetryBackoff 计算 runtime 级 provider 重试等待时长。
func providerRetryBackoff(attempt int) time.Duration {
	wait := providerRetryBaseWait << (attempt - 1)
	jitter := float64(wait) * (0.5 + rand.Float64())
	wait = time.Duration(jitter)
	if wait > providerRetryMaxWait {
		wait = providerRetryMaxWait
	}
	return wait
}

// cloneMessages 深拷贝消息切片，避免后台调度读取到后续运行态修改。
func cloneMessages(messages []providertypes.Message) []providertypes.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]providertypes.Message, 0, len(messages))
	for _, message := range messages {
		next := message
		if len(message.ToolCalls) > 0 {
			next.ToolCalls = append([]providertypes.ToolCall(nil), message.ToolCalls...)
		}
		if len(message.ToolMetadata) > 0 {
			next.ToolMetadata = make(map[string]string, len(message.ToolMetadata))
			for key, value := range message.ToolMetadata {
				next.ToolMetadata[key] = value
			}
		}
		cloned = append(cloned, next)
	}
	return cloned
}
