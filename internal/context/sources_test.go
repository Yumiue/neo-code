package context

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neo-code/internal/promptasset"
	"neo-code/internal/rules"
)

func TestCorePromptSourceSectionsReturnsClone(t *testing.T) {
	t.Parallel()

	source := corePromptSource{}
	first, err := source.Sections(context.Background(), BuildInput{})
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if len(first) == 0 {
		t.Fatalf("expected non-empty core prompt sections")
	}

	first[0].Title = "changed"

	second, err := source.Sections(context.Background(), BuildInput{})
	if err != nil {
		t.Fatalf("Sections() second call error = %v", err)
	}
	if second[0].Title != promptasset.CoreSections()[0].Title {
		t.Fatalf("expected cloned sections, got %+v", second)
	}
}

func TestRulesPromptSourceSectionsSkipsWhenNoRulesExist(t *testing.T) {
	t.Parallel()

	baseDir := filepath.Join(t.TempDir(), ".neocode")
	sections, err := newRulesPromptSource(rules.NewLoader(baseDir)).Sections(context.Background(), BuildInput{
		Metadata: Metadata{ProjectRoot: t.TempDir(), Workdir: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if len(sections) != 0 {
		t.Fatalf("expected no rules sections, got %+v", sections)
	}
}

func TestRulesPromptSourceSectionsRendersRules(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), ".neocode")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir baseDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, projectRuleFileName), []byte("rule-body"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	sections, err := newRulesPromptSource(rules.NewLoader(baseDir)).Sections(context.Background(), BuildInput{
		Metadata: Metadata{ProjectRoot: root, Workdir: root},
	})
	if err != nil {
		t.Fatalf("Sections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected one rules section, got %+v", sections)
	}
	if got := renderPromptSection(sections[0]); got == "" {
		t.Fatalf("expected rendered rules section")
	}
	if got := renderPromptSection(sections[0]); !strings.Contains(got, "### Project Rules") {
		t.Fatalf("expected project rules block, got %q", got)
	}
}

func TestRulesPromptSourceSectionsReflectRuleFileUpdatesOnNextBuild(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), ".neocode")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir baseDir: %v", err)
	}
	if _, err := rules.WriteGlobalRule(context.Background(), baseDir, "global-v1"); err != nil {
		t.Fatalf("WriteGlobalRule(v1) error = %v", err)
	}

	source := newRulesPromptSource(rules.NewLoader(baseDir))
	buildInput := BuildInput{Metadata: Metadata{ProjectRoot: root, Workdir: root}}

	firstSections, err := source.Sections(context.Background(), buildInput)
	if err != nil {
		t.Fatalf("Sections(first) error = %v", err)
	}
	firstPrompt := renderPromptSection(firstSections[0])
	if !strings.Contains(firstPrompt, "global-v1") {
		t.Fatalf("expected first prompt to include global-v1, got %q", firstPrompt)
	}

	if _, err := rules.WriteGlobalRule(context.Background(), baseDir, "global-v2"); err != nil {
		t.Fatalf("WriteGlobalRule(v2) error = %v", err)
	}

	secondSections, err := source.Sections(context.Background(), buildInput)
	if err != nil {
		t.Fatalf("Sections(second) error = %v", err)
	}
	secondPrompt := renderPromptSection(secondSections[0])
	if !strings.Contains(secondPrompt, "global-v2") || strings.Contains(secondPrompt, "global-v1") {
		t.Fatalf("expected second prompt to reflect latest global rule, got %q", secondPrompt)
	}
}

func TestCorePromptSourceSectionsHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (corePromptSource{}).Sections(ctx, BuildInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
