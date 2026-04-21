package chatcompletions

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

// EmitFromSDKStream 消费 OpenAI SDK 的 typed stream 并发出统一流式事件。
func EmitFromSDKStream(
	ctx context.Context,
	stream any,
	events chan<- providertypes.StreamEvent,
) error {
	var (
		finishReason string
		usage        providertypes.Usage
		toolCalls    = make(map[int]*providertypes.ToolCall)
	)

	// Since we cannot easily reference the generic Stream type here without importing internal pkgs,
	// we use a more dynamic approach or just accept the type from the caller.
	// In Go, we can't easily iterate over a generic type passed as any without reflection or a specific interface.
	// But the SDK's stream has Next(), Current(), and Err() methods.

	type StreamScanner interface {
		Next() bool
		Current() openai.ChatCompletionChunk
		Err() error
	}

	typedStream, ok := stream.(StreamScanner)
	if !ok {
		return fmt.Errorf("invalid stream type: %T", stream)
	}

	for typedStream.Next() {
		chunk := typedStream.Current()

		// In v3 SDK, Usage is a struct, we check if it's non-zero.
		if chunk.Usage.TotalTokens > 0 {
			extractStreamUsage(&usage, chunk.Usage)
		}

		for _, choice := range chunk.Choices {
			if string(choice.FinishReason) != "" {
				finishReason = string(choice.FinishReason)
			}
			if choice.Delta.Content != "" {
				if err := provider.EmitTextDelta(ctx, events, choice.Delta.Content); err != nil {
					return err
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				if err := mergeToolCallDeltaFromSDK(ctx, events, toolCalls, delta); err != nil {
					return err
				}
			}
		}
	}

	if err := typedStream.Err(); err != nil {
		return fmt.Errorf("SDK stream error: %w", err)
	}

	return provider.EmitMessageDone(ctx, events, finishReason, &usage)
}

// extractStreamUsage 将 OpenAI usage 覆盖到统一 token 统计。
func extractStreamUsage(usage *providertypes.Usage, raw openai.CompletionUsage) {
	*usage = providertypes.Usage{
		InputTokens:  int(raw.PromptTokens),
		OutputTokens: int(raw.CompletionTokens),
		TotalTokens:  int(raw.TotalTokens),
	}
}

// mergeToolCallDeltaFromSDK 将单个 SDK tool call 增量合并到累积状态，并在必要时发出起始/增量事件。
func mergeToolCallDeltaFromSDK(
	ctx context.Context,
	events chan<- providertypes.StreamEvent,
	toolCalls map[int]*providertypes.ToolCall,
	delta openai.ChatCompletionChunkChoiceDeltaToolCall,
) error {
	index := int(delta.Index)
	call, exists := toolCalls[index]
	if !exists {
		call = &providertypes.ToolCall{}
		toolCalls[index] = call
	}

	hadName := strings.TrimSpace(call.Name) != ""
	if id := strings.TrimSpace(delta.ID); id != "" {
		call.ID = id
	}
	if name := strings.TrimSpace(delta.Function.Name); name != "" {
		call.Name = name
	}

	if !hadName && strings.TrimSpace(call.Name) != "" {
		if err := provider.EmitToolCallStart(ctx, events, index, call.ID, call.Name); err != nil {
			return err
		}
	}

	if args := delta.Function.Arguments; args != "" {
		call.Arguments += args
		if err := provider.EmitToolCallDelta(ctx, events, index, call.ID, args); err != nil {
			return err
		}
	}
	return nil
}
