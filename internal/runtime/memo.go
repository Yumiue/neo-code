package runtime

import (
	"strings"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/tools"
)

// triggerMemoExtraction 在 Run 结束后异步触发记忆提取，避免阻塞主闭环。
func (s *Service) triggerMemoExtraction(sessionID string, messages []providertypes.Message, skip bool) {
	if s == nil || s.memoExtractor == nil || len(messages) == 0 {
		return
	}
	if skip {
		return
	}

	s.memoExtractor.Schedule(sessionID, cloneMessages(messages))
}

// isSuccessfulRememberToolCall 判断工具调用是否成功完成显式记忆写入。
func isSuccessfulRememberToolCall(callName string, result tools.ToolResult, execErr error) bool {
	if execErr != nil || result.IsError {
		return false
	}
	return strings.TrimSpace(callName) == tools.ToolNameMemoRemember
}

// cloneMessages 深拷贝消息切片，避免后台调度读取到后续运行态修改。
func cloneMessages(messages []providertypes.Message) []providertypes.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]providertypes.Message, 0, len(messages))
	for _, message := range messages {
		next := message
		if len(message.ToolCalls) > 0 {
			next.ToolCalls = append([]providertypes.ToolCall(nil), message.ToolCalls...)
		}
		if len(message.ToolMetadata) > 0 {
			next.ToolMetadata = make(map[string]string, len(message.ToolMetadata))
			for key, value := range message.ToolMetadata {
				next.ToolMetadata[key] = value
			}
		}
		cloned = append(cloned, next)
	}
	return cloned
}
