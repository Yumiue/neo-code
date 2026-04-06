package tui

import (
	"regexp"
	"strings"
	"testing"

	tuiinfra "neo-code/internal/tui/infra"
)

var markdownTestANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestNewMarkdownRendererAndRender(t *testing.T) {
	rendererAny, err := newMarkdownRenderer()
	if err != nil {
		t.Fatalf("newMarkdownRenderer() error = %v", err)
	}

	renderer, ok := rendererAny.(*tuiinfra.CachedMarkdownRenderer)
	if !ok {
		t.Fatalf("expected CachedMarkdownRenderer type, got %T", rendererAny)
	}

	output, err := renderer.Render("# Title\n\n- one\n- two", 40)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if output == "" {
		t.Fatalf("expected non-empty markdown output")
	}
	if renderer.RendererCount() != 1 {
		t.Fatalf("expected one cached term renderer, got %d", renderer.RendererCount())
	}
	if renderer.CacheCount() != 1 {
		t.Fatalf("expected one cached render result, got %d", renderer.CacheCount())
	}
}

func TestMarkdownRendererHandlesEmptyInputAndCacheEviction(t *testing.T) {
	rendererAny, err := newMarkdownRenderer()
	if err != nil {
		t.Fatalf("newMarkdownRenderer() error = %v", err)
	}
	renderer := rendererAny.(*tuiinfra.CachedMarkdownRenderer)

	emptyOutput, err := renderer.Render(" \n\t ", 32)
	if err != nil {
		t.Fatalf("Render(empty) error = %v", err)
	}
	if emptyOutput != emptyMessageText {
		t.Fatalf("expected empty message placeholder, got %q", emptyOutput)
	}

	renderer.SetMaxCacheEntries(1)
	if _, err := renderer.Render("first", 20); err != nil {
		t.Fatalf("Render(first) error = %v", err)
	}
	if _, err := renderer.Render("second", 20); err != nil {
		t.Fatalf("Render(second) error = %v", err)
	}
	if renderer.CacheOrderCount() != 1 || renderer.CacheCount() != 1 {
		t.Fatalf("expected cache eviction to keep one entry, got order=%d cache=%d", renderer.CacheOrderCount(), renderer.CacheCount())
	}
}

func TestMarkdownRendererCachesByWidth(t *testing.T) {
	rendererAny, err := newMarkdownRenderer()
	if err != nil {
		t.Fatalf("newMarkdownRenderer() error = %v", err)
	}
	renderer := rendererAny.(*tuiinfra.CachedMarkdownRenderer)

	text := "plain text"
	if _, err := renderer.Render(text, 20); err != nil {
		t.Fatalf("Render(width=20) error = %v", err)
	}
	if _, err := renderer.Render(text, 50); err != nil {
		t.Fatalf("Render(width=50) error = %v", err)
	}
	if renderer.RendererCount() != 2 {
		t.Fatalf("expected width-specific renderer cache, got %d", renderer.RendererCount())
	}
}

func TestMarkdownRendererPreservesContent(t *testing.T) {
	rendererAny, err := newMarkdownRenderer()
	if err != nil {
		t.Fatalf("newMarkdownRenderer() error = %v", err)
	}
	renderer := rendererAny.(*tuiinfra.CachedMarkdownRenderer)

	output, err := renderer.Render("Title\n\n- first item\n- second item", 40)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	visible := markdownTestANSIPattern.ReplaceAllString(output, "")
	if !strings.Contains(visible, "Title") || !strings.Contains(visible, "first item") || !strings.Contains(visible, "second item") {
		t.Fatalf("expected markdown content to be preserved, got %q", visible)
	}
}
