package wire

// StreamChunk 表示 Gemini SSE 流中的单个响应载荷。
type StreamChunk struct {
	Candidates []Candidate      `json:"candidates,omitempty"`
	Usage      *UsageMetadata   `json:"usageMetadata,omitempty"`
	Error      *GeminiWireError `json:"error,omitempty"`
}

// Candidate 表示 Gemini 候选输出。
type Candidate struct {
	Index        int     `json:"index"`
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
}

// Content 表示候选内容体。
type Content struct {
	Parts []Part `json:"parts,omitempty"`
}

// Part 表示 Gemini 返回的内容分片。
type Part struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *FunctionCall `json:"functionCall,omitempty"`
}

// FunctionCall 表示 Gemini 返回的函数调用分片。
type FunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

// UsageMetadata 表示 Gemini usage 统计。
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiWireError 表示 Gemini 协议错误对象。
type GeminiWireError struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ErrorResponse 表示 Gemini HTTP 错误响应结构。
type ErrorResponse struct {
	Error GeminiWireError `json:"error"`
}
