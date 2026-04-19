package responses

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

func TestBuildRequest(t *testing.T) {
	t.Parallel()

	cfg := testCfg("https://api.example.com/v1", "gpt-4.1", "test-key")
	payload, err := BuildRequest(context.Background(), cfg, providertypes.GenerateRequest{
		SystemPrompt: "system prompt",
		Messages: []providertypes.Message{
			{
				Role: providertypes.RoleUser,
				Parts: []providertypes.ContentPart{
					providertypes.NewTextPart("look"),
					providertypes.NewRemoteImagePart("https://example.com/a.png"),
				},
			},
			{
				Role: providertypes.RoleAssistant,
				Parts: []providertypes.ContentPart{
					providertypes.NewTextPart("calling tool"),
				},
				ToolCalls: []providertypes.ToolCall{
					{ID: "call_1", Name: "filesystem_read_file", Arguments: `{"path":"README.md"}`},
				},
			},
			{
				Role:       providertypes.RoleTool,
				ToolCallID: "call_1",
				Parts:      []providertypes.ContentPart{providertypes.NewTextPart("file-content")},
			},
		},
		Tools: []providertypes.ToolSpec{
			{Name: "filesystem_read_file", Description: "read file", Schema: map[string]any{"type": "object"}},
		},
	})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	if payload.Model != "gpt-4.1" || !payload.Stream {
		t.Fatalf("unexpected payload meta: %+v", payload)
	}
	if payload.Instructions != "system prompt" {
		t.Fatalf("expected instructions, got %q", payload.Instructions)
	}
	if payload.ToolChoice != "auto" || len(payload.Tools) != 1 {
		t.Fatalf("unexpected tool payload: %+v", payload.Tools)
	}
	if len(payload.Input) != 4 {
		t.Fatalf("expected 4 input items, got %d (%+v)", len(payload.Input), payload.Input)
	}
	if payload.Input[0].Role != providertypes.RoleUser || len(payload.Input[0].Content) != 2 {
		t.Fatalf("unexpected first input item: %+v", payload.Input[0])
	}
	if payload.Input[0].Content[1].Type != "input_image" || payload.Input[0].Content[1].ImageURL != "https://example.com/a.png" {
		t.Fatalf("unexpected image conversion: %+v", payload.Input[0].Content[1])
	}
	if payload.Input[2].Type != "function_call" || payload.Input[2].CallID != "call_1" {
		t.Fatalf("unexpected function_call item: %+v", payload.Input[2])
	}
	if payload.Input[3].Type != "function_call_output" || payload.Input[3].CallID != "call_1" || payload.Input[3].Output != "file-content" {
		t.Fatalf("unexpected function_call_output item: %+v", payload.Input[3])
	}
}

