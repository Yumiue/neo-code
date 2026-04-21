package openaicompat

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"

	"neo-code/internal/provider"
)

const (
	maxErrorBodySize = 64 * 1024
)

var htmlTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ParseError 解析 OpenAI-compatible HTTP 错误响应，并在读取阶段限制响应体大小。
func ParseError(resp *http.Response) error {
	data, truncated, readErr := readBoundedBody(resp.Body, maxErrorBodySize)
	if readErr != nil {
		return provider.NewProviderErrorFromStatus(
			resp.StatusCode,
			fmt.Sprintf("%sread error response: %v", errorPrefix, readErr),
		)
	}

	var parsed errorResponse
	if err := json.Unmarshal(data, &parsed); err == nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return provider.NewProviderErrorFromStatus(resp.StatusCode, parsed.Error.Message)
	}

	summary := summarizeErrorBody(data, truncated)
	if summary == "" {
		return provider.NewProviderErrorFromStatus(resp.StatusCode, resp.Status)
	}

	if isHTMLResponse(resp.Header.Get("Content-Type"), data) {
		summary = sanitizeHTMLSummary(data, truncated)
		message := "upstream returned a non-JSON HTML error page"
		if summary != "" {
			message += ": " + summary
		}
		return provider.NewProviderErrorFromStatus(resp.StatusCode, message)
	}

	return provider.NewProviderErrorFromStatus(resp.StatusCode, summary)
}

// readBoundedBody 读取受限响应体，超过上限时返回截断标记。
func readBoundedBody(body io.Reader, limit int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) <= limit {
		return data, false, nil
	}
	return data[:limit], true, nil
}

// summarizeErrorBody 清洗并脱敏错误响应内容，避免原始响应直接泄漏到上层。
func summarizeErrorBody(payload []byte, truncated bool) string {
	if len(payload) == 0 {
		return ""
	}

	summary := sanitizePrintableText(payload)
	summary = redactSensitiveSummary(summary)
	if summary == "" {
		return ""
	}
	if truncated {
		return summary + " ...(truncated)"
	}
	return summary
}

// isHTMLResponse 判断响应头或响应体是否表现为 HTML 错误页。
func isHTMLResponse(contentType string, payload []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(contentType))
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return true
	}
	return looksLikeHTMLPayload(payload)
}

// sanitizeHTMLSummary 将 HTML 片段转换为可读摘要，避免标签污染错误信息。
func sanitizeHTMLSummary(payload []byte, truncated bool) string {
	summary := summarizeErrorBody(payload, truncated)
	summary = strings.TrimSpace(htmlTagPattern.ReplaceAllString(summary, " "))
	return summary
}

// looksLikeHTMLPayload 判断响应片段是否明显是 HTML 页面内容。
func looksLikeHTMLPayload(payload []byte) bool {
	trimmed := strings.TrimSpace(strings.ToLower(string(payload)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}
