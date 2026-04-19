package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"neo-code/internal/provider"
	providertypes "neo-code/internal/provider/types"
)

const defaultMaxTokens = 4096
const maxSessionAssetsTotalBytes = providertypes.MaxSessionAssetsTotalBytes

// BuildRequest 将通用 GenerateRequest 转换为 Gemini native 请求结构。
func BuildRequest(ctx context.Context, cfg provider.RuntimeConfig, req providertypes.GenerateRequest) (Request, string, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(cfg.DefaultModel)
	}
	if model == "" {
		return Request{}, "", errors.New(errorPrefix + "model is empty")
	}

	payload := Request{
		Contents: make([]Content, 0, len(req.Messages)),
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		payload.SystemInstruction = &Content{
			Parts: []Part{{Text: req.SystemPrompt}},
		}
	}

	assetLimits := providertypes.NormalizeSessionAssetLimits(cfg.SessionAssetLimits)
	var usedSessionAssetBytes int64
	for _, message := range req.Messages {
		remainingSessionAssetBytes := assetLimits.MaxSessionAssetsTotalBytes - usedSessionAssetBytes
		content, consumedBytes, err := toGeminiContentWithBudget(
			ctx,
			message,
			req.SessionAssetReader,
			remainingSessionAssetBytes,
			assetLimits,
		)
		if err != nil {
			return Request{}, "", err
		}
		usedSessionAssetBytes += consumedBytes
		if len(content.Parts) == 0 {
			continue
		}
		payload.Contents = append(payload.Contents, content)
	}

	if len(req.Tools) > 0 {
		decls := make([]FunctionDeclaration, 0, len(req.Tools))
		for _, spec := range req.Tools {
			decls = append(decls, FunctionDeclaration{
				Name:        strings.TrimSpace(spec.Name),
				Description: strings.TrimSpace(spec.Description),
				Parameters:  normalizeToolSchemaForGemini(spec.Schema),
			})
		}
		payload.Tools = []Tool{{FunctionDeclarations: decls}}
		payload.ToolConfig = &ToolConfig{FunctionCallingConfig: FunctionCallingConfig{Mode: "AUTO"}}
	}

	return payload, model, nil
}

