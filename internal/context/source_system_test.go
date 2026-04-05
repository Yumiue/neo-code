package context

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCollectSystemStateHandlesGitUnavailable(t *testing.T) {
	t.Parallel()

	state, err := collectSystemState(context.Background(), testMetadata("/workspace"), func(ctx context.Context, workdir string, args ...string) (string, error) {
		return "", errors.New("git unavailable")
	})
	if err != nil {
		t.Fatalf("collectSystemState() error = %v", err)
	}

	if state.Git.Available {
		t.Fatalf("expected git to be unavailable")
	}

	section := renderPromptSection(renderSystemStateSection(state))
	if !strings.Contains(section, "- git: unavailable") {
		t.Fatalf("expected unavailable git section, got %q", section)
	}
}

func TestCollectSystemStateIncludesGitSummaryFromSingleCall(t *testing.T) {
	t.Parallel()

	callCount := 0
	runner := func(ctx context.Context, workdir string, args ...string) (string, error) {
		callCount++
		if strings.Join(args, " ") != "status --short --branch" {
			return "", errors.New("unexpected git command")
		}
		return "## feature/context...origin/feature/context\n M internal/context/builder.go\n", nil
	}

	state, err := collectSystemState(context.Background(), testMetadata("/workspace"), runner)
	if err != nil {
		t.Fatalf("collectSystemState() error = %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected a single git call, got %d", callCount)
	}
	if !state.Git.Available {
		t.Fatalf("expected git to be available")
	}
	if state.Git.Branch != "feature/context" {
		t.Fatalf("expected branch to be trimmed, got %q", state.Git.Branch)
	}
	if !state.Git.Dirty {
		t.Fatalf("expected dirty git state")
	}

	section := renderPromptSection(renderSystemStateSection(state))
	if !strings.Contains(section, "branch=`feature/context`") {
		t.Fatalf("expected branch in system section, got %q", section)
	}
	if !strings.Contains(section, "dirty=`dirty`") {
		t.Fatalf("expected dirty marker in system section, got %q", section)
	}
	if strings.Contains(section, "internal/context/builder.go") {
		t.Fatalf("did not expect full git status output in system section, got %q", section)
	}
}

func TestCollectSystemStateReturnsContextError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := collectSystemState(ctx, testMetadata("/workspace"), func(ctx context.Context, workdir string, args ...string) (string, error) {
		return "", ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}
