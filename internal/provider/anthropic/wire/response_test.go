package wire

import (
	"context"
	"strings"
	"testing"

	providertypes "neo-code/internal/provider/types"
)

func TestConsumeStreamAllowsDeltaBeforeStart(t *testing.T) {
	t.Parallel()

	events := make(chan providertypes.StreamEvent, 16)
	stream := strings.Join([]string{
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"README.md\"}"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call_1","name":"filesystem_read_file"}}`,
		"",
		"event: message_start",
		`data: {"type":"message_start","message":{"usage":{"input_tokens":3}}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	if err := ConsumeStream(context.Background(), strings.NewReader(stream), events); err != nil {
		t.Fatalf("ConsumeStream() error = %v", err)
	}

	drained := drainEvents(events)
	var foundDelta, foundStart, foundDone bool
	for _, event := range drained {
		switch event.Type {
		case providertypes.StreamEventToolCallDelta:
			foundDelta = true
		case providertypes.StreamEventToolCallStart:
			foundStart = true
		case providertypes.StreamEventMessageDone:
			foundDone = true
			payload, err := event.MessageDoneValue()
			if err != nil {
				t.Fatalf("MessageDoneValue() error = %v", err)
			}
			if payload.FinishReason != "tool_use" {
				t.Fatalf("expected finish reason tool_use, got %q", payload.FinishReason)
			}
			if payload.Usage == nil || payload.Usage.TotalTokens != 5 {
				t.Fatalf("expected total tokens 5, got %+v", payload.Usage)
			}
		}
	}
	if !foundDelta || !foundStart || !foundDone {
		t.Fatalf("expected delta/start/done events, got %+v", drained)
	}
}

func TestConsumeStreamRejectsDeltaWithoutStart(t *testing.T) {
	t.Parallel()

	events := make(chan providertypes.StreamEvent, 8)
	stream := strings.Join([]string{
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"README.md\"}"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	err := ConsumeStream(context.Background(), strings.NewReader(stream), events)
	if err == nil || !strings.Contains(err.Error(), "missing content_block_start") {
		t.Fatalf("expected missing content_block_start error, got %v", err)
	}
}

func drainEvents(events <-chan providertypes.StreamEvent) []providertypes.StreamEvent {
	var drained []providertypes.StreamEvent
	for {
		select {
		case event := <-events:
			drained = append(drained, event)
		default:
			return drained
		}
	}
}
