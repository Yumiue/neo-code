package tools

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type builtinSummarizerRegistration struct {
	toolName   string
	summarizer ContentSummarizer
}

var builtinSummarizers = []builtinSummarizerRegistration{
	{toolName: ToolNameBash, summarizer: bashSummarizer},
	{toolName: ToolNameFilesystemReadFile, summarizer: readFileSummarizer},
	{toolName: ToolNameFilesystemWriteFile, summarizer: writeFileSummarizer},
	{toolName: ToolNameFilesystemEdit, summarizer: editSummarizer},
	{toolName: ToolNameFilesystemGrep, summarizer: grepSummarizer},
	{toolName: ToolNameFilesystemGlob, summarizer: globSummarizer},
	{toolName: ToolNameWebFetch, summarizer: webfetchSummarizer},
}

// RegisterBuiltinSummarizers 将所有内置工具的内容摘要器注册到 Registry。
// 应在所有工具注册完成后调用一次。
func RegisterBuiltinSummarizers(registry *Registry) {
	if registry == nil {
		return
	}
	for _, item := range builtinSummarizers {
		registry.RegisterSummarizer(item.toolName, item.summarizer)
	}
}

const summaryMaxRunes = 200

// bashSummarizer 仅保留结构化执行元信息，避免把原始输出内容重新注入上下文。
func bashSummarizer(content string, metadata map[string]string, isError bool) string {
	var parts []string

	if isError {
		parts = append(parts, "[exit=non-zero]")
	} else {
		parts = append(parts, "[exit=0]")
	}

	if workdir := metadata["workdir"]; workdir != "" {
		parts = append(parts, "workdir="+workdir)
	}

	trimmed := strings.TrimSpace(content)
	if trimmed != "" {
		parts = appendTextStats(parts, trimmed)
	}

	return truncateRunes(strings.Join(parts, " "), summaryMaxRunes)
}

// readFileSummarizer 仅保留稳定元信息，避免在摘要中再次暴露文件正文。
func readFileSummarizer(content string, metadata map[string]string, isError bool) string {
	path := metadata["path"]
	if path == "" {
		return ""
	}

	trimmed := strings.TrimRight(content, "\n")
	lineCount := stableLineCount(trimmed)

	var parts []string
	parts = append(parts, "[summary]", path, "lines="+strconv.Itoa(lineCount))
	if trimmed != "" {
		parts = append(parts, "chars="+strconv.Itoa(utf8.RuneCountInString(trimmed)))
	}

	return truncateRunes(strings.Join(parts, " "), summaryMaxRunes)
}

// writeFileSummarizer 保留文件路径与写入字节数。
func writeFileSummarizer(content string, metadata map[string]string, isError bool) string {
	path := metadata["path"]
	if path == "" {
		return ""
	}
	bytes := metadata["bytes"]
	return truncateRunes("[summary] wrote "+path+" ("+bytes+" bytes)", summaryMaxRunes)
}

// editSummarizer 保留编辑路径与替换范围。
func editSummarizer(content string, metadata map[string]string, isError bool) string {
	path := metadata["relative_path"]
	if path == "" {
		path = metadata["path"]
	}
	if path == "" {
		return ""
	}
	searchLen := metadata["search_length"]
	replaceLen := metadata["replacement_length"]
	return truncateRunes(
		"[summary] edited "+path+" (search="+searchLen+" chars, replace="+replaceLen+" chars)",
		summaryMaxRunes,
	)
}

// grepSummarizer 保留搜索根目录、匹配计数与前若干文件名。
func grepSummarizer(content string, metadata map[string]string, isError bool) string {
	var parts []string
	parts = append(parts, "[summary] grep")

	if root := metadata["root"]; root != "" {
		parts = append(parts, "root="+root)
	}

	if matchedFiles := metadata["matched_files"]; matchedFiles != "" {
		parts = append(parts, "files="+matchedFiles)
	}
	if matchedLines := metadata["matched_lines"]; matchedLines != "" {
		parts = append(parts, "lines="+matchedLines)
	}

	// 从 content 中提取前几个不重复文件名
	contentLines := strings.Split(strings.TrimSpace(content), "\n")
	fileSet := make(map[string]struct{})
	var fileNames []string
	for _, line := range contentLines {
		if len(fileSet) >= 3 {
			break
		}
		idx := strings.Index(line, ":")
		if idx > 0 {
			f := line[:idx]
			if _, ok := fileSet[f]; !ok {
				fileSet[f] = struct{}{}
				fileNames = append(fileNames, f)
			}
		}
	}
	if len(fileNames) > 0 {
		parts = append(parts, "matches="+strings.Join(fileNames, ", "))
	}

	return truncateRunes(strings.Join(parts, " "), summaryMaxRunes)
}

// globSummarizer 保留匹配计数与前若干文件名。
func globSummarizer(content string, metadata map[string]string, isError bool) string {
	count := metadata["count"]
	if count == "" {
		count = "?"
	}

	contentLines := strings.Split(strings.TrimSpace(content), "\n")
	const previewLimit = 3
	var preview []string
	for i, line := range contentLines {
		if i >= previewLimit {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			preview = append(preview, trimmed)
		}
	}

	var parts []string
	parts = append(parts, "[summary] glob", count+" files")
	if len(preview) > 0 {
		parts = append(parts, strings.Join(preview, ", "))
	}

	return truncateRunes(strings.Join(parts, " "), summaryMaxRunes)
}

// webfetchSummarizer 保留可稳定持久化的 webfetch 结果标记。
func webfetchSummarizer(content string, metadata map[string]string, isError bool) string {
	var parts []string
	parts = append(parts, "[summary] webfetch")

	if truncated := metadata["truncated"]; truncated == "true" {
		parts = append(parts, "truncated=true")
	}

	return truncateRunes(strings.Join(parts, " "), summaryMaxRunes)
}

// truncateRunes 按 rune 数量截断字符串，超出时追加 "..."。
func truncateRunes(text string, maxRunes int) string {
	if maxRunes <= 0 || text == "" {
		return text
	}
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes]) + "..."
}

// stableLineCount 统计文本行数；空文本返回 0，末尾换行不会产生额外空行计数。
func stableLineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// appendTextStats 为摘要补充文本统计字段，保持统一的结构化输出格式。
func appendTextStats(parts []string, text string) []string {
	return append(parts,
		"lines="+strconv.Itoa(stableLineCount(text)),
		"chars="+strconv.Itoa(utf8.RuneCountInString(text)),
	)
}
