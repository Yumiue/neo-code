package openaicompat

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"neo-code/internal/provider"
)

func TestParseErrorHTMLBodyReturnsSanitizedSummary(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Status:     "502 Bad Gateway",
		Header: http.Header{
			"Content-Type": []string{"text/html; charset=utf-8"},
		},
		Body: ioNopCloser(`<!doctype html><html><body><h1>Gateway error</h1><p>api_key=sk-secret-abcdef12</p></body></html>`),
	}

	err := ParseError(resp)
	if err == nil {
		t.Fatal("expected provider error")
	}
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected *provider.ProviderError, got %T: %v", err, err)
	}
	if !strings.Contains(providerErr.Message, "non-JSON HTML error page") {
		t.Fatalf("expected html classification message, got %q", providerErr.Message)
	}
	if strings.Contains(strings.ToLower(providerErr.Message), "<html") {
		t.Fatalf("expected html tags to be stripped from message, got %q", providerErr.Message)
	}
	if strings.Contains(providerErr.Message, "sk-secret-abcdef12") {
		t.Fatalf("expected sensitive values to be redacted, got %q", providerErr.Message)
	}
}
