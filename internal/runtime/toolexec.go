package runtime

import (
	"context"
	"errors"
	"strings"

	providertypes "neo-code/internal/provider/types"
)

// executeAssistantToolCalls 顺序执行 assistant 返回的全部工具调用并回写结果。
func (s *Service) executeAssistantToolCalls(
	ctx context.Context,
	state *runState,
	snapshot turnSnapshot,
	assistant providertypes.Message,
) error {
	for _, call := range assistant.ToolCalls {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.emit(ctx, EventToolStart, state.runID, state.session.ID, call)

		result, execErr := s.executeToolCallWithPermission(ctx, permissionExecutionInput{
			RunID:       state.runID,
			SessionID:   state.session.ID,
			Call:        call,
			Workdir:     snapshot.workdir,
			ToolTimeout: snapshot.toolTimeout,
		})
		if errors.Is(execErr, context.Canceled) {
			return execErr
		}
		if execErr == nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		if execErr != nil && strings.TrimSpace(result.Content) == "" {
			result.Content = execErr.Error()
		}
		if err := s.appendToolMessageAndSave(ctx, state, call, result); err != nil {
			if execErr != nil && errors.Is(err, context.Canceled) {
				s.emit(ctx, EventToolResult, state.runID, state.session.ID, result)
			}
			return err
		}
		if err := ctx.Err(); err != nil && execErr == nil {
			return err
		}

		s.emit(ctx, EventToolResult, state.runID, state.session.ID, result)
		if isSuccessfulRememberToolCall(call.Name, result, execErr) {
			state.rememberedThisRun = true
		}
		if execErr != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
	return nil
}
