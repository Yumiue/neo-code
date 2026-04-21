package openaicompat

import (
	"io"
	"testing"
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
