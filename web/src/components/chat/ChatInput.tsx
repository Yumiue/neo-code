import { useState, useRef, useEffect } from 'react'
import { useChatStore, createUserMessage } from '@/store/useChatStore'
import { useGatewayStore } from '@/store/useGatewayStore'
import { useSessionStore } from '@/store/useSessionStore'
import { gatewayAPI } from '@/api/gateway'
import { Send, Square, Paperclip, AtSign } from 'lucide-react'

/** 聊天输入框 */
export default function ChatInput() {
  const [text, setText] = useState('')
  const [rows, setRows] = useState(1)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const isGenerating = useChatStore((s) => s.isGenerating)
  const addMessage = useChatStore((s) => s.addMessage)
  const setGenerating = useChatStore((s) => s.setGenerating)
  const sessionId = useGatewayStore((s) => s.boundSessionId)

  useEffect(() => {
    const lines = text.split('\n').length
    setRows(Math.min(Math.max(lines, 1), 8))
  }, [text])

  async function handleSubmit() {
    const input = text.trim()
    if (!input || isGenerating) return

    setText('')
    addMessage(createUserMessage(input))
    setGenerating(true)

    try {
      const ack = await gatewayAPI.run({
        session_id: sessionId || undefined,
        input_text: input,
      })

      // 从 ack 响应中回写 session_id 和 run_id
      const gwStore = useGatewayStore.getState()
      if (ack.run_id) {
        gwStore.setCurrentRunId(ack.run_id)
      }
      if (ack.session_id && ack.session_id !== sessionId) {
        // 新会话：后端返回了真实 session_id
        gwStore.setBoundSession(ack.session_id)
        useSessionStore.getState().setCurrentSessionId(ack.session_id)
      }
    } catch (err) {
      setGenerating(false)
      console.error('Run failed:', err)
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit()
    }
  }

  async function handleCancel() {
    const runId = useGatewayStore.getState().currentRunId
    if (runId) {
      try {
        await gatewayAPI.cancel({ run_id: runId })
      } catch (err) {
        console.error('Cancel failed:', err)
      }
    }
    setGenerating(false)
  }

  return (
    <div style={styles.container}>
      <div style={styles.inputBox}>
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入指令或问题... (Shift+Enter 换行)"
          rows={rows}
          style={styles.textarea}
        />
        <div style={styles.toolbar}>
          <div style={styles.leftActions}>
            <button style={styles.toolBtn} title="附加文件">
              <Paperclip size={16} />
            </button>
            <button style={styles.toolBtn} title="引用上下文">
              <AtSign size={16} />
            </button>
          </div>
          <button
            style={{
              ...styles.sendBtn,
              opacity: text.trim() || isGenerating ? 1 : 0.4,
              cursor: text.trim() || isGenerating ? 'pointer' : 'default',
            }}
            onClick={isGenerating ? handleCancel : handleSubmit}
          >
            {isGenerating ? <Square size={16} /> : <Send size={16} />}
          </button>
        </div>
      </div>
      <div style={styles.hint}>
        NeoCode 可能会生成不准确的信息，请验证重要代码。
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    padding: '12px 16px 8px',
    flexShrink: 0,
    background: 'var(--bg-primary)',
  },
  inputBox: {
    border: '1px solid var(--border-primary)',
    borderRadius: 'var(--radius-lg)',
    background: 'var(--bg-secondary)',
    overflow: 'hidden',
    transition: 'border-color 0.2s',
  },
  textarea: {
    width: '100%',
    padding: '10px 12px',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-primary)',
    fontSize: 14,
    fontFamily: 'var(--font-ui)',
    lineHeight: 1.6,
    resize: 'none',
    outline: 'none',
    minHeight: 20,
    maxHeight: 200,
  },
  toolbar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '6px 8px',
    borderTop: '1px solid var(--border-primary)',
  },
  leftActions: {
    display: 'flex',
    gap: 2,
  },
  toolBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 28,
    height: 28,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  sendBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 32,
    height: 32,
    borderRadius: 'var(--radius-md)',
    border: 'none',
    background: 'var(--accent)',
    color: '#fff',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  hint: {
    fontSize: 11,
    color: 'var(--text-tertiary)',
    textAlign: 'center',
    marginTop: 6,
    paddingBottom: 4,
  },
}
