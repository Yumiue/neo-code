package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/controlplane"
	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
)

type toolExecutionSummary struct {
	Calls                       []providertypes.ToolCall
	Results                     []tools.ToolResult
	HasSuccessfulWorkspaceWrite bool
	HasSuccessfulVerification   bool
}

// collectCompletionState 基于当前运行态与本轮 assistant 行为生成 completion 输入。
func collectCompletionState(
	state *runState,
	_ providertypes.Message,
	assistantHasToolCalls bool,
) controlplane.CompletionState {
	current := state.completion
	current.HasPendingAgentTodos = hasPendingAgentTodos(state.session.Todos)
	if assistantHasToolCalls {
		current.LastTurnVerifyPassed = false
		return current
	}
	return current
}

// applyToolExecutionCompletion 更新一轮工具执行后的 completion 事实。
func applyToolExecutionCompletion(current controlplane.CompletionState, summary toolExecutionSummary) controlplane.CompletionState {
	current.LastTurnVerifyPassed = false
	if summary.HasSuccessfulWorkspaceWrite {
		current.RequiresVerification = true
		current.HasUnverifiedWrites = true
	}
	if current.RequiresVerification && summary.HasSuccessfulVerification {
		current.HasUnverifiedWrites = false
		current.LastTurnVerifyPassed = true
	}
	return current
}

// collectProgressInput 基于执行前后事实组装 progress 评估输入。
func collectProgressInput(
	runState controlplane.RunState,
	beforeTask agentsession.TaskState,
	afterTask agentsession.TaskState,
	beforeTodos []agentsession.TodoItem,
	afterTodos []agentsession.TodoItem,
	summary toolExecutionSummary,
	verifyPassed bool,
	noProgressLimit int,
	repeatLimit int,
) controlplane.ProgressInput {
	evidence := deriveProgressEvidence(beforeTask, afterTask, beforeTodos, afterTodos, summary, verifyPassed)
	return controlplane.ProgressInput{
		RunState:             runState,
		Evidence:             evidence,
		CurrentToolSignature: computeToolSignature(summary.Calls),
		ResultFingerprint:    computeToolResultFingerprint(summary.Results),
		SubgoalFingerprint:   computeSubgoalFingerprint(afterTask, afterTodos, summary.Calls),
		NoProgressLimit:      noProgressLimit,
		RepeatCycleLimit:     repeatLimit,
	}
}

// deriveProgressEvidence 从本轮前后快照和工具摘要中提取结构化 evidence。
func deriveProgressEvidence(
	beforeTask agentsession.TaskState,
	afterTask agentsession.TaskState,
	beforeTodos []agentsession.TodoItem,
	afterTodos []agentsession.TodoItem,
	summary toolExecutionSummary,
	verifyPassed bool,
) []controlplane.ProgressEvidenceRecord {
	var evidence []controlplane.ProgressEvidenceRecord

	if computeTaskStateSignature(beforeTask) != computeTaskStateSignature(afterTask) {
		evidence = append(evidence, controlplane.ProgressEvidenceRecord{Kind: controlplane.EvidenceTaskStateChanged})
	}
	if computeTodoStateSignature(beforeTodos) != computeTodoStateSignature(afterTodos) {
		evidence = append(evidence, controlplane.ProgressEvidenceRecord{Kind: controlplane.EvidenceTodoStateChanged})
	}
	if summary.HasSuccessfulWorkspaceWrite {
		evidence = append(evidence, controlplane.ProgressEvidenceRecord{Kind: controlplane.EvidenceWriteApplied})
	}
	if verifyPassed {
		evidence = append(evidence, controlplane.ProgressEvidenceRecord{Kind: controlplane.EvidenceVerifyPassed})
	}
	if hasSuccessfulInformationalResult(summary.Results) {
		evidence = append(evidence, controlplane.ProgressEvidenceRecord{Kind: controlplane.EvidenceNewInfoNonDup})
	}
	return evidence
}

