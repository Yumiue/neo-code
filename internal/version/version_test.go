package version

import "testing"

func TestCurrentFallsBackToDev(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "   "
	if got := Current(); got != "dev" {
		t.Fatalf("Current() = %q, want %q", got, "dev")
	}

	Version = " v1.2.3 "
	if got := Current(); got != "v1.2.3" {
		t.Fatalf("Current() = %q, want %q", got, "v1.2.3")
	}
}

func TestIsSemverRelease(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		matched bool
	}{
		{name: "with v prefix", value: "v1.2.3", matched: true},
		{name: "without v prefix", value: "1.2.3", matched: true},
		{name: "prerelease", value: "v1.2.3-rc.1", matched: true},
		{name: "build metadata", value: "v1.2.3+meta", matched: true},
		{name: "dev", value: "dev", matched: false},
		{name: "missing patch", value: "v1.2", matched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSemverRelease(tt.value); got != tt.matched {
				t.Fatalf("IsSemverRelease(%q) = %v, want %v", tt.value, got, tt.matched)
			}
		})
	}
}
