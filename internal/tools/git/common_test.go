package git

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCommonHelpers(t *testing.T) {
	t.Parallel()

	if got := effectiveRoot("/workspace/root", " "); got != "/workspace/root" {
		t.Fatalf("effectiveRoot(default) = %q", got)
	}
	if got := effectiveRoot("/workspace/root", "/tmp/repo"); got != "/tmp/repo" {
		t.Fatalf("effectiveRoot(custom) = %q", got)
	}

	formatted := formatFileList([]fileEntry{
		{Status: "modified", Path: filepath.Join("pkg", "a.go")},
		{Status: "renamed", Path: filepath.Join("pkg", "new.go"), OldPath: filepath.Join("pkg", "old.go"), Snippet: "line1\nline2"},
	}, 3, true)
	for _, want := range []string{
		"returned_count: 2",
		"total_count: 3",
		"truncated: true",
		"- status: modified",
		"path: pkg/a.go",
		"old_path: pkg/old.go",
		"snippet: |",
		"    line1",
		"    line2",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatFileList() missing %q in %q", want, formatted)
		}
	}

	if got := itoa(0); got != "0" {
		t.Fatalf("itoa(0) = %q", got)
	}
	if got := itoa(-42); got != "-42" {
		t.Fatalf("itoa(-42) = %q", got)
	}
	if got := boolToString(true); got != "true" {
		t.Fatalf("boolToString(true) = %q", got)
	}
	if got := boolToString(false); got != "false" {
		t.Fatalf("boolToString(false) = %q", got)
	}
}
