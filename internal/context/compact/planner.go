package compact

import (
	"fmt"
	"strings"

	"neo-code/internal/config"
	"neo-code/internal/provider"
)

// compactionPlan 描述一次 compact 在摘要生成前的归档与保留结果。
type compactionPlan struct {
	Archived             []provider.Message
	Retained             []provider.Message
	ArchivedMessageCount int
	Applied              bool
}

type atomicBlock struct {
	start        int
	end          int
	messageCount int
	protected    bool
}

// compactionPlanner 只负责选择策略并规划 archived/retained 消息边界。
type compactionPlanner struct{}

// Plan 根据 mode 与配置返回摘要前的裁剪规划结果。
func (compactionPlanner) Plan(mode Mode, messages []provider.Message, cfg config.CompactConfig) (compactionPlan, error) {
	if mode == ModeReactive {
		return planKeepRecent(messages, cfg.ManualKeepRecentMessages), nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ManualStrategy)) {
	case config.CompactManualStrategyKeepRecent:
		return planKeepRecent(messages, cfg.ManualKeepRecentMessages), nil
	case config.CompactManualStrategyFullReplace:
		return planFullReplace(messages), nil
	default:
		return compactionPlan{}, fmt.Errorf("compact: manual strategy %q is not supported", cfg.ManualStrategy)
	}
}

// planKeepRecent 计算 keep_recent 策略下需要摘要与保留的消息集合。
func planKeepRecent(messages []provider.Message, keepMessages int) compactionPlan {
	blocks := collectAtomicBlocks(messages)
	retainedStart := retainedStartForKeepRecent(blocks, keepMessages)
	if retainedStart <= 0 {
		return compactionPlan{
			Retained: cloneMessages(messages),
			Applied:  false,
		}
	}

	archived, retained := splitMessagesAt(messages, retainedStart)
	return compactionPlan{
		Archived:             archived,
		Retained:             retained,
		ArchivedMessageCount: len(archived),
		Applied:              len(archived) > 0,
	}
}

// planFullReplace 计算 full_replace 策略下需要摘要与保留的消息集合。
func planFullReplace(messages []provider.Message) compactionPlan {
	if len(messages) == 0 {
		return compactionPlan{}
	}

	blocks := collectAtomicBlocks(messages)
	retainedStart, hasProtectedTail := protectedTailStart(blocks)
	if !hasProtectedTail {
		retainedStart = len(messages)
	}

	archived, retained := splitMessagesAt(messages, retainedStart)
	return compactionPlan{
		Archived:             archived,
		Retained:             retained,
		ArchivedMessageCount: len(archived),
		Applied:              len(archived) > 0,
	}
}

// collectAtomicBlocks 按不可拆分的消息块分组，确保 tool call 与结果始终一起保留。
func collectAtomicBlocks(messages []provider.Message) []atomicBlock {
	blocks := make([]atomicBlock, 0, len(messages))
	for i := 0; i < len(messages); {
		start := i
		end := i + 1
		if messages[start].Role == provider.RoleAssistant && len(messages[start].ToolCalls) > 0 {
			for end < len(messages) && messages[end].Role == provider.RoleTool {
				end++
			}
		}
		blocks = append(blocks, atomicBlock{
			start:        start,
			end:          end,
			messageCount: end - start,
			protected:    isProtectedThoughtBlock(messages[start:end]),
		})
		i = end
	}
	if lastUserIndex := lastExplicitUserMessageIndex(messages); lastUserIndex >= 0 {
		markBlockProtected(blocks, lastUserIndex)
	}
	return blocks
}

// isProtectedThoughtBlock 为未来结构化 thought block 预留保护入口，当前默认不命中。
func isProtectedThoughtBlock(messages []provider.Message) bool {
	return false
}

// lastExplicitUserMessageIndex 返回最后一条非空用户消息的位置，用于保护最近明确指令。
func lastExplicitUserMessageIndex(messages []provider.Message) int {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == provider.RoleUser && strings.TrimSpace(messages[index].Content) != "" {
			return index
		}
	}
	return -1
}

// markBlockProtected 将包含目标消息的原子块标记为受保护块。
func markBlockProtected(blocks []atomicBlock, messageIndex int) {
	for index := range blocks {
		if messageIndex >= blocks[index].start && messageIndex < blocks[index].end {
			blocks[index].protected = true
			return
		}
	}
}

// retainedStartForKeepRecent 计算 keep_recent 策略下保留尾部的起点。
func retainedStartForKeepRecent(blocks []atomicBlock, keepMessages int) int {
	if len(blocks) == 0 {
		return 0
	}

	retainedStart := blocks[0].start
	retainedMessages := 0
	for index := len(blocks) - 1; index >= 0; index-- {
		retainedMessages += blocks[index].messageCount
		retainedStart = blocks[index].start
		if retainedMessages >= keepMessages {
			break
		}
	}

	protectedStart, ok := protectedTailStart(blocks)
	if ok && protectedStart < retainedStart {
		retainedStart = protectedStart
	}
	return retainedStart
}

// protectedTailStart 返回必须原样保留的受保护尾部起点。
func protectedTailStart(blocks []atomicBlock) (int, bool) {
	for index := range blocks {
		if blocks[index].protected {
			return blocks[index].start, true
		}
	}
	return 0, false
}

// splitMessagesAt 按 retained 起点切分 archived 与 retained，并返回深拷贝结果。
func splitMessagesAt(messages []provider.Message, retainedStart int) ([]provider.Message, []provider.Message) {
	if retainedStart <= 0 {
		return nil, cloneMessages(messages)
	}
	if retainedStart >= len(messages) {
		return cloneMessages(messages), nil
	}
	return cloneMessages(messages[:retainedStart]), cloneMessages(messages[retainedStart:])
}
