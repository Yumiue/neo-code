import { describe, it, expect, beforeEach } from 'vitest'
import { useChatStore } from './useChatStore'

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
})

describe('useChatStore', () => {
  it('addMessage appends a message', () => {
    useChatStore.getState().addMessage({
      id: 'msg-1',
      role: 'user',
      content: 'hello',
      type: 'text',
      timestamp: 1,
    })
    expect(useChatStore.getState().messages).toHaveLength(1)
    expect(useChatStore.getState().messages[0].content).toBe('hello')
  })

  it('appendChunk concatenates to streaming message', () => {
    const store = useChatStore.getState()
    store.addMessage({ id: 'stream-1', role: 'assistant', content: 'Hel', type: 'text', timestamp: 1 })
    store.setStreamingMessageId('stream-1')
    store.appendChunk('lo')
    expect(useChatStore.getState().messages[0].content).toBe('Hello')
  })

  it('finalizeMessage replaces content for streaming id', () => {
    const store = useChatStore.getState()
    store.addMessage({ id: 'stream-1', role: 'assistant', content: 'partial', type: 'text', timestamp: 1 })
    store.setStreamingMessageId('stream-1')
    store.finalizeMessage('stream-1', 'final text')
    expect(useChatStore.getState().messages[0].content).toBe('final text')
    expect(useChatStore.getState().streamingMessageId).toBe('')
  })

  it('clearMessages removes all messages', () => {
    const store = useChatStore.getState()
    store.addMessage({ id: 'msg-1', role: 'user', content: 'hi', type: 'text', timestamp: 1 })
    store.clearMessages()
    expect(useChatStore.getState().messages).toHaveLength(0)
  })

  it('setGenerating toggles generation state', () => {
    useChatStore.getState().setGenerating(true)
    expect(useChatStore.getState().isGenerating).toBe(true)
    useChatStore.getState().setGenerating(false)
    expect(useChatStore.getState().isGenerating).toBe(false)
  })
})
