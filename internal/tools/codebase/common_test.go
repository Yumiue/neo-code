package codebase

import (
	"path/filepath"
	"testing"
)

func TestCodebaseCommonHelpers(t *testing.T) {
	t.Parallel()

	if got := effectiveRoot("/workspace/root", " "); got != "/workspace/root" {
		t.Fatalf("effectiveRoot(default) = %q", got)
	}
	if got := effectiveRoot("/workspace/root", "/tmp/repo"); got != "/tmp/repo" {
		t.Fatalf("effectiveRoot(custom) = %q", got)
	}
	if got := itoa(0); got != "0" {
		t.Fatalf("itoa(0) = %q", got)
	}
	if got := itoa(-9); got != "-9" {
		t.Fatalf("itoa(-9) = %q", got)
	}
	if got := boolToString(true); got != "true" {
		t.Fatalf("boolToString(true) = %q", got)
	}
	if got := boolToString(false); got != "false" {
		t.Fatalf("boolToString(false) = %q", got)
	}
	if got := filepathSlashClean("a/b"); got != filepath.Clean(filepath.FromSlash("a/b")) {
		t.Fatalf("filepathSlashClean() = %q", got)
	}
}
