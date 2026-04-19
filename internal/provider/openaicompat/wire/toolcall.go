package wire

import (
	"context"
	"strings"

	"neo-code/internal/provider/streaming"
	providertypes "neo-code/internal/provider/types"
)

// MergeToolCallDelta 将单个 tool call 增量合并到累积状态，并在必要时发出开始/增量事件。
func MergeToolCallDelta(
	ctx context.Context,
	events chan<- providertypes.StreamEvent,
	toolCalls map[int]*providertypes.ToolCall,
	delta ToolCallDelta,
) error {
	call, exists := toolCalls[delta.Index]
	if !exists {
		call = &providertypes.ToolCall{}
		toolCalls[delta.Index] = call
	}

	hadName := strings.TrimSpace(call.Name) != ""
	if id := strings.TrimSpace(delta.ID); id != "" {
		call.ID = id
	}
	if name := strings.TrimSpace(delta.Function.Name); name != "" {
		call.Name = name
	}

	if !hadName && strings.TrimSpace(call.Name) != "" {
		if err := streaming.EmitToolCallStart(ctx, events, delta.Index, call.ID, call.Name); err != nil {
			return err
		}
	}

	if args := delta.Function.Arguments; args != "" {
		call.Arguments += args
		if err := streaming.EmitToolCallDelta(ctx, events, delta.Index, call.ID, args); err != nil {
			return err
		}
	}
	return nil
}
