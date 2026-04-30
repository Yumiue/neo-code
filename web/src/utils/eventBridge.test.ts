import { describe, it, expect, beforeEach } from 'vitest'
import { useChatStore } from '@/stores/useChatStore'
import { useGatewayStore } from '@/stores/useGatewayStore'
import { useSessionStore } from '@/stores/useSessionStore'
import { handleGatewayEvent } from './eventBridge'
import { EventType } from '@/api/protocol'

function createMockGatewayAPI() {
  return {} as any
}

beforeEach(() => {
  useChatStore.setState({
    messages: [],
    isGenerating: false,
    streamingMessageId: '',
    permissionRequests: [],
    tokenUsage: null,
    phase: '',
    stopReason: '',
  } as any)
  useGatewayStore.setState({
    connectionState: 'disconnected',
    currentRunId: '',
    token: '',
    authenticated: false,
  } as any)
})

describe('eventBridge', () => {
  it('AgentChunk adds assistant message and appends text', () => {
    const api = createMockGatewayAPI()
    handleGatewayEvent({
      type: EventType.AgentChunk,
      payload: { payload: { runtime_event_type: EventType.AgentChunk, payload: 'hello' } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    const msgs = useChatStore.getState().messages
    expect(msgs).toHaveLength(1)
    expect(msgs[0].role).toBe('assistant')
    expect(msgs[0].content).toBe('hello')
  })

  it('AgentChunk appends to existing streaming message', () => {
    const api = createMockGatewayAPI()
    const store = useChatStore.getState()
    store.addMessage({ id: 's1', role: 'assistant', content: 'He', type: 'text', timestamp: 1 })
    store.setStreamingMessageId('s1')

    handleGatewayEvent({
      type: EventType.AgentChunk,
      payload: { payload: { runtime_event_type: EventType.AgentChunk, payload: 'llo' } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    expect(useChatStore.getState().messages[0].content).toBe('Hello')
  })

  it('AgentDone finalizes message from parts array', () => {
    const api = createMockGatewayAPI()
    const store = useChatStore.getState()
    store.addMessage({ id: 's1', role: 'assistant', content: 'He', type: 'text', timestamp: 1 })
    store.setStreamingMessageId('s1')
    store.setGenerating(true)

    handleGatewayEvent({
      type: EventType.AgentDone,
      payload: { payload: { runtime_event_type: EventType.AgentDone, payload: { parts: [{ text: 'Hello world' }] } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    expect(useChatStore.getState().messages[0].content).toBe('Hello world')
    expect(useChatStore.getState().isGenerating).toBe(false)
    expect(useChatStore.getState().streamingMessageId).toBe('')
  })

  it('AgentDone falls back to content field when parts missing', () => {
    const api = createMockGatewayAPI()
    const store = useChatStore.getState()
    store.addMessage({ id: 's1', role: 'assistant', content: '', type: 'text', timestamp: 1 })
    store.setStreamingMessageId('s1')
    store.setGenerating(true)

    handleGatewayEvent({
      type: EventType.AgentDone,
      payload: { payload: { runtime_event_type: EventType.AgentDone, payload: { content: 'fallback' } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    expect(useChatStore.getState().messages[0].content).toBe('fallback')
  })

  it('InputNormalizes sets currentSessionId and currentRunId', () => {
    const api = createMockGatewayAPI()
    handleGatewayEvent({
      type: EventType.InputNormalized,
      payload: { payload: { runtime_event_type: EventType.InputNormalized, payload: { session_id: 'sess-1', run_id: 'run-1' } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    expect(useSessionStore.getState().currentSessionId).toBe('sess-1')
    expect(useGatewayStore.getState().currentRunId).toBe('run-1')
  })

  it('ToolStart adds a tool call message', () => {
    const api = createMockGatewayAPI()
    handleGatewayEvent({
      type: EventType.ToolStart,
      payload: { payload: { runtime_event_type: EventType.ToolStart, payload: { name: 'read_file', id: 'tc1', arguments: '{"path":"/a"}' } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    const msgs = useChatStore.getState().messages
    expect(msgs).toHaveLength(1)
    expect(msgs[0].type).toBe('tool_call')
    expect(msgs[0].toolName).toBe('read_file')
  })

  it('ToolResult updates an existing tool call message', () => {
    const api = createMockGatewayAPI()
    // 先触发 ToolStart 创建工具消息
    handleGatewayEvent({
      type: EventType.ToolStart,
      payload: { payload: { runtime_event_type: EventType.ToolStart, payload: { name: 'read_file', id: 'tc1', arguments: '{"path":"/a"}' } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    // 再触发 ToolResult 更新结果
    handleGatewayEvent({
      type: EventType.ToolResult,
      payload: { payload: { runtime_event_type: EventType.ToolResult, payload: { tool_call_id: 'tc1', content: 'file contents', is_error: false } } },
      session_id: 'sess-1',
      run_id: 'run-1',
    }, api)

    const msgs = useChatStore.getState().messages
    expect(msgs).toHaveLength(1)
    expect(msgs[0].role).toBe('tool')
    expect(msgs[0].toolResult).toBe('file contents')
    expect(msgs[0].toolStatus).toBe('done')
  })
})
