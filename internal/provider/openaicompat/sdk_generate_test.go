package openaicompat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

func TestGenerateSDKAndHTTPParity(t *testing.T) {
	testCases := []struct {
		name         string
		chatProtocol string
		endpointPath string
		streamBody   string
	}{
		{
			name:         "chat_completions",
			chatProtocol: provider.ChatProtocolOpenAIChatCompletions,
			endpointPath: "/chat/completions",
			streamBody: "data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"filesystem_read_file\"}}]}}]}\n" +
				"data: {\"choices\":[{\"finish_reason\":\"stop\",\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"arguments\":\"{\\\"path\\\":\\\"README.md\\\"}\"}}]}}]}\n" +
				"data: [DONE]\n\n",
		},
		{
			name:         "responses",
			chatProtocol: provider.ChatProtocolOpenAIResponses,
			endpointPath: "/responses",
			streamBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello \"}\n" +
				"data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"filesystem_read_file\"}}\n" +
				"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_1\",\"output_index\":0,\"delta\":\"{\\\"path\\\":\\\"README.md\\\"}\"}\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":5,\"output_tokens\":2,\"total_tokens\":7}}}\n" +
				"data: [DONE]\n\n",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.endpointPath {
					t.Fatalf("expected path %q, got %q", tt.endpointPath, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("expected authorization header, got %q", got)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprint(w, tt.streamBody)
			}))
			defer server.Close()

			cfg := resolvedConfig(server.URL, "gpt-4.1")
			cfg.ChatProtocol = tt.chatProtocol
			cfg.ChatEndpointPath = tt.endpointPath

			httpEvents := runGenerateAndDrainEvents(t, cfg, executionModeHTTP)
			sdkEvents := runGenerateAndDrainEvents(t, cfg, executionModeSDK)

			projectedHTTP := projectStreamEvents(httpEvents)
			projectedSDK := projectStreamEvents(sdkEvents)
			if len(projectedHTTP) != len(projectedSDK) {
				t.Fatalf("event length mismatch, http=%v sdk=%v", projectedHTTP, projectedSDK)
			}
			for i := range projectedHTTP {
				if projectedHTTP[i] != projectedSDK[i] {
					t.Fatalf("event mismatch at index %d, http=%q sdk=%q", i, projectedHTTP[i], projectedSDK[i])
				}
			}
		})
	}
}

func runGenerateAndDrainEvents(t *testing.T, cfg provider.RuntimeConfig, mode string) []providertypes.StreamEvent {
	t.Helper()

	p, err := New(cfg, withExecutionMode(mode))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	events := make(chan providertypes.StreamEvent, 32)
	err = p.Generate(context.Background(), providertypes.GenerateRequest{
		Messages: []providertypes.Message{
			{Role: providertypes.RoleUser, Parts: []providertypes.ContentPart{providertypes.NewTextPart("hello")}},
		},
		Tools: []providertypes.ToolSpec{
			{
				Name:        "filesystem_read_file",
				Description: "read file",
				Schema:      map[string]any{"type": "object"},
			},
		},
	}, events)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return drainStreamEvents(events)
}

func projectStreamEvents(events []providertypes.StreamEvent) []string {
	projected := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Type {
		case providertypes.StreamEventTextDelta:
			payload, _ := event.TextDeltaValue()
			projected = append(projected, "text:"+payload.Text)
		case providertypes.StreamEventToolCallStart:
			payload, _ := event.ToolCallStartValue()
			projected = append(projected, fmt.Sprintf("tool_start:%d:%s:%s", payload.Index, payload.ID, payload.Name))
		case providertypes.StreamEventToolCallDelta:
			payload, _ := event.ToolCallDeltaValue()
			projected = append(projected, fmt.Sprintf("tool_delta:%d:%s:%s", payload.Index, payload.ID, payload.ArgumentsDelta))
		case providertypes.StreamEventMessageDone:
			payload, _ := event.MessageDoneValue()
			total := 0
			if payload.Usage != nil {
				total = payload.Usage.TotalTokens
			}
			projected = append(projected, fmt.Sprintf("done:%s:%d", payload.FinishReason, total))
		default:
			projected = append(projected, string(event.Type))
		}
	}
	return projected
}
