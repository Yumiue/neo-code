package wire

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseErrorTruncatesLargeBody(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		Status:     "502 Bad Gateway",
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 70*1024))),
	}
	err := ParseError(resp)
	if err == nil {
		t.Fatal("expected parse error result")
	}
	if !strings.Contains(err.Error(), "...(truncated)") {
		t.Fatalf("expected truncated marker, got %v", err)
	}
}
