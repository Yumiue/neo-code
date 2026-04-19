package wire

// ErrorResponse 表示 Anthropic HTTP 错误响应。
type ErrorResponse struct {
	Type  string          `json:"type,omitempty"`
	Error *AnthropicError `json:"error,omitempty"`
}

// AnthropicError 表示 Anthropic 错误对象。
type AnthropicError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

// StreamPayload 表示 Anthropic SSE 流中的单个 JSON 载荷。
type StreamPayload struct {
	Type         string              `json:"type,omitempty"`
	Index        int                 `json:"index,omitempty"`
	Error        *AnthropicError     `json:"error,omitempty"`
	ContentBlock *StreamContentBlock `json:"content_block,omitempty"`
	Delta        *StreamDelta        `json:"delta,omitempty"`
	Message      *StreamMessage      `json:"message,omitempty"`
	Usage        *StreamUsage        `json:"usage,omitempty"`
}

// StreamContentBlock 表示 content_block_start 的块结构。
type StreamContentBlock struct {
	Type  string         `json:"type,omitempty"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

// StreamDelta 表示 content_block_delta/message_delta 增量体。
type StreamDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// StreamMessage 表示 message_start 事件中的 message 元信息。
type StreamMessage struct {
	Usage *StreamUsage `json:"usage,omitempty"`
}

// StreamUsage 表示 Anthropic usage 统计。
type StreamUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}
