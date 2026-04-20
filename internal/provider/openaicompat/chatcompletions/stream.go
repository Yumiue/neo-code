package chatcompletions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat/sse"
	providertypes "neo-code/internal/provider/types"
)

// StreamToolCallDelta 表示流式响应中的 tool call 增量。
type StreamToolCallDelta struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

// StreamUsage 表示流式 chunk 中返回的 token 使用信息。
type StreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolCallDelta 是 Chat Completions 流式 tool call 增量的兼容别名。
type ToolCallDelta = StreamToolCallDelta

// Usage 是 Chat Completions 流式 usage 结构的兼容别名。
type Usage = StreamUsage

// StreamChunk 表示 Chat Completions SSE 流中的单个 payload。
type StreamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string                `json:"role,omitempty"`
			Content   string                `json:"content,omitempty"`
			ToolCalls []StreamToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *StreamUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ConsumeStream 消费 Chat Completions SSE 响应并发出统一流式事件。
func ConsumeStream(
	ctx context.Context,
	body io.Reader,
	events chan<- providertypes.StreamEvent,
) error {
	reader := sse.NewBoundedReader(body)

	var (
		finishReason string
		usage        providertypes.Usage
		done         bool
		toolCalls    = make(map[int]*providertypes.ToolCall)
	)

	dataLines := make([]string, 0, 4)

	processChunk := func(payload string) error {
		if strings.TrimSpace(payload) == "[DONE]" {
			done = true
			return nil
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("%sdecode stream chunk: %w", errorPrefix, err)
		}

		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return errors.New(chunk.Error.Message)
		}

		extractStreamUsage(&usage, chunk.Usage)

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			if choice.Delta.Content != "" {
				if err := provider.EmitTextDelta(ctx, events, choice.Delta.Content); err != nil {
					return err
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				if err := mergeToolCallDelta(ctx, events, toolCalls, delta); err != nil {
					return err
				}
			}
		}
		return nil
	}

	finishStream := func() error {
		return provider.EmitMessageDone(ctx, events, finishReason, &usage)
	}

	flushPendingData := func() error {
		defer func() { dataLines = dataLines[:0] }()
		return provider.FlushDataLines(dataLines, processChunk)
	}

	for {
		if !done {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		line, err := reader.ReadLine()
		if err != nil && !errors.Is(err, io.EOF) {
			if done {
				if flushErr := flushPendingData(); flushErr != nil {
					return flushErr
				}
				return finishStream()
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			if strings.TrimSpace(finishReason) != "" {
				return finishStream()
			}
			return fmt.Errorf("%w: %w", provider.ErrStreamInterrupted, err)
		}

		switch {
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				if flushErr := flushPendingData(); flushErr != nil {
					return flushErr
				}
				done = true
			} else {
				dataLines = append(dataLines, data)
			}
		case line == "":
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			if done {
				return finishStream()
			}
		case strings.HasPrefix(line, ":"):
		}

		if errors.Is(err, io.EOF) {
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			if done {
				return finishStream()
			}
			if strings.TrimSpace(finishReason) != "" {
				return finishStream()
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("%w: missing [DONE] marker before EOF", provider.ErrStreamInterrupted)
		}
	}
}

// extractStreamUsage 将 OpenAI usage 覆盖到统一 token 统计。
func extractStreamUsage(usage *providertypes.Usage, raw *StreamUsage) {
	if raw == nil {
		return
	}
	*usage = providertypes.Usage{
		InputTokens:  raw.PromptTokens,
		OutputTokens: raw.CompletionTokens,
		TotalTokens:  raw.TotalTokens,
	}
}

// ExtractStreamUsage 将 Chat Completions usage 覆盖到统一 token 统计。
func ExtractStreamUsage(usage *providertypes.Usage, raw *Usage) {
	extractStreamUsage(usage, raw)
}

// mergeToolCallDelta 将单个 tool call 增量合并到累积状态，并在必要时发出起始/增量事件。
func mergeToolCallDelta(
	ctx context.Context,
	events chan<- providertypes.StreamEvent,
	toolCalls map[int]*providertypes.ToolCall,
	delta StreamToolCallDelta,
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
		if err := provider.EmitToolCallStart(ctx, events, delta.Index, call.ID, call.Name); err != nil {
			return err
		}
	}

	if args := delta.Function.Arguments; args != "" {
		call.Arguments += args
		if err := provider.EmitToolCallDelta(ctx, events, delta.Index, call.ID, args); err != nil {
			return err
		}
	}
	return nil
}

// MergeToolCallDelta 合并 Chat Completions tool call 增量并发送统一事件。
func MergeToolCallDelta(
	ctx context.Context,
	events chan<- providertypes.StreamEvent,
	toolCalls map[int]*providertypes.ToolCall,
	delta ToolCallDelta,
) error {
	return mergeToolCallDelta(ctx, events, toolCalls, delta)
}
