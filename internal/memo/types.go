package memo

import (
	"context"
	"time"

	providertypes "neo-code/internal/provider/types"
)

// Type 定义记忆条目的闭合分类。
type Type string

const (
	// TypeUser 表示用户画像、偏好和背景信息。
	TypeUser Type = "user"
	// TypeFeedback 表示用户对 Agent 行为的纠正与指导。
	TypeFeedback Type = "feedback"
	// TypeProject 表示项目事实、决策和进行中的工作。
	TypeProject Type = "project"
	// TypeReference 表示外部资源和信息入口。
	TypeReference Type = "reference"
)

const (
	// SourceAutoExtract 表示记忆由自动提取器生成。
	SourceAutoExtract = "extractor_auto"
	// SourceUserManual 表示记忆由用户手动创建。
	SourceUserManual = "user_manual"
	// SourceToolInitiated 表示记忆由模型主动调用工具创建。
	SourceToolInitiated = "tool_initiated"
)

// Entry 表示单个持久化记忆条目。
type Entry struct {
	ID        string
	Type      Type
	Title     string
	Content   string
	Keywords  []string
	Source    string
	TopicFile string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Index 表示 MEMO.md 索引文件的内存模型。
type Index struct {
	Entries   []Entry
	UpdatedAt time.Time
}

// Store 定义记忆持久化的最小抽象。
type Store interface {
	LoadIndex(ctx context.Context) (*Index, error)
	SaveIndex(ctx context.Context, index *Index) error
	LoadTopic(ctx context.Context, filename string) (string, error)
	SaveTopic(ctx context.Context, filename string, content string) error
	DeleteTopic(ctx context.Context, filename string) error
	ListTopics(ctx context.Context) ([]string, error)
}

// Extractor 定义从对话消息中提取记忆的最小能力。
type Extractor interface {
	Extract(ctx context.Context, messages []providertypes.Message) ([]Entry, error)
}

// TextGenerator 定义调用 LLM 生成文本的最小能力，用于记忆提取。
// 该接口隔离 provider 细节，避免 memo 包直接依赖 provider 基础设施。
type TextGenerator interface {
	Generate(ctx context.Context, prompt string, messages []providertypes.Message) (string, error)
}

// ValidTypes 返回所有合法的记忆类型。
func ValidTypes() []Type {
	return []Type{TypeUser, TypeFeedback, TypeProject, TypeReference}
}

// IsValidType 检查给定类型是否合法。
func IsValidType(t Type) bool {
	switch t {
	case TypeUser, TypeFeedback, TypeProject, TypeReference:
		return true
	default:
		return false
	}
}

// ParseType 将字符串解析为 Type，不合法时返回 false。
func ParseType(s string) (Type, bool) {
	t := Type(s)
	return t, IsValidType(t)
}