// toGeminiContent 将通用消息转换为 Gemini content，并保留工具调用语义。
func toGeminiContent(ctx context.Context, message providertypes.Message, assetReader providertypes.SessionAssetReader) (Content, error) {
	content, _, err := toGeminiContentWithBudget(
		ctx,
		message,
		assetReader,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return content, err
}

// toGeminiContentWithBudget 将通用消息转换为 Gemini content，并记录 session_asset 消耗字节数。
func toGeminiContentWithBudget(
	ctx context.Context,
	message providertypes.Message,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) (Content, int64, error) {
	if err := providertypes.ValidateParts(message.Parts); err != nil {
		return Content{}, 0, fmt.Errorf("%sinvalid message parts: %w", errorPrefix, err)
	}
	if remainingAssetBudget < 0 {
		remainingAssetBudget = 0
	}
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	var usedAssetBytes int64

	switch strings.TrimSpace(message.Role) {
	case providertypes.RoleSystem:
		return Content{}, usedAssetBytes, nil
	case providertypes.RoleUser:
		parts, consumedBytes, err := toGeminiUserPartsWithBudget(
			ctx,
			message.Parts,
			assetReader,
			remainingAssetBudget,
			normalizedAssetLimits,
		)
		if err != nil {
			return Content{}, 0, err
		}
		usedAssetBytes += consumedBytes
		return Content{Role: "user", Parts: parts}, usedAssetBytes, nil
	case providertypes.RoleAssistant:
		parts, consumedBytes, err := toGeminiAssistantPartsWithBudget(
			ctx,
			message,
			assetReader,
			remainingAssetBudget,
			normalizedAssetLimits,
		)
		if err != nil {
			return Content{}, 0, err
		}
		usedAssetBytes += consumedBytes
		return Content{Role: "model", Parts: parts}, usedAssetBytes, nil
	case providertypes.RoleTool:
		part, err := toGeminiToolResultPart(message)
		if err != nil {
			return Content{}, 0, err
		}
		return Content{Role: "user", Parts: []Part{part}}, usedAssetBytes, nil
	default:
		return Content{}, 0, fmt.Errorf("%sunsupported message role %q", errorPrefix, message.Role)
	}
}

// toGeminiUserParts 将用户消息内容转换为 Gemini parts。
func toGeminiUserParts(parts []providertypes.ContentPart) ([]Part, error) {
	converted, _, err := toGeminiPartsWithBudget(
		context.Background(),
		parts,
		nil,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return converted, err
}

// toGeminiUserPartsWithBudget 将用户消息内容转换为 Gemini parts，并记录 session_asset 消耗。
func toGeminiUserPartsWithBudget(
	ctx context.Context,
	parts []providertypes.ContentPart,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) ([]Part, int64, error) {
	return toGeminiPartsWithBudget(ctx, parts, assetReader, remainingAssetBudget, assetLimits)
}

// toGeminiAssistantParts 将助手消息转换为 Gemini model parts，包含文本与 functionCall。
func toGeminiAssistantParts(message providertypes.Message) ([]Part, error) {
	converted, _, err := toGeminiAssistantPartsWithBudget(
		context.Background(),
		message,
		nil,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return converted, err
}

// toGeminiAssistantPartsWithBudget 将助手消息转换为 Gemini model parts，并记录 session_asset 消耗。
func toGeminiAssistantPartsWithBudget(
	ctx context.Context,
	message providertypes.Message,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) ([]Part, int64, error) {
	result, consumedBytes, err := toGeminiPartsWithBudget(
		ctx,
		message.Parts,
		assetReader,
		remainingAssetBudget,
		assetLimits,
	)
	if err != nil {
		return nil, 0, err
	}
	for _, call := range message.ToolCalls {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			continue
		}
		args, err := decodeToolArgumentsToObject(call.Arguments)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, Part{
			FunctionCall: &FunctionCall{
				ID:   strings.TrimSpace(call.ID),
				Name: name,
				Args: args,
			},
		})
	}
	return result, consumedBytes, nil
}

// toGeminiPartsWithBudget 将文本/图片片段映射到 Gemini parts，并执行 session_asset 预算校验。
func toGeminiPartsWithBudget(
	ctx context.Context,
	parts []providertypes.ContentPart,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) ([]Part, int64, error) {
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	if remainingAssetBudget < 0 {
		remainingAssetBudget = 0
	}
	result := make([]Part, 0, len(parts))
	var usedAssetBytes int64
	for _, part := range parts {
		switch part.Kind {
		case providertypes.ContentPartText:
			if part.Text != "" {
				result = append(result, Part{Text: part.Text})
			}
		case providertypes.ContentPartImage:
			switch {
			case part.Image != nil && part.Image.SourceType == providertypes.ImageSourceRemote:
				result = append(result, Part{
					FileData: &FileData{
						FileURI: part.Image.URL,
					},
				})
			case part.Image != nil && part.Image.SourceType == providertypes.ImageSourceSessionAsset:
				if part.Image.Asset == nil || strings.TrimSpace(part.Image.Asset.ID) == "" {
					return nil, 0, errors.New("session_asset image missing asset id")
				}
				if assetReader == nil {
					return nil, 0, errors.New("session_asset reader is not configured")
				}
				inlineData, readBytes, err := resolveSessionAssetInlineData(
					ctx,
					assetReader,
					part.Image.Asset,
					remainingAssetBudget-usedAssetBytes,
					normalizedAssetLimits,
				)
				if err != nil {
					return nil, 0, err
				}
				usedAssetBytes += readBytes
				result = append(result, Part{InlineData: inlineData})
			default:
				return nil, 0, errors.New("unsupported source type for image part")
			}
		}
	}
	return result, usedAssetBytes, nil
}

// toGeminiToolResultPart 将工具结果消息映射为 Gemini functionResponse part。
func toGeminiToolResultPart(message providertypes.Message) (Part, error) {
	toolName := strings.TrimSpace(message.ToolMetadata["tool_name"])
	if toolName == "" {
		toolName = "tool"
	}

	response := map[string]any{}
	text := renderMessageText(message.Parts)
	if text != "" {
		response["content"] = text
	}
	if toolCallID := strings.TrimSpace(message.ToolCallID); toolCallID != "" {
		response["tool_call_id"] = toolCallID
	}
	if len(message.ToolMetadata) > 0 {
		metadata := make(map[string]any, len(message.ToolMetadata))
		for key, value := range message.ToolMetadata {
			metadata[key] = value
		}
		response["metadata"] = metadata
	}
	if len(response) == 0 {
		response["content"] = ""
	}

	return Part{FunctionResponse: &FunctionResponse{Name: toolName, Response: response}}, nil
}

// decodeToolArgumentsToObject 将工具参数 JSON 解码为对象，失败时回退 raw 字符串包装。
func decodeToolArgumentsToObject(raw string) (map[string]any, error) {
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

// renderMessageText 折叠消息中的文本片段，供工具结果透传使用。
func renderMessageText(parts []providertypes.ContentPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Kind == providertypes.ContentPartText {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

// normalizeToolSchemaForGemini 归一化工具参数 schema，避免非法顶层结构导致服务端拒绝。
func normalizeToolSchemaForGemini(schema map[string]any) map[string]any {
	normalized := cloneSchemaTopLevel(schema)
	if len(normalized) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
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

// cloneSchemaTopLevel 复制 schema 顶层 map，避免归一化阶段污染调用方输入。
func cloneSchemaTopLevel(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(schema))
	for key, value := range schema {
		cloned[key] = value
	}
	return cloned
}

// resolveSessionAssetInlineData 读取会话附件并转换为 Gemini 可发送的 inlineData，仅在请求阶段临时生成。
func resolveSessionAssetInlineData(
	ctx context.Context,
	assetReader providertypes.SessionAssetReader,
	asset *providertypes.AssetRef,
	remainingBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) (*InlineData, int64, error) {
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if remainingBudget <= 0 {
		return nil, 0, fmt.Errorf(
			"session_asset total exceeds %d bytes",
			normalizedAssetLimits.MaxSessionAssetsTotalBytes,
		)
	}
	reader, mimeType, err := assetReader.Open(ctx, asset.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("open session_asset %q: %w", asset.ID, err)
	}
	defer func() { _ = reader.Close() }()

	readLimit := normalizedAssetLimits.MaxSessionAssetBytes
	if remainingBudget < readLimit {
		readLimit = remainingBudget
	}

	data, err := io.ReadAll(io.LimitReader(reader, readLimit+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read session_asset %q: %w", asset.ID, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if int64(len(data)) > readLimit {
		if readLimit < normalizedAssetLimits.MaxSessionAssetBytes {
			return nil, 0, fmt.Errorf(
				"session_asset total exceeds %d bytes",
				normalizedAssetLimits.MaxSessionAssetsTotalBytes,
			)
		}
		return nil, 0, fmt.Errorf("session_asset %q exceeds %d bytes", asset.ID, normalizedAssetLimits.MaxSessionAssetBytes)
	}
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("session_asset %q is empty", asset.ID)
	}

	resolvedMime := strings.TrimSpace(mimeType)
	if resolvedMime == "" {
		resolvedMime = strings.TrimSpace(asset.MimeType)
	}
	normalizedMime := strings.ToLower(resolvedMime)
	if normalizedMime == "" {
		return nil, 0, fmt.Errorf("session_asset %q missing mime type", asset.ID)
	}
	if !strings.HasPrefix(normalizedMime, "image/") {
		return nil, 0, fmt.Errorf("session_asset %q has unsupported mime type %q", asset.ID, resolvedMime)
	}

	return &InlineData{
		MimeType: normalizedMime,
		Data:     base64.StdEncoding.EncodeToString(data),
	}, int64(len(data)), nil
}
