package wire

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"neo-code/internal/provider"
)

const (
	errorPrefix      = "anthropic provider: "
	maxErrorBodySize = 64 * 1024
)

// ParseError 解析 Anthropic HTTP 错误响应，并限制错误体最大读取长度。
func ParseError(resp *http.Response) error {
	data, truncated, readErr := readBoundedBody(resp.Body, maxErrorBodySize)
	if readErr != nil {
		return provider.NewProviderErrorFromStatus(
			resp.StatusCode,
			fmt.Sprintf("%sread error response: %v", errorPrefix, readErr),
		)
	}

	var parsed ErrorResponse
	if err := json.Unmarshal(data, &parsed); err == nil && parsed.Error != nil {
		if message := strings.TrimSpace(parsed.Error.Message); message != "" {
			return provider.NewProviderErrorFromStatus(resp.StatusCode, message)
		}
	}

	bodyText := strings.TrimSpace(string(data))
	if bodyText == "" {
		return provider.NewProviderErrorFromStatus(resp.StatusCode, resp.Status)
	}
	if truncated {
		bodyText += " ...(truncated)"
	}
	return provider.NewProviderErrorFromStatus(resp.StatusCode, bodyText)
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
