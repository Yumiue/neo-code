package decider

import "strings"

// InferTaskKind 通过规则推断任务类型，避免依赖模型分类。
func InferTaskKind(goal string) TaskKind {
	text := strings.ToLower(strings.TrimSpace(goal))
	if text == "" {
		return TaskKindChatAnswer
	}

	hasTodo := containsAny(text, "todo", "待办")
	hasSubAgent := containsAny(text, "subagent", "子代理")
	hasWriteVerb := containsAny(
		text,
		"创建文件", "写入", "修改文件", "编辑文件", "新增文件", "补丁", "修复代码",
		"create file", "write file", "edit file", "update file", "apply patch", "implement", "fix",
	)
	hasFileTarget := containsAny(text, ".txt", ".go", ".md", ".json", ".yaml", ".yml", ".ts", ".tsx")
	hasWriteIntentToken := containsAny(text, "创建", "写", "改", "edit", "write", "update", "create", "modify")
	hasWrite := hasWriteVerb || (hasFileTarget && hasWriteIntentToken)
	hasRead := containsAny(
		text,
		"读取", "查看", "总结", "分析", "检索", "搜索", "审查", "review", "verify", "验证", "校验",
		"read", "grep", "glob", "list", "inspect", "analyze", "summarize",
	)
	hasPlan := containsAny(text, "计划", "规划", "plan", "todo 列表", "todo list")
	hasTodoAction := containsAny(text, "创建 todo", "更新 todo", "完成 todo", "标记 todo", "todo")

	switch {
	case hasSubAgent && hasWrite:
		return TaskKindSubAgent
	case hasSubAgent:
		return TaskKindSubAgent
	case hasPlan && hasTodo && !hasWrite:
		return TaskKindTodoState
	case hasTodo && hasTodoAction && !hasWrite:
		return TaskKindTodoState
	case hasWrite && hasRead:
		return TaskKindMixed
	case hasWrite:
		return TaskKindWorkspaceWrite
	case hasRead:
		return TaskKindReadOnly
	default:
		return TaskKindChatAnswer
	}
}

// containsAny 判断文本是否包含任一关键词。
func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(keyword))) {
			return true
		}
	}
	return false
}
