package anthropic

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

// BuildRequest 将通用 GenerateRequest 转换为 Anthropic /messages 请求结构。
func BuildRequest(ctx context.Context, cfg provider.RuntimeConfig, req providertypes.GenerateRequest) (Request, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(cfg.DefaultModel)
	}
	if model == "" {
		return Request{}, errors.New(errorPrefix + "model is empty")
	}

	payload := Request{
		Model:     model,
		MaxTokens: defaultMaxTokens,
		Messages:  make([]Message, 0, len(req.Messages)),
		Stream:    true,
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		payload.System = req.SystemPrompt
	}

	assetLimits := providertypes.NormalizeSessionAssetLimits(cfg.SessionAssetLimits)
	var usedSessionAssetBytes int64
	for _, message := range req.Messages {
		remainingSessionAssetBytes := assetLimits.MaxSessionAssetsTotalBytes - usedSessionAssetBytes
		converted, consumedBytes, err := toAnthropicMessageWithBudget(
			ctx,
			message,
			req.SessionAssetReader,
			remainingSessionAssetBytes,
			assetLimits,
		)
		if err != nil {
			return Request{}, err
		}
		usedSessionAssetBytes += consumedBytes
		if len(converted.Content) == 0 {
			continue
		}
		payload.Messages = append(payload.Messages, converted)
	}

	if len(req.Tools) > 0 {
		payload.Tools = make([]ToolDefinition, 0, len(req.Tools))
		for _, spec := range req.Tools {
			payload.Tools = append(payload.Tools, ToolDefinition{
				Name:        strings.TrimSpace(spec.Name),
				Description: strings.TrimSpace(spec.Description),
				InputSchema: normalizeToolSchema(spec.Schema),
			})
		}
	}

	return payload, nil
}

// toAnthropicMessage 将通用消息映射为 Anthropic 消息结构，并保留工具调用语义。
func toAnthropicMessage(
	ctx context.Context,
	message providertypes.Message,
	assetReader providertypes.SessionAssetReader,
) (Message, error) {
	converted, _, err := toAnthropicMessageWithBudget(
		ctx,
		message,
		assetReader,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return converted, err
}

// toAnthropicMessageWithBudget 将通用消息映射为 Anthropic 消息结构，并记录 session_asset 消耗字节数。
func toAnthropicMessageWithBudget(
	ctx context.Context,
	message providertypes.Message,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) (Message, int64, error) {
	if err := providertypes.ValidateParts(message.Parts); err != nil {
		return Message{}, 0, fmt.Errorf("%sinvalid message parts: %w", errorPrefix, err)
	}
	if remainingAssetBudget < 0 {
		remainingAssetBudget = 0
	}
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	var usedAssetBytes int64

	switch strings.TrimSpace(message.Role) {
	case providertypes.RoleSystem:
		return Message{}, usedAssetBytes, nil
	case providertypes.RoleUser:
		blocks, consumedBytes, err := toAnthropicTextBlocksWithBudget(
			ctx,
			message.Parts,
			assetReader,
			remainingAssetBudget,
			normalizedAssetLimits,
		)
		if err != nil {
			return Message{}, 0, err
		}
		usedAssetBytes += consumedBytes
		return Message{Role: "user", Content: blocks}, usedAssetBytes, nil
	case providertypes.RoleAssistant:
		blocks, consumedBytes, err := toAnthropicAssistantBlocksWithBudget(
			ctx,
			message,
			assetReader,
			remainingAssetBudget,
			normalizedAssetLimits,
		)
		if err != nil {
			return Message{}, 0, err
		}
		usedAssetBytes += consumedBytes
		return Message{Role: "assistant", Content: blocks}, usedAssetBytes, nil
	case providertypes.RoleTool:
		block, err := toAnthropicToolResultBlock(message)
		if err != nil {
			return Message{}, 0, err
		}
		return Message{Role: "user", Content: []ContentBlock{block}}, usedAssetBytes, nil
	default:
		return Message{}, 0, fmt.Errorf("%sunsupported message role %q", errorPrefix, message.Role)
	}
}

// toAnthropicTextBlocks 将文本内容转换为 Anthropic text block。
func toAnthropicTextBlocks(parts []providertypes.ContentPart) ([]ContentBlock, error) {
	blocks, _, err := toAnthropicTextBlocksWithBudget(
		context.Background(),
		parts,
		nil,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return blocks, err
}

// toAnthropicTextBlocksWithBudget 将文本/图片内容转换为 Anthropic 块，并记录 session_asset 消耗。
func toAnthropicTextBlocksWithBudget(
	ctx context.Context,
	parts []providertypes.ContentPart,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) ([]ContentBlock, int64, error) {
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	if remainingAssetBudget < 0 {
		remainingAssetBudget = 0
	}
	blocks := make([]ContentBlock, 0, len(parts))
	var usedAssetBytes int64
	for _, part := range parts {
		switch part.Kind {
		case providertypes.ContentPartText:
			if part.Text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: part.Text})
			}
		case providertypes.ContentPartImage:
			switch {
			case part.Image != nil && part.Image.SourceType == providertypes.ImageSourceRemote:
				blocks = append(blocks, ContentBlock{
					Type: "image",
					Source: &ImageSource{
						Type: "url",
						URL:  part.Image.URL,
					},
				})
			case part.Image != nil && part.Image.SourceType == providertypes.ImageSourceSessionAsset:
				if part.Image.Asset == nil || strings.TrimSpace(part.Image.Asset.ID) == "" {
					return nil, 0, errors.New("session_asset image missing asset id")
				}
				if assetReader == nil {
					return nil, 0, errors.New("session_asset reader is not configured")
				}
				source, readBytes, err := resolveSessionAssetImageSource(
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
				blocks = append(blocks, ContentBlock{Type: "image", Source: source})
			default:
				return nil, 0, errors.New("unsupported source type for image part")
			}
		}
	}
	return blocks, usedAssetBytes, nil
}

