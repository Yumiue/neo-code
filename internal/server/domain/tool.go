package domain

import "encoding/json"

// ToolCall 表示工具调用请求。
type ToolCall struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

type Tool interface {
	Definition() ToolDefinition
	Run(params map[string]interface{}) *ToolResult
}

// ToolResult 表示执行工具的结果。
type ToolResult struct {
	ToolName string                 `json:"tool"`
	Success  bool                   `json:"success"`
	Output   string                 `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolDefinition 描述工具的定义，包括名称、描述和参数规范。
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  []ToolParamSpec `json:"parameters"`
}

// ToolParamSpec 描述工具的单个参数规范。
type ToolParamSpec struct {
	Name        string `json:"name"`        // 参数名称
	Type        string `json:"type"`        // 参数类型（string, integer, boolean 等）
	Required    bool   `json:"required"`    // 是否必需
	Description string `json:"description"` // 参数描述
}

func (tr *ToolResult) MarshalJSON() ([]byte, error) {
	type Alias ToolResult
	return json.Marshal(&struct {
		*Alias
		Output string `json:"output,omitempty"`
		Error  string `json:"error,omitempty"`
	}{
		Alias:  (*Alias)(tr),
		Output: tr.Output,
		Error:  tr.Error,
	})
}