// computeTaskStateSignature 计算 task_state 的结构化签名。
func computeTaskStateSignature(task agentsession.TaskState) string {
	encoded, err := json.Marshal(task.Clone())
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

// computeToolResultFingerprint 计算本轮工具结果的聚合指纹。
func computeToolResultFingerprint(results []tools.ToolResult) string {
	if len(results) == 0 {
		return ""
	}
	type normalizedResult struct {
		Name       string `json:"name"`
		IsError    bool   `json:"is_error"`
		Content    string `json:"content"`
		ErrorClass string `json:"error_class,omitempty"`
	}

	normalized := make([]normalizedResult, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Name) == "" {
			return ""
		}
		entry := normalizedResult{
			Name:    strings.TrimSpace(result.Name),
			IsError: result.IsError,
			Content: normalizeToolResultContent(result.Content),
		}
		if result.IsError {
			entry.ErrorClass = classifyToolError(result)
		}
		normalized = append(normalized, entry)
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

// computeSubgoalFingerprint 生成当前轮子目标的轻量指纹。
func computeSubgoalFingerprint(
	task agentsession.TaskState,
	todos []agentsession.TodoItem,
	calls []providertypes.ToolCall,
) string {
	type subgoalSnapshot struct {
		NextStep  string   `json:"next_step,omitempty"`
		OpenItems []string `json:"open_items,omitempty"`
		Todos     []string `json:"todos,omitempty"`
	}

	snapshot := subgoalSnapshot{
		NextStep:  strings.TrimSpace(task.NextStep),
		OpenItems: append([]string(nil), task.OpenItems...),
	}
	for _, item := range todos {
		if item.Status.IsTerminal() {
			continue
		}
		snapshot.Todos = append(snapshot.Todos, strings.TrimSpace(item.Content))
	}
	if snapshot.NextStep == "" && len(snapshot.OpenItems) == 0 && len(snapshot.Todos) == 0 {
		return computeToolSignature(calls)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

// hasPendingAgentTodos 判断当前 session 中是否仍存在未闭合 todo。
func hasPendingAgentTodos(items []agentsession.TodoItem) bool {
	for _, item := range items {
		if item.Status.IsTerminal() {
			continue
		}
		return true
	}
	return false
}

// hasSuccessfulInformationalResult 判断本轮是否至少获得一个成功的非写入工具结果。
func hasSuccessfulInformationalResult(results []tools.ToolResult) bool {
	for _, result := range results {
		if result.IsError {
			continue
		}
		switch strings.TrimSpace(result.Name) {
		case tools.ToolNameFilesystemWriteFile, tools.ToolNameFilesystemEdit:
			continue
		default:
			return true
		}
	}
	return false
}

// hasSuccessfulVerificationResult 判断本轮是否执行了显式验证动作且获得成功结果。
func hasSuccessfulVerificationResult(calls []providertypes.ToolCall, results []tools.ToolResult) bool {
	if len(calls) == 0 || len(results) == 0 {
		return false
	}

	successful := make(map[string]tools.ToolResult, len(results))
	for _, result := range results {
		if result.IsError {
			continue
		}
		toolCallID := strings.TrimSpace(result.ToolCallID)
		if toolCallID != "" {
			successful[toolCallID] = result
			continue
		}
		key := "name:" + strings.ToLower(strings.TrimSpace(result.Name))
		successful[key] = result
	}

	for _, call := range calls {
		if !isExplicitVerificationCall(call) {
			continue
		}
		if _, ok := successful[strings.TrimSpace(call.ID)]; ok {
			return true
		}
		if _, ok := successful["name:"+strings.ToLower(strings.TrimSpace(call.Name))]; ok {
			return true
		}
	}
	return false
}

// isExplicitVerificationCall 判断工具调用是否明确承担验证职责，避免把任意成功读取都算成 verify passed。
func isExplicitVerificationCall(call providertypes.ToolCall) bool {
	if !strings.EqualFold(strings.TrimSpace(call.Name), tools.ToolNameBash) {
		return false
	}

	command, ok := parseBashVerificationCommand(call.Arguments)
	if !ok {
		return false
	}
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}

	for _, keyword := range verificationCommandKeywords {
		if strings.Contains(command, keyword) {
			return true
		}
	}
	return false
}

// parseBashVerificationCommand 解析 bash 工具参数中的 command 字段，为验证分类提供稳定输入。
func parseBashVerificationCommand(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	var payload struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false
	}
	command := strings.TrimSpace(payload.Command)
	if command == "" {
		return "", false
	}
	return command, true
}

// normalizeToolResultContent 对工具结果文本做稳定化裁剪，避免无关差异放大指纹抖动。
func normalizeToolResultContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) <= 256 {
		return trimmed
	}
	return trimmed[:256]
}

// classifyToolError 为错误结果生成轻量分类，避免直接依赖完整错误文案。
func classifyToolError(result tools.ToolResult) string {
	trimmed := strings.ToLower(strings.TrimSpace(result.Content))
	switch {
	case strings.Contains(trimmed, "timeout"):
		return "timeout"
	case strings.Contains(trimmed, "denied"):
		return "permission_denied"
	case strings.Contains(trimmed, "not found"):
		return "not_found"
	default:
		return "generic_error"
	}
}

var verificationCommandKeywords = []string{
	"go test",
	"go vet",
	"go build",
	"golangci-lint",
	"pytest",
	"ruff check",
	"mypy",
	"cargo test",
	"cargo check",
	"cargo clippy",
	"npm test",
	"npm run test",
	"npm run lint",
	"npm run build",
	"pnpm test",
	"pnpm run test",
	"pnpm run lint",
	"pnpm run build",
	"yarn test",
	"yarn lint",
	"yarn build",
	"make test",
	"make check",
	"ctest",
	"gradle test",
	"mvn test",
}