// toAnthropicAssistantBlocks 将助手消息转换为文本与 tool_use 块。
func toAnthropicAssistantBlocks(message providertypes.Message) ([]ContentBlock, error) {
	blocks, _, err := toAnthropicAssistantBlocksWithBudget(
		context.Background(),
		message,
		nil,
		maxSessionAssetsTotalBytes,
		providertypes.DefaultSessionAssetLimits(),
	)
	return blocks, err
}

// toAnthropicAssistantBlocksWithBudget 将助手消息转换为文本、图片与 tool_use 块。
func toAnthropicAssistantBlocksWithBudget(
	ctx context.Context,
	message providertypes.Message,
	assetReader providertypes.SessionAssetReader,
	remainingAssetBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) ([]ContentBlock, int64, error) {
	blocks, consumedBytes, err := toAnthropicTextBlocksWithBudget(
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
		input, err := decodeToolArgumentsToObject(call.Arguments)
		if err != nil {
			return nil, 0, err
		}
		blocks = append(blocks, ContentBlock{
			Type:  "tool_use",
			ID:    strings.TrimSpace(call.ID),
			Name:  name,
			Input: input,
		})
	}
	return blocks, consumedBytes, nil
}

// toAnthropicToolResultBlock 将工具结果消息映射为 tool_result 块。
func toAnthropicToolResultBlock(message providertypes.Message) (ContentBlock, error) {
	toolUseID := strings.TrimSpace(message.ToolCallID)
	if toolUseID == "" {
		return ContentBlock{}, errors.New(errorPrefix + "tool result message requires tool_call_id")
	}
	content := renderMessageText(message.Parts)
	if content == "" {
		content = ""
	}
	return ContentBlock{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   content,
	}, nil
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

// renderMessageText 折叠消息中的文本片段，供 tool_result 透传使用。
func renderMessageText(parts []providertypes.ContentPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if part.Kind == providertypes.ContentPartText {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

// normalizeToolSchema 归一化工具 schema，确保顶层为 object。
func normalizeToolSchema(schema map[string]any) map[string]any {
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

// resolveSessionAssetImageSource 读取会话附件并转换为 Anthropic 可发送的 base64 source，仅在请求阶段临时生成。
func resolveSessionAssetImageSource(
	ctx context.Context,
	assetReader providertypes.SessionAssetReader,
	asset *providertypes.AssetRef,
	remainingBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) (*ImageSource, int64, error) {
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

	return &ImageSource{
		Type:      "base64",
		MediaType: normalizedMime,
		Data:      base64.StdEncoding.EncodeToString(data),
	}, int64(len(data)), nil
}
