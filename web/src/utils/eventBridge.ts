import { EventType, type MessageFrame, type TokenUsage, type PermissionRequestPayload } from '@/api/protocol'
import { useChatStore, createAssistantMessage, createToolCallMessage } from '@/store/useChatStore'
import { useUIStore } from '@/store/useUIStore'
import { useGatewayStore } from '@/store/useGatewayStore'
import { useSessionStore } from '@/store/useSessionStore'

/**
 * 将 Gateway SSE 事件帧桥接到 Zustand store action。
 * 从 Go internal/tui/services/gateway_stream_client.go 的 decodeRuntimeEventFromGatewayNotification 对齐。
 */
export function handleGatewayEvent(frame: MessageFrame) {
  const payload = frame.payload as Record<string, unknown> | undefined
  if (!payload) return

  const eventType = payload.event_type as string | undefined
  if (!eventType) return

  const chatStore = useChatStore.getState()
  const uiStore = useUIStore.getState()
  const gwStore = useGatewayStore.getState()

  // 从帧级别提取 session_id / run_id
  const frameSessionId = frame.session_id
  const frameRunId = frame.run_id

  switch (eventType) {
    case EventType.AgentChunk: {
      // AI 文本流式输出
      const text = payload.payload as string | undefined
      if (!text) break

      // 如果没有流式消息，创建一条
      if (!chatStore.streamingMessageId) {
        const msg = createAssistantMessage()
        chatStore.addMessage(msg)
        chatStore.setStreamingMessageId(msg.id)
      }
      chatStore.appendChunk(text)
      break
    }

    case EventType.AgentDone: {
      // AI 回复完成
      if (chatStore.streamingMessageId) {
        const content = (payload.payload as Record<string, unknown>)?.content as string | undefined ?? ''
        chatStore.finalizeMessage(chatStore.streamingMessageId, content)
      }
      chatStore.setGenerating(false)

      // AgentDone 后刷新会话列表（标题可能已更新）
      if (frameSessionId) {
        useSessionStore.getState().fetchSessions().catch(() => {})
      }
      break
    }

    case EventType.ToolStart: {
      // 工具调用开始
      const toolPayload = payload.payload as Record<string, unknown> | undefined
      const toolName = (toolPayload?.name as string) ?? 'unknown'
      const toolCallId = (toolPayload?.id as string) ?? ''
      const toolArgs = (toolPayload?.arguments as string) ?? ''
      const msg = createToolCallMessage(toolName, toolCallId, toolArgs)
      chatStore.addMessage(msg)
      break
    }

    case EventType.ToolResult: {
      // 工具调用结果
      const resultPayload = payload.payload as Record<string, unknown> | undefined
      const toolCallId = (resultPayload?.tool_call_id as string) ?? ''
      const content = (resultPayload?.content as string) ?? ''
      chatStore.updateToolCall(toolCallId, content, 'done')
      break
    }

    case EventType.ToolChunk: {
      // 工具输出流式片段
      const chunkPayload = payload.payload as Record<string, unknown> | undefined
      const toolCallId = (chunkPayload?.tool_call_id as string) ?? ''
      const chunk = (chunkPayload?.content as string) ?? ''
      if (toolCallId) {
        chatStore.appendToolOutput(toolCallId, chunk)
      }
      break
    }

    case EventType.UserMessage: {
      // 用户消息确认回显（可选：服务端确认后更新）
      break
    }

    case EventType.InputNormalized: {
      // 输入归一化事件，提取 session_id / run_id
      const normPayload = payload.payload as Record<string, unknown> | undefined
      const sessionId = (normPayload?.session_id as string) ?? frameSessionId ?? ''
      const runId = (normPayload?.run_id as string) ?? frameRunId ?? ''
      if (runId) gwStore.setCurrentRunId(runId)
      if (sessionId && sessionId !== gwStore.boundSessionId) {
        gwStore.setBoundSession(sessionId)
        useSessionStore.getState().setCurrentSessionId(sessionId)
      }
      break
    }

    case EventType.PermissionRequested: {
      // 权限审批请求
      const permPayload = payload.payload as PermissionRequestPayload | undefined
      if (permPayload) {
        chatStore.addPermissionRequest(permPayload)
      }
      break
    }

    case EventType.PermissionResolved: {
      // 权限审批结果
      const resolvedPayload = payload.payload as Record<string, unknown> | undefined
      const requestId = (resolvedPayload?.request_id as string) ?? ''
      if (requestId) {
        chatStore.removePermissionRequest(requestId)
      }
      break
    }

    case EventType.TokenUsage: {
      // Token 用量统计
      const usage = payload.payload as TokenUsage | undefined
      if (usage) {
        chatStore.updateTokenUsage(usage)
      }
      break
    }

    case EventType.Error: {
      // 错误事件
      const errorMsg = (payload.payload as string) ?? '未知错误'
      uiStore.showToast(errorMsg, 'error')
      chatStore.setGenerating(false)
      break
    }

    case EventType.StopReasonDecided: {
      // 运行终止
      const reasonPayload = payload.payload as Record<string, unknown> | undefined
      const reason = (reasonPayload?.reason as string) ?? ''
      chatStore.setStopReason(reason)
      chatStore.setGenerating(false)
      break
    }

    case EventType.PhaseChanged: {
      // 阶段切换
      const phasePayload = payload.payload as Record<string, unknown> | undefined
      const phase = (phasePayload?.to as string) ?? ''
      chatStore.setPhase(phase)
      break
    }

    case EventType.RunCanceled: {
      chatStore.setGenerating(false)
      uiStore.showToast('运行已取消', 'info')
      break
    }

    case EventType.ToolCallThinking: {
      // 思考过程（预留）
      break
    }

    case EventType.CompactStart: {
      uiStore.showToast('正在压缩上下文...', 'info')
      break
    }

    case EventType.CompactApplied: {
      uiStore.showToast('上下文压缩完成', 'success')
      break
    }

    case EventType.CompactError: {
      const errMsg = (payload.payload as string) ?? '压缩失败'
      uiStore.showToast(errMsg, 'error')
      break
    }

    case EventType.SkillActivated: {
      const skillPayload = payload.payload as Record<string, unknown> | undefined
      const skillId = (skillPayload?.skill_id as string) ?? ''
      uiStore.showToast(`技能已激活: ${skillId}`, 'success')
      break
    }

    case EventType.SkillDeactivated: {
      const skillPayload2 = payload.payload as Record<string, unknown> | undefined
      const skillId2 = (skillPayload2?.skill_id as string) ?? ''
      uiStore.showToast(`技能已停用: ${skillId2}`, 'info')
      break
    }

    case EventType.SkillMissing: {
      const skillPayload3 = payload.payload as Record<string, unknown> | undefined
      const skillId3 = (skillPayload3?.skill_id as string) ?? ''
      uiStore.showToast(`技能不可用: ${skillId3}`, 'error')
      break
    }

    case EventType.AssetSaved: {
      const assetPayload = payload.payload as Record<string, unknown> | undefined
      const assetPath = (assetPayload?.path as string) ?? ''
      if (assetPath) {
        uiStore.showToast(`文件已保存: ${assetPath}`, 'success')
        // 更新文件变更列表
        const currentChanges = useUIStore.getState().fileChanges
        const exists = currentChanges.find((c) => c.path === assetPath)
        if (!exists) {
          useUIStore.setState({
            fileChanges: [...currentChanges, {
              id: `fc_${Date.now()}`,
              path: assetPath,
              status: 'modified' as const,
              additions: 0,
              deletions: 0,
            }],
          })
        }
      }
      break
    }

    case EventType.AssetSaveFailed: {
      const failPayload = payload.payload as Record<string, unknown> | undefined
      const failPath = (failPayload?.path as string) ?? ''
      uiStore.showToast(`文件保存失败: ${failPath}`, 'error')
      break
    }

    case EventType.TodoUpdated: {
      // 预留：todo 列表更新
      break
    }

    case EventType.TodoConflict: {
      const conflictPayload = payload.payload as Record<string, unknown> | undefined
      const reason = (conflictPayload?.reason as string) ?? ''
      uiStore.showToast(`Todo 冲突: ${reason}`, 'error')
      break
    }

    case EventType.TodoSummaryInjected: {
      // 预留
      break
    }

    case EventType.ProgressEvaluated: {
      // 预留：进度评估
      break
    }

    case EventType.VerificationStarted: {
      chatStore.setPhase('verification')
      uiStore.showToast('验证开始', 'info')
      break
    }

    case EventType.VerificationStageFinished:
    case EventType.VerificationFinished:
    case EventType.VerificationCompleted: {
      // 验证阶段/全部完成
      break
    }

    case EventType.VerificationFailed: {
      const vFailPayload = payload.payload as string ?? '验证失败'
      uiStore.showToast(vFailPayload, 'error')
      break
    }

    case EventType.AcceptanceDecided: {
      // 验收决定
      break
    }

    default:
      // 未知事件类型，忽略
      break
  }
}
