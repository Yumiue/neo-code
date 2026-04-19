package wire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"neo-code/internal/provider"
	"neo-code/internal/provider/streaming"
	"neo-code/internal/provider/streaming/sse"
	providertypes "neo-code/internal/provider/types"
)

type toolCallState struct {
	ID       string
	Name     string
	SawStart bool
	SawDelta bool
}

// ConsumeStream 消费 Anthropic SSE 响应并发出统一流式事件。
func ConsumeStream(ctx context.Context, body io.Reader, events chan<- providertypes.StreamEvent) error {
	reader := sse.NewBoundedReader(body)

	var (
		finishReason string
		usage        providertypes.Usage
		hasPayload   bool
		toolCallSeq  int
		currentEvent string
	)
	toolCalls := make(map[int]toolCallState)
	dataLines := make([]string, 0, 4)

	processPayload := func(eventType string, payload string) error {
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" {
			return nil
		}
		if trimmed == "[DONE]" {
			return nil
		}

		var chunk StreamPayload
		if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
			return fmt.Errorf("%sdecode stream chunk: %w", errorPrefix, err)
		}
		if chunk.Type == "" {
			chunk.Type = strings.TrimSpace(eventType)
		}
		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return errors.New(strings.TrimSpace(chunk.Error.Message))
		}

		hasPayload = true
		switch chunk.Type {
		case "message_start":
			if chunk.Message != nil && chunk.Message.Usage != nil {
				if chunk.Message.Usage.InputTokens > 0 {
					usage.InputTokens = chunk.Message.Usage.InputTokens
				}
				if chunk.Message.Usage.OutputTokens > 0 {
					usage.OutputTokens = chunk.Message.Usage.OutputTokens
				}
			}
		case "content_block_start":
			if chunk.ContentBlock == nil {
				return nil
			}
			if chunk.ContentBlock.Type == "text" && chunk.ContentBlock.Text != "" {
				return streaming.EmitTextDelta(ctx, events, chunk.ContentBlock.Text)
			}
			if chunk.ContentBlock.Type != "tool_use" {
				return nil
			}

			state := toolCalls[chunk.Index]
			if id := strings.TrimSpace(chunk.ContentBlock.ID); id != "" {
				state.ID = id
			}
			if name := strings.TrimSpace(chunk.ContentBlock.Name); name != "" {
				state.Name = name
			}
			if state.ID == "" {
				toolCallSeq++
				state.ID = "anthropic-call-" + strconv.Itoa(toolCallSeq)
			}

			emitStart := !state.SawStart
			state.SawStart = true
			toolCalls[chunk.Index] = state

			if emitStart {
				if err := streaming.EmitToolCallStart(ctx, events, chunk.Index, state.ID, state.Name); err != nil {
					return err
				}
			}
			if len(chunk.ContentBlock.Input) > 0 && !state.SawDelta {
				argsJSON, err := json.Marshal(chunk.ContentBlock.Input)
				if err != nil {
					return fmt.Errorf("%sencode tool_use input: %w", errorPrefix, err)
				}
				return streaming.EmitToolCallDelta(ctx, events, chunk.Index, state.ID, string(argsJSON))
			}
		case "content_block_delta":
			if chunk.Delta == nil {
				return nil
			}
			switch chunk.Delta.Type {
			case "text_delta":
				return streaming.EmitTextDelta(ctx, events, chunk.Delta.Text)
			case "input_json_delta":
				state := toolCalls[chunk.Index]
				if strings.TrimSpace(state.ID) == "" {
					toolCallSeq++
					state.ID = "anthropic-call-" + strconv.Itoa(toolCallSeq)
				}
				state.SawDelta = true
				toolCalls[chunk.Index] = state
				return streaming.EmitToolCallDelta(ctx, events, chunk.Index, state.ID, chunk.Delta.PartialJSON)
			}
		case "message_delta":
			if chunk.Delta != nil {
				if reason := strings.TrimSpace(chunk.Delta.StopReason); reason != "" {
					finishReason = reason
				}
			}
			if chunk.Usage != nil {
				if chunk.Usage.OutputTokens > 0 {
					usage.OutputTokens = chunk.Usage.OutputTokens
				}
				if chunk.Usage.InputTokens > 0 {
					usage.InputTokens = chunk.Usage.InputTokens
				}
			}
		case "error":
			if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
				return errors.New(strings.TrimSpace(chunk.Error.Message))
			}
		}
		return nil
	}

	flushPendingData := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		return processPayload(currentEvent, payload)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadLine()
		if err != nil && !errors.Is(err, io.EOF) {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			return fmt.Errorf("%w: %w", provider.ErrStreamInterrupted, err)
		}

		switch {
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case line == "":
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
		case strings.HasPrefix(line, ":"):
			// 注释/心跳行忽略。
		}

		if errors.Is(err, io.EOF) {
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			break
		}
	}

	if !hasPayload {
		return fmt.Errorf("%w: empty anthropic stream payload", provider.ErrStreamInterrupted)
	}
	for index, state := range toolCalls {
		if state.SawDelta && !state.SawStart {
			return fmt.Errorf("%sinvalid tool_use stream at index %d: missing content_block_start", errorPrefix, index)
		}
		if state.SawStart && strings.TrimSpace(state.Name) == "" {
			return fmt.Errorf("%sinvalid tool_use stream at index %d: missing tool name", errorPrefix, index)
		}
	}
	if usage.TotalTokens <= 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return streaming.EmitMessageDone(ctx, events, finishReason, &usage)
}
