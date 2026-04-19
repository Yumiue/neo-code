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

// ConsumeStream 消费 Gemini SSE 响应并发出统一流式事件。
func ConsumeStream(ctx context.Context, body io.Reader, events chan<- providertypes.StreamEvent) error {
	reader := sse.NewBoundedReader(body)

	var (
		finishReason string
		usage        providertypes.Usage
		hasPayload   bool
		callSeq      int
	)
	dataLines := make([]string, 0, 4)

	processChunk := func(payload string) error {
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" {
			return nil
		}
		if trimmed == "[DONE]" {
			return nil
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
			return fmt.Errorf("%sdecode stream chunk: %w", errorPrefix, err)
		}
		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return errors.New(strings.TrimSpace(chunk.Error.Message))
		}

		hasPayload = true
		extractUsage(&usage, chunk.Usage)
		for _, candidate := range chunk.Candidates {
			if reason := normalizeFinishReason(candidate.FinishReason); reason != "" {
				finishReason = reason
			}
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					if err := streaming.EmitTextDelta(ctx, events, part.Text); err != nil {
						return err
					}
				}
				if part.FunctionCall != nil {
					callSeq++
					callID := strings.TrimSpace(part.FunctionCall.ID)
					if callID == "" {
						callID = "gemini-call-" + strconv.Itoa(callSeq)
					}
					name := strings.TrimSpace(part.FunctionCall.Name)
					if name == "" {
						continue
					}
					if err := streaming.EmitToolCallStart(ctx, events, callSeq-1, callID, name); err != nil {
						return err
					}
					argsJSON, err := encodeArguments(part.FunctionCall.Args)
					if err != nil {
						return err
					}
					if err := streaming.EmitToolCallDelta(ctx, events, callSeq-1, callID, argsJSON); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}

	flushPendingData := func() error {
		defer func() { dataLines = dataLines[:0] }()
		return streaming.FlushDataLines(dataLines, processChunk)
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
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case line == "":
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
		case strings.HasPrefix(line, "event:"), strings.HasPrefix(line, ":"):
			// Gemini 事件名与注释行当前不参与业务处理。
		}

		if errors.Is(err, io.EOF) {
			if flushErr := flushPendingData(); flushErr != nil {
				return flushErr
			}
			break
		}
	}

	if !hasPayload {
		return fmt.Errorf("%w: empty gemini stream payload", provider.ErrStreamInterrupted)
	}
	return streaming.EmitMessageDone(ctx, events, finishReason, &usage)
}

// extractUsage 从 Gemini usageMetadata 中抽取统一 token 统计。
func extractUsage(usage *providertypes.Usage, raw *UsageMetadata) {
	if raw == nil {
		return
	}
	usage.InputTokens = raw.PromptTokenCount
	usage.OutputTokens = raw.CandidatesTokenCount
	usage.TotalTokens = raw.TotalTokenCount
}

// encodeArguments 将函数参数对象编码为 JSON 字符串，供统一 tool_call_delta 事件复用。
func encodeArguments(args map[string]any) (string, error) {
	if len(args) == 0 {
		return "{}", nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("%sencode function args: %w", errorPrefix, err)
	}
	return string(encoded), nil
}

// normalizeFinishReason 规范化 Gemini finish reason，便于上层统一处理。
func normalizeFinishReason(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
