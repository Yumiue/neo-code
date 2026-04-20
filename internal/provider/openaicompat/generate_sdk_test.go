package openaicompat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providertypes "neo-code/internal/provider/types"
)

type closeTrackingReadCloser struct {
	reader io.Reader
	closed bool
}

func (c *closeTrackingReadCloser) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *closeTrackingReadCloser) Close() error {
	c.closed = true
	return nil
}

func TestSendSDKStreamRequestClosesBodyWhenPostReturnsHTTPError(t *testing.T) {
	t.Parallel()

	closedBody := &closeTrackingReadCloser{reader: strings.NewReader(`{"error":{"message":"upstream broken"}}`)}
	p, err := New(resolvedConfig("https://example.com", "gpt-4.1"), withTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Header:     make(http.Header),
			Body:       closedBody,
			Request:    req,
		}, nil
	})))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	parseErr := errors.New("parsed")
	gotErr := p.sendSDKStreamRequest(
		context.Background(),
		"https://example.com/chat/completions",
		map[string]string{"k": "v"},
		func(context.Context, io.Reader, chan<- providertypes.StreamEvent) error {
			t.Fatal("consumeStream should not be called on http error status")
			return nil
		},
		func(resp *http.Response) error {
			_, _ = io.ReadAll(resp.Body)
			return parseErr
		},
		make(chan providertypes.StreamEvent, 1),
	)
	if !errors.Is(gotErr, parseErr) {
		t.Fatalf("expected parse error, got %v", gotErr)
	}
	if !closedBody.closed {
		t.Fatal("expected response body to be closed in error branch")
	}
}

func TestGenerateReturnsProtocolErrorForHTML200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><body><h1>bad gateway</h1></body></html>")
	}))
	defer server.Close()

	p, err := New(resolvedConfig(server.URL, "gpt-4.1"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p.client = server.Client()

	gotErr := p.Generate(context.Background(), providertypes.GenerateRequest{
		Model: "gpt-4.1",
		Messages: []providertypes.Message{
			{Role: providertypes.RoleUser, Parts: []providertypes.ContentPart{providertypes.NewTextPart("hi")}},
		},
	}, make(chan providertypes.StreamEvent, 1))
	if gotErr == nil {
		t.Fatal("expected protocol guard error for html body")
	}
	if strings.Contains(gotErr.Error(), "missing [DONE] marker before EOF") {
		t.Fatalf("unexpected SSE EOF marker error: %v", gotErr)
	}
	if !strings.Contains(gotErr.Error(), "did not return an SSE stream") {
		t.Fatalf("expected protocol mismatch error, got %v", gotErr)
	}
}

func TestValidateStreamingResponseAllowsMissingContentTypeForNonHTMLBody(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n",
		)),
	}

	if err := validateStreamingResponse(resp); err != nil {
		t.Fatalf("validateStreamingResponse() error = %v", err)
	}
	restoredBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read restored body error = %v", err)
	}
	if !strings.Contains(string(restoredBody), "[DONE]") {
		t.Fatalf("expected body stream to remain readable, got %q", string(restoredBody))
	}
}

func TestResolveChatEndpointPathByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		mode string
		want string
	}{
		{
			name: "preserves explicit path",
			path: "/gateway/chat/completions",
			mode: "responses",
			want: "/gateway/chat/completions",
		},
		{
			name: "fills chat completions path by default mode",
			path: "",
			mode: "",
			want: "/chat/completions",
		},
		{
			name: "fills responses path for responses mode",
			path: "",
			mode: "responses",
			want: "/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveChatEndpointPathByMode(tt.path, tt.mode); got != tt.want {
				t.Fatalf("resolveChatEndpointPathByMode(%q, %q) = %q, want %q", tt.path, tt.mode, got, tt.want)
			}
		})
	}
}
