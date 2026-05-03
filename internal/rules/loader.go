package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	agentsFileName        = "AGENTS.md"
	documentReadRuneLimit = 16000
	snapshotRuneLimit     = 4000
	defaultRulesDir       = ".neocode"
)

// DefaultTruncationNotice 是规则内容超出注入预算时附加的统一提示。
const DefaultTruncationNotice = "\n[truncated to fit rules budget]\n"

// Document 表示单个规则文件的已加载内容快照。
type Document struct {
	Path      string
	Content   string
	Truncated bool
}

// Snapshot 表示当前轮可见的全局与项目规则快照。
type Snapshot struct {
	GlobalAGENTS  Document
	ProjectAGENTS Document
}

// Loader 定义规则快照的最小加载能力。
type Loader interface {
	Load(ctx context.Context, projectRoot string) (Snapshot, error)
}

type fileLoader struct {
	baseDir string
}

// NewLoader 创建基于本地文件系统的规则加载器。
func NewLoader(baseDir string) Loader {
	return &fileLoader{
		baseDir: strings.TrimSpace(baseDir),
	}
}

// Load 读取项目根与全局 AGENTS.md，并返回统一快照。
func (l *fileLoader) Load(ctx context.Context, projectRoot string) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	projectDoc, err := l.loadProjectDocument(ctx, projectRoot)
	if err != nil {
		return Snapshot{}, err
	}
	globalDoc, err := l.loadGlobalDocument(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		GlobalAGENTS:  globalDoc,
		ProjectAGENTS: projectDoc,
	}
	return enforceSnapshotBudget(snapshot), nil
}

// loadProjectDocument 读取项目根下的 AGENTS.md 作为项目规则。
func (l *fileLoader) loadProjectDocument(ctx context.Context, projectRoot string) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	target := ProjectRulePath(projectRoot)
	if strings.TrimSpace(target) == "" {
		return Document{}, nil
	}
	if !filepath.IsAbs(target) {
		return Document{}, fmt.Errorf("rules: project rule path %s is not absolute", target)
	}
	return readRuleDocument(target)
}

// loadGlobalDocument 读取全局 AGENTS.md 作为跨项目默认规则。
func (l *fileLoader) loadGlobalDocument(ctx context.Context) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	target := GlobalRulePath(l.baseDir)
	if strings.TrimSpace(target) == "" {
		return Document{}, nil
	}
	return readRuleDocument(target)
}

// resolveBaseDir 解析全局规则目录，默认回落到 ~/.neocode。
func resolveBaseDir(baseDir string) string {
	trimmed := strings.TrimSpace(baseDir)
	if trimmed != "" {
		return filepath.Clean(trimmed)
	}

	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(strings.TrimSpace(home)) {
		return ""
	}
	return filepath.Join(home, defaultRulesDir)
}

// enforceSnapshotBudget 按项目规则优先级裁剪合并后的规则快照总预算。
func enforceSnapshotBudget(snapshot Snapshot) Snapshot {
	remaining := snapshotRuneLimit

	snapshot.ProjectAGENTS, remaining = truncateDocumentToBudget(snapshot.ProjectAGENTS, remaining)
	snapshot.GlobalAGENTS, remaining = truncateDocumentToBudget(snapshot.GlobalAGENTS, remaining)

	return snapshot
}

// truncateDocumentToBudget 按剩余预算裁剪单个规则文档，并尽量保持 Markdown 结构闭合。
func truncateDocumentToBudget(document Document, budget int) (Document, int) {
	content := strings.TrimSpace(document.Content)
	if content == "" {
		document.Content = ""
		return document, maxInt(budget, 0)
	}

	if budget <= 0 {
		document.Content = ""
		document.Truncated = true
		return document, 0
	}

	trimmed, truncated := truncateRuleMarkdown(content, budget)
	document.Content = trimmed
	document.Truncated = document.Truncated || truncated
	return document, maxInt(budget-runeCount(trimmed), 0)
}

// truncateRuleMarkdown 按 rune 数量裁剪规则文本，并在需要时补齐未闭合的代码块围栏。
func truncateRuleMarkdown(input string, max int) (string, bool) {
	trimmed, truncated := truncateRunes(strings.TrimSpace(input), max)
	if !truncated {
		return trimmed, false
	}

	trimmed = strings.TrimRight(trimmed, "\n")
	if strings.Count(trimmed, "```")%2 == 1 {
		if max > len([]rune("\n```")) {
			trimmed, _ = truncateRunes(trimmed, max-len([]rune("\n```")))
			trimmed = strings.TrimRight(trimmed, "\n")
		}
		trimmed += "\n```"
	}
	return trimmed, true
}

// truncateRunes 按 rune 数量裁剪文本，避免破坏 UTF-8 多字节字符。
func truncateRunes(input string, max int) (string, bool) {
	if max <= 0 {
		return "", input != ""
	}
	if runeCount(input) <= max {
		return input, false
	}

	runes := []rune(input)
	return string(runes[:max]), true
}

// runeCount 统一按 rune 数量统计文本体积。
func runeCount(input string) int {
	return utf8.RuneCountInString(input)
}

// maxInt 返回两个整数中的较大值，用于避免预算结果出现负数。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
