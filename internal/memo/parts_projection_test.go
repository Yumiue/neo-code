package memo

import (
	"testing"

	providertypes "neo-code/internal/provider/types"
)

func TestRenderMemoPartsUsesImagePlaceholder(t *testing.T) {
	t.Parallel()

	parts := []providertypes.ContentPart{
		providertypes.NewTextPart("note:"),
		providertypes.NewRemoteImagePart("https://example.com/img.png"),
	}
	if got := renderMemoParts(parts); got != "note:[Image]" {
		t.Fatalf("renderMemoParts() = %q, want %q", got, "note:[Image]")
	}
}

func TestHasMemoRelevantUserInputTreatsImageAsInput(t *testing.T) {
	t.Parallel()

	if hasMemoRelevantUserInput([]providertypes.ContentPart{providertypes.NewTextPart("  ")}) {
		t.Fatalf("blank text should not be treated as meaningful input")
	}
	if !hasMemoRelevantUserInput([]providertypes.ContentPart{providertypes.NewRemoteImagePart("https://example.com/img.png")}) {
		t.Fatalf("image should be treated as meaningful user input")
	}
}
