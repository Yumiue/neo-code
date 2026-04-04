package compact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"neo-code/internal/provider"
)

func TestTranscriptStoreSaveSanitizesSessionIDAndWritesJSONL(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	store := transcriptStore{
		now:         func() time.Time { return time.Unix(1712052000, 123456789) },
		randomToken: func() (string, error) { return "token1234", nil },
		userHomeDir: func() (string, error) { return home, nil },
		mkdirAll:    os.MkdirAll,
		writeFile:   os.WriteFile,
		rename:      os.Rename,
		remove:      os.Remove,
	}

	id, path, err := store.Save([]provider.Message{
		{Role: provider.RoleUser, Content: "hello"},
	}, "session with spaces", filepath.Join(home, "workspace"))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if id == "" || path == "" {
		t.Fatalf("expected transcript metadata, got id=%q path=%q", id, path)
	}
	if filepath.Ext(path) != transcriptFileExtension {
		t.Fatalf("expected transcript extension %q, got %q", transcriptFileExtension, path)
	}
	if !strings.Contains(filepath.Base(path), "session_with_spaces") {
		t.Fatalf("expected sanitized session id in path, got %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected transcript content")
	}
}

func TestTranscriptFileModeForOS(t *testing.T) {
	t.Parallel()

	if got := transcriptFileModeForOS("windows"); got != 0o644 {
		t.Fatalf("expected windows mode 0644, got %#o", got)
	}
	if got := transcriptFileModeForOS("linux"); got != 0o600 {
		t.Fatalf("expected non-windows mode 0600, got %#o", got)
	}
}
