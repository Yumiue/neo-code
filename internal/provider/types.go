package provider

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	IsError    bool       `json:"is_error,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type ChatRequest struct {
	Model        string     `json:"model"`
	SystemPrompt string     `json:"system_prompt"`
	Messages     []Message  `json:"messages"`
	Tools        []ToolSpec `json:"tools,omitempty"`
}

type ChatResponse struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Usage        Usage   `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type ModelDescriptor struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	ContextWindow   int             `json:"context_window,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
}

type ProviderCatalogItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Models      []ModelDescriptor `json:"models,omitempty"`
}

type ProviderSelection struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

type StreamEventType string

const (
	// StreamEventTextDelta 表示模型输出的文本片段。
	StreamEventTextDelta StreamEventType = "text_delta"
	// StreamEventToolCallStart 表示模型开始请求工具调用，TUI 可据此展示过渡提示。
	StreamEventToolCallStart StreamEventType = "tool_call_start"
	// StreamEventToolCallDelta 表示工具调用参数的增量片段。
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	// StreamEventMessageDone 表示本轮消息完成，包含最终统计信息。
	StreamEventMessageDone StreamEventType = "message_done"
)

// StreamEvent 表示 provider 驱动层向 runtime 推送的流式事件。
type StreamEvent struct {
	Type StreamEventType

	// text_delta
	Text string `json:"text,omitempty"` // 文本片段

	// tool_call_start / tool_call_delta
	ToolCallIndex      int    `json:"tool_call_index,omitempty"`      // 工具调用索引
	ToolCallID         string `json:"tool_call_id,omitempty"`         // 工具调用 ID（tool_call_start 时使用）
	ToolName           string `json:"tool_name,omitempty"`            // 工具名称（tool_call_start 时使用）
	ToolArgumentsDelta string `json:"tool_arguments_delta,omitempty"` // 参数增量片段（tool_call_delta 时使用）

	// message_done
	FinishReason string `json:"finish_reason,omitempty"` // 结束原因（仅 message_done 时有效）
	Usage        *Usage `json:"usage,omitempty"`         // 使用统计（仅 message_done 时有效）
}
