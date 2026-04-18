package discovery

import "testing"

func TestResolveEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		baseURL      string
		endpointPath string
		want         string
	}{
		{
			name:         "joins base and relative path",
			baseURL:      "https://api.example.com/v1/",
			endpointPath: "models",
			want:         "https://api.example.com/v1/models",
		},
		{
			name:         "keeps leading slash path",
			baseURL:      "https://api.example.com/v1",
			endpointPath: "/models",
			want:         "https://api.example.com/v1/models",
		},
		{
			name:         "empty endpoint returns base",
			baseURL:      "https://api.example.com/v1/",
			endpointPath: "   ",
			want:         "https://api.example.com/v1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveEndpoint(tt.baseURL, tt.endpointPath); got != tt.want {
				t.Fatalf("ResolveEndpoint(%q, %q) = %q, want %q", tt.baseURL, tt.endpointPath, got, tt.want)
			}
		})
	}
}
