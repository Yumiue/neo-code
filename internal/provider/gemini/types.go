package gemini

// Request 表示 Gemini streamGenerateContent 端点请求体。
type Request struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"system_instruction,omitempty"`
	Tools             []Tool            `json:"tools,omitempty"`
	ToolConfig        *ToolConfig       `json:"tool_config,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generation_config,omitempty"`
}

// Content 表示 Gemini 消息内容单元。
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// Part 表示 Gemini 内容分片。
type Part struct {
	Text             string            `json:"text,omitempty"`
	InlineData       *InlineData       `json:"inlineData,omitempty"`
	FileData         *FileData         `json:"fileData,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

// InlineData 表示 Gemini 内联二进制数据分片。
type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FileData 表示 Gemini 远程文件数据分片。
type FileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri"`
}

// FunctionCall 表示 Gemini function call 分片。
type FunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// FunctionResponse 表示 Gemini function response 分片。
type FunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response,omitempty"`
}

// Tool 表示 Gemini 工具声明容器。
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// FunctionDeclaration 表示 Gemini 函数工具定义。
type FunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolConfig 表示 Gemini 工具调用配置。
type ToolConfig struct {
	FunctionCallingConfig FunctionCallingConfig `json:"functionCallingConfig"`
}

// FunctionCallingConfig 表示 Gemini 工具调用策略。
type FunctionCallingConfig struct {
	Mode string `json:"mode,omitempty"`
}

// GenerationConfig 表示 Gemini 生成行为配置。
type GenerationConfig struct {
	Temperature *float64 `json:"temperature,omitempty"`
}
