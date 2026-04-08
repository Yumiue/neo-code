package gateway

import (
	"context"
	"time"
)

// RuntimeEventType 表示运行事件类型。
type RuntimeEventType string

const (
	// RuntimeEventTypeRunProgress 表示运行过程事件。
	RuntimeEventTypeRunProgress RuntimeEventType = "run_progress"
	// RuntimeEventTypeRunDone 表示运行完成事件。
	RuntimeEventTypeRunDone RuntimeEventType = "run_done"
	// RuntimeEventTypeRunError 表示运行错误事件。
	RuntimeEventTypeRunError RuntimeEventType = "run_error"
)

// RunInput 表示网关向下游运行端口发起 run 动作时的输入。
type RunInput struct {
	// RequestID 是客户端请求标识。
	RequestID string
	// SessionID 是会话标识。
	SessionID string
	// RunID 是运行标识。
	RunID string
	// InputText 是文本输入。
	InputText string
	// InputParts 是多模态输入分片。
	InputParts []InputPart
	// Workdir 是请求级工作目录覆盖值。
	Workdir string
}

// CompactInput 表示网关向下游运行端口发起 compact 动作时的输入。
type CompactInput struct {
	// RequestID 是客户端请求标识。
	RequestID string
	// SessionID 是会话标识。
	SessionID string
	// RunID 是运行标识。
	RunID string
}

// CompactResult 表示 compact 动作完成后返回的结果。
type CompactResult struct {
	// Applied 表示是否实际应用压缩结果。
	Applied bool
	// BeforeChars 是压缩前字符数。
	BeforeChars int
	// AfterChars 是压缩后字符数。
	AfterChars int
	// SavedRatio 是压缩节省比例。
	SavedRatio float64
	// TriggerMode 是触发模式标识。
	TriggerMode string
	// TranscriptID 是压缩产物标识。
	TranscriptID string
	// TranscriptPath 是压缩产物路径。
	TranscriptPath string
}

// RuntimeEvent 表示运行端口推送给网关的统一事件。
type RuntimeEvent struct {
	// Type 是事件类型。
	Type RuntimeEventType `json:"type"`
	// RunID 是运行标识。
	RunID string `json:"run_id,omitempty"`
	// SessionID 是会话标识。
	SessionID string `json:"session_id,omitempty"`
	// Payload 是事件扩展负载。
	Payload any `json:"payload,omitempty"`
}

// SessionMessage 表示会话消息快照中的单条消息。
type SessionMessage struct {
	// Role 是消息角色。
	Role string `json:"role"`
	// Content 是消息内容。
	Content string `json:"content"`
	// ToolCallID 是工具消息关联的调用标识。
	ToolCallID string `json:"tool_call_id,omitempty"`
	// IsError 表示该消息是否为错误结果。
	IsError bool `json:"is_error,omitempty"`
}

// Session 表示网关视角的会话详情。
type Session struct {
	// ID 是会话标识。
	ID string `json:"id"`
	// Title 是会话标题。
	Title string `json:"title"`
	// CreatedAt 是会话创建时间。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 是会话更新时间。
	UpdatedAt time.Time `json:"updated_at"`
	// Workdir 是会话工作目录。
	Workdir string `json:"workdir,omitempty"`
	// Messages 是会话消息快照。
	Messages []SessionMessage `json:"messages,omitempty"`
}

// SessionSummary 表示会话列表项摘要。
type SessionSummary struct {
	// ID 是会话标识。
	ID string `json:"id"`
	// Title 是会话标题。
	Title string `json:"title"`
	// CreatedAt 是会话创建时间。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 是会话更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// RuntimePort 定义网关访问运行时编排的下游端口契约。
type RuntimePort interface {
	// Run 启动一次运行编排。
	Run(ctx context.Context, input RunInput) error
	// Compact 对指定会话触发一次手动压缩。
	Compact(ctx context.Context, input CompactInput) (CompactResult, error)
	// CancelActiveRun 取消当前活跃运行。
	CancelActiveRun() bool
	// Events 返回统一运行事件流。
	Events() <-chan RuntimeEvent
	// ListSessions 返回会话摘要列表。
	ListSessions(ctx context.Context) ([]SessionSummary, error)
	// LoadSession 加载指定会话详情。
	LoadSession(ctx context.Context, id string) (Session, error)
	// SetSessionWorkdir 更新会话工作目录。
	SetSessionWorkdir(ctx context.Context, sessionID string, workdir string) (Session, error)
}

// Gateway 定义网关主契约。
type Gateway interface {
	// Serve 启动网关服务并绑定运行端口。
	Serve(ctx context.Context, runtimePort RuntimePort) error
	// Close 优雅关闭网关服务。
	Close(ctx context.Context) error
}
