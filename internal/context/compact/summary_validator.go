package compact

import (
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/context/internalcompact"
)

var summarySections = internalcompact.SummarySections()

// compactSummaryValidator 只负责 compact 摘要的规范化与结构校验。
type compactSummaryValidator struct{}

// Validate 校验摘要结构与长度，并返回可持久化的规范化结果。
func (compactSummaryValidator) Validate(summary string, maxChars int) (string, error) {
	summary = normalizeSummary(summary)
	if maxChars > 0 {
		runes := []rune(summary)
		if len(runes) > maxChars {
			summary = normalizeSummary(string(runes[:maxChars]))
		}
	}

	if err := validateSummaryStructure(summary); err != nil {
		return "", err
	}
	return summary, nil
}

// normalizeSummary 统一 compact 摘要换行与首尾空白，便于后续结构校验。
func normalizeSummary(summary string) string {
	summary = strings.ReplaceAll(summary, "\r\n", "\n")
	return strings.TrimSpace(summary)
}

// validateSummaryStructure 校验摘要是否满足共享协议要求的固定结构。
func validateSummaryStructure(summary string) error {
	summary = normalizeSummary(summary)
	if summary == "" {
		return errors.New("compact: summary is empty")
	}

	lines := strings.Split(summary, "\n")
	index := nextNonEmptyLine(lines, 0)
	if index >= len(lines) || strings.TrimSpace(lines[index]) != internalcompact.SummaryMarker {
		return fmt.Errorf("compact: summary must start with %s", internalcompact.SummaryMarker)
	}
	index++

	for _, section := range summarySections {
		index = nextNonEmptyLine(lines, index)
		if index >= len(lines) || strings.TrimSpace(lines[index]) != section+":" {
			return fmt.Errorf("compact: summary missing required section %q", section)
		}
		index++

		bullets := 0
		for index < len(lines) {
			rawLine := strings.TrimRight(lines[index], " \t")
			line := strings.TrimSpace(rawLine)
			bulletLine := strings.TrimLeft(rawLine, " \t")
			switch {
			case line == "":
				index++
			case isSummarySectionHeader(line):
				if bullets == 0 {
					return fmt.Errorf("compact: summary section %q requires at least one bullet", section)
				}
				goto nextSection
			case bulletLine == "-":
				return fmt.Errorf("compact: summary section %q contains an empty bullet", section)
			case !strings.HasPrefix(bulletLine, "- "):
				return fmt.Errorf("compact: summary section %q contains invalid line %q", section, line)
			case strings.TrimSpace(strings.TrimPrefix(bulletLine, "- ")) == "":
				return fmt.Errorf("compact: summary section %q contains an empty bullet", section)
			default:
				bullets++
				index++
			}
		}
		if bullets == 0 {
			return fmt.Errorf("compact: summary section %q requires at least one bullet", section)
		}

	nextSection:
	}

	index = nextNonEmptyLine(lines, index)
	if index < len(lines) {
		return fmt.Errorf("compact: summary contains unexpected trailing content %q", strings.TrimSpace(lines[index]))
	}
	return nil
}

// nextNonEmptyLine 返回从给定位置开始的下一条非空白行下标。
func nextNonEmptyLine(lines []string, start int) int {
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return start
}

// isSummarySectionHeader 判断当前行是否为 compact 摘要协议中的 section 头。
func isSummarySectionHeader(line string) bool {
	for _, section := range summarySections {
		if line == section+":" {
			return true
		}
	}
	return false
}