func TestBuildRequestSessionAssetImage(t *testing.T) {
	t.Parallel()

	cfg := testCfg("https://api.example.com/v1", "gpt-4.1", "test-key")
	payload, err := BuildRequest(context.Background(), cfg, providertypes.GenerateRequest{
		Messages: []providertypes.Message{
			{
				Role: providertypes.RoleUser,
				Parts: []providertypes.ContentPart{
					providertypes.NewSessionAssetImagePart("asset-1", "image/png"),
				},
			},
		},
		SessionAssetReader: stubSessionAssetReader{
			assets: map[string]stubSessionAsset{
				"asset-1": {data: []byte("image-data"), mime: "image/png"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if len(payload.Input) != 1 || len(payload.Input[0].Content) != 1 {
		t.Fatalf("unexpected input payload: %+v", payload.Input)
	}
	part := payload.Input[0].Content[0]
	if part.Type != "input_image" || !strings.HasPrefix(part.ImageURL, "data:image/png;base64,") {
		t.Fatalf("unexpected session asset image conversion: %+v", part)
	}
}

func TestBuildRequestRejectsInvalidToolResultMessage(t *testing.T) {
	t.Parallel()

	cfg := testCfg("https://api.example.com/v1", "gpt-4.1", "test-key")
	_, err := BuildRequest(context.Background(), cfg, providertypes.GenerateRequest{
		Messages: []providertypes.Message{
			{
				Role:  providertypes.RoleTool,
				Parts: []providertypes.ContentPart{providertypes.NewTextPart("missing id")},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "tool_call_id") {
		t.Fatalf("expected tool_call_id error, got %v", err)
	}
}

func TestConsumeStream(t *testing.T) {
	t.Parallel()

	events := make(chan providertypes.StreamEvent, 8)
	err := ConsumeStream(context.Background(), strings.NewReader(
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello \"}\n"+
			"data: {\"type\":\"response.output_item.added\",\"output_index\":1,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"filesystem_read_file\"}}\n"+
			"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_1\",\"output_index\":1,\"delta\":\"{\\\"path\\\":\\\"README.md\\\"}\"}\n"+
			"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":5,\"output_tokens\":2,\"total_tokens\":7}}}\n"+
			"data: [DONE]\n\n",
	), events)
	if err != nil {
		t.Fatalf("ConsumeStream() error = %v", err)
	}

	drained := drainEvents(events)
	if len(drained) != 4 {
		t.Fatalf("expected 4 events, got %d (%+v)", len(drained), drained)
	}
	if drained[0].Type != providertypes.StreamEventTextDelta ||
		drained[1].Type != providertypes.StreamEventToolCallStart ||
		drained[2].Type != providertypes.StreamEventToolCallDelta ||
		drained[3].Type != providertypes.StreamEventMessageDone {
		t.Fatalf("unexpected event sequence: %+v", drained)
	}
	donePayload, doneErr := drained[3].MessageDoneValue()
	if doneErr != nil {
		t.Fatalf("MessageDoneValue() error = %v", doneErr)
	}
	if donePayload.FinishReason != "stop" || donePayload.Usage == nil || donePayload.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected done payload: %+v", donePayload)
	}
}

func TestProviderGenerate(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedAuth string
	var capturedPayload Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w,
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n"+
				"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n"+
				"data: [DONE]\n\n",
		)
	}))
	defer server.Close()

	p, err := New(testCfg(server.URL, "gpt-4.1", "test-key"), server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	events := make(chan providertypes.StreamEvent, 8)
	err = p.Generate(context.Background(), providertypes.GenerateRequest{
		Messages: []providertypes.Message{
			{Role: providertypes.RoleUser, Parts: []providertypes.ContentPart{providertypes.NewTextPart("hello")}},
		},
	}, events)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if capturedPath != "/responses" {
		t.Fatalf("unexpected request path: %q", capturedPath)
	}
	if capturedAuth != "Bearer test-key" {
		t.Fatalf("unexpected auth header: %q", capturedAuth)
	}
	if capturedPayload.Model != "gpt-4.1" || !capturedPayload.Stream {
		t.Fatalf("unexpected payload: %+v", capturedPayload)
	}

	drained := drainEvents(events)
	if len(drained) != 2 || drained[0].Type != providertypes.StreamEventTextDelta || drained[1].Type != providertypes.StreamEventMessageDone {
		t.Fatalf("unexpected events: %+v", drained)
	}
}

func TestNewValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(testCfg("", "gpt-4.1", "test-key"), &http.Client{}); err == nil || !strings.Contains(err.Error(), "base url is empty") {
		t.Fatalf("expected base url validation error, got %v", err)
	}
	if _, err := New(testCfg("https://api.example.com/v1", "gpt-4.1", ""), &http.Client{}); err == nil || !strings.Contains(err.Error(), "api key is empty") {
		t.Fatalf("expected api key validation error, got %v", err)
	}
	if _, err := New(testCfg("https://api.example.com/v1", "gpt-4.1", "test-key"), nil); err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Fatalf("expected nil client error, got %v", err)
	}
}

func testCfg(baseURL, model, apiKey string) provider.RuntimeConfig {
	return provider.RuntimeConfig{
		Driver:           provider.DriverOpenAICompat,
		BaseURL:          baseURL,
		DefaultModel:     model,
		APIKey:           apiKey,
		ChatProtocol:     provider.ChatProtocolOpenAIResponses,
		ChatEndpointPath: "/responses",
	}
}

func drainEvents(events <-chan providertypes.StreamEvent) []providertypes.StreamEvent {
	var drained []providertypes.StreamEvent
	for {
		select {
		case evt := <-events:
			drained = append(drained, evt)
		default:
			return drained
		}
	}
}

type stubSessionAsset struct {
	data []byte
	mime string
	err  error
}

type stubSessionAssetReader struct {
	assets map[string]stubSessionAsset
}

func (r stubSessionAssetReader) Open(_ context.Context, assetID string) (io.ReadCloser, string, error) {
	asset, ok := r.assets[assetID]
	if !ok {
		return nil, "", errors.New("asset not found")
	}
	if asset.err != nil {
		return nil, "", asset.err
	}
	return io.NopCloser(strings.NewReader(string(asset.data))), asset.mime, nil
}
