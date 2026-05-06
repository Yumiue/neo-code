package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neo-code/internal/tools"
)

func TestGitCommonHelpers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	child := filepath.Join(root, "nested", "repo")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if got, err := tools.ResolveEffectiveRoot(root, " "); err != nil || got != root {
		t.Fatalf("effectiveRoot(default) = %q", got)
	}
	if got, err := tools.ResolveEffectiveRoot(root, "nested/repo"); err != nil || got != child {
		t.Fatalf("effectiveRoot(custom) = %q", got)
	}
	if _, err := tools.ResolveEffectiveRoot(root, "../escape"); err == nil {
		t.Fatal("ResolveEffectiveRoot should reject escaping workdir")
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
