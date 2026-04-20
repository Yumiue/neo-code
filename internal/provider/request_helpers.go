package provider

import (
	"encoding/json"
	"strings"

	providertypes "neo-code/internal/provider/types"
)

// CloneSchemaTopLevel 复制 schema 顶层 map，避免归一化阶段污染调用方输入。
func CloneSchemaTopLevel(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(schema))
	for key, value := range schema {
		cloned[key] = value
	}
	return cloned
}

// NormalizeToolSchemaObject 归一化工具 schema，保证顶层为 object 且始终具备 properties。
func NormalizeToolSchemaObject(schema map[string]any) map[string]any {
	normalized := CloneSchemaTopLevel(schema)
	if len(normalized) == 0 {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	typeName, _ := normalized["type"].(string)
	if strings.TrimSpace(strings.ToLower(typeName)) != "object" {
		normalized["type"] = "object"
	}
	if _, ok := normalized["properties"].(map[string]any); !ok {
		normalized["properties"] = map[string]any{}
	}
	return normalized
}

// DecodeToolArgumentsToObject 将工具参数 JSON 解码为对象，失败时回退为包裹字段。
func DecodeToolArgumentsToObject(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return map[string]any{"raw": trimmed}, nil
	}
	if object, ok := parsed.(map[string]any); ok {
		return object, nil
	}
	return map[string]any{"value": parsed}, nil
}

// RenderMessageText 折叠消息中的文本片段，供协议适配层复用。
func RenderMessageText(parts []providertypes.ContentPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Kind == providertypes.ContentPartText {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}
