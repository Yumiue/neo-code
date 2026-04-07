package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"neo-code/internal/tools"
)

const mcpToolNamePrefix = "mcp."

// AdapterFactory 基于 registry 快照构造 MCP tool 适配器集合。
type AdapterFactory struct {
	registry *Registry
}

// NewAdapterFactory 创建 MCP adapter 工厂。
func NewAdapterFactory(registry *Registry) *AdapterFactory {
	return &AdapterFactory{registry: registry}
}

// BuildTools 将当前所有 MCP tool 快照转换为统一 tools.Tool 列表。
func (f *AdapterFactory) BuildTools(ctx context.Context) ([]tools.Tool, error) {
	if f == nil || f.registry == nil {
		return nil, errors.New("mcp: adapter factory registry is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	snapshots := f.registry.Snapshot()
	if len(snapshots) == 0 {
		return nil, nil
	}

	result := make([]tools.Tool, 0, len(snapshots)*2)
	for _, snapshot := range snapshots {
		for _, descriptor := range snapshot.Tools {
			adapter, err := NewAdapter(f.registry, snapshot.ServerID, descriptor)
			if err != nil {
				return nil, err
			}
			result = append(result, adapter)
		}
	}
	return result, nil
}

// Adapter 将单个 MCP tool 适配为统一 tools.Tool 接口。
type Adapter struct {
	registry    *Registry
	serverID    string
	toolName    string
	description string
	schema      map[string]any
}

// NewAdapter 创建指定 server/tool 的 MCP 适配器。
func NewAdapter(registry *Registry, serverID string, descriptor ToolDescriptor) (*Adapter, error) {
	if registry == nil {
		return nil, errors.New("mcp: registry is nil")
	}
	normalizedServerID := normalizeServerID(serverID)
	if normalizedServerID == "" {
		return nil, errors.New("mcp: server id is empty")
	}
	normalizedToolName := strings.TrimSpace(descriptor.Name)
	if normalizedToolName == "" {
		return nil, errors.New("mcp: descriptor tool name is empty")
	}

	return &Adapter{
		registry:    registry,
		serverID:    normalizedServerID,
		toolName:    normalizedToolName,
		description: strings.TrimSpace(descriptor.Description),
		schema:      ensureObjectSchema(descriptor.InputSchema),
	}, nil
}

// Name 返回统一的 MCP tool 名称：mcp.<server_id>.<tool_name>。
func (a *Adapter) Name() string {
	return composeToolName(a.serverID, a.toolName)
}

// Description 返回工具描述，不存在时回退到稳定默认文案。
func (a *Adapter) Description() string {
	if strings.TrimSpace(a.description) != "" {
		return a.description
	}
	return fmt.Sprintf("MCP tool %s from server %s", a.toolName, a.serverID)
}

// Schema 返回 MCP 工具输入 schema 的标准对象结构。
func (a *Adapter) Schema() map[string]any {
	return cloneSchema(a.schema)
}

// MicroCompactPolicy 返回 MCP tool 历史结果默认 micro compact 策略。
func (a *Adapter) MicroCompactPolicy() tools.MicroCompactPolicy {
	return tools.MicroCompactPolicyCompact
}

// Execute 分发 MCP tool 调用并收敛为统一 ToolResult。
func (a *Adapter) Execute(ctx context.Context, call tools.ToolCallInput) (tools.ToolResult, error) {
	if a == nil || a.registry == nil {
		err := errors.New("mcp: adapter is not initialized")
		return tools.NewErrorResult("mcp", tools.NormalizeErrorReason("mcp", err), "", nil), err
	}
	if err := ctx.Err(); err != nil {
		return tools.NewErrorResult(a.Name(), tools.NormalizeErrorReason(a.Name(), err), "", adapterMetadata(a.serverID, a.toolName)), err
	}

	result, err := a.registry.Call(ctx, a.serverID, a.toolName, call.Arguments)
	if err != nil {
		errorResult := tools.NewErrorResult(a.Name(), tools.NormalizeErrorReason(a.Name(), err), "", adapterMetadata(a.serverID, a.toolName))
		errorResult.ToolCallID = call.ID
		return errorResult, err
	}

	metadata := adapterMetadata(a.serverID, a.toolName)
	for key, value := range result.Metadata {
		metadata[key] = value
	}

	toolResult := tools.ToolResult{
		ToolCallID: call.ID,
		Name:       a.Name(),
		Content:    strings.TrimSpace(result.Content),
		IsError:    result.IsError,
		Metadata:   metadata,
	}
	if strings.TrimSpace(toolResult.Content) == "" {
		toolResult.Content = "ok"
	}
	return tools.ApplyOutputLimit(toolResult, tools.DefaultOutputLimitBytes), nil
}

// composeToolName 组装统一的 MCP tool 名称，保持权限映射可预测。
func composeToolName(serverID string, toolName string) string {
	return mcpToolNamePrefix + normalizeServerID(serverID) + "." + strings.TrimSpace(toolName)
}

// ensureObjectSchema 确保 schema 至少是 object，避免上层 provider 解析异常。
func ensureObjectSchema(schema map[string]any) map[string]any {
	cloned := cloneSchema(schema)
	if len(cloned) == 0 {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	if strings.TrimSpace(fmt.Sprintf("%v", cloned["type"])) == "" {
		cloned["type"] = "object"
	}
	if _, ok := cloned["properties"]; !ok {
		cloned["properties"] = map[string]any{}
	}
	return cloned
}

// adapterMetadata 生成 MCP 调用结果的基础元信息。
func adapterMetadata(serverID string, toolName string) map[string]any {
	return map[string]any{
		"mcp_server_id": normalizeServerID(serverID),
		"mcp_tool_name": strings.TrimSpace(toolName),
	}
}
