package wire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"neo-code/internal/provider"
	"neo-code/internal/provider/streaming"
	"neo-code/internal/provider/streaming/sse"
	providertypes "neo-code/internal/provider/types"
)

// ConsumeStream 消费 OpenAI-compatible SSE 响应并发出统一流式事件。
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

		var chunk Chunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("%sdecode stream chunk: %w", errorPrefix, err)
		}

		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return errors.New(chunk.Error.Message)
		}

		ExtractStreamUsage(&usage, chunk.Usage)

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			if choice.Delta.Content != "" {
				if err := streaming.EmitTextDelta(ctx, events, choice.Delta.Content); err != nil {
					return err
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				if err := MergeToolCallDelta(ctx, events, toolCalls, delta); err != nil {
					return err
				}
			}
		}
		return nil
	}

	finishStream := func() error {
		return streaming.EmitMessageDone(ctx, events, finishReason, &usage)
	}

	flushPendingData := func() error {
		defer func() { dataLines = dataLines[:0] }()
		return streaming.FlushDataLines(dataLines, processChunk)
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
			// SSE 注释/心跳行，忽略。
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

// ExtractStreamUsage 将 OpenAI usage 覆盖到累计 token 统计。
func ExtractStreamUsage(usage *providertypes.Usage, raw *Usage) {
	if raw == nil {
		return
	}
	*usage = providertypes.Usage{
		InputTokens:  raw.PromptTokens,
		OutputTokens: raw.CompletionTokens,
		TotalTokens:  raw.TotalTokens,
	}
}
