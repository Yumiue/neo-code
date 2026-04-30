import { memo, useState } from 'react'
import { type ChatMessage } from '@/stores/useChatStore'
import ToolCallCard from './ToolCallCard'
import CodeBlock from './CodeBlock'
import { Bot, ChevronRight } from 'lucide-react'

interface MessageItemProps {
  message: ChatMessage
  isLast?: boolean
}

/** 单条消息渲染 */
const MessageItem = memo(function MessageItem({ message, isLast = false }: MessageItemProps) {
  if (message.type === 'welcome') {
    return <WelcomeMessage message={message} />
  }

  if (message.type === 'thinking') {
    return <ThinkingMessage message={message} />
  }

  if (message.type === 'tool_call') {
    return <ToolCallCard message={message} />
  }

  if (message.type === 'code') {
    return (
      <AIMessage message={message} isLast={isLast}>
        <CodeBlock code={message.content} language={message.language || 'text'} filename={message.filename} />
      </AIMessage>
    )
  }

  if (message.role === 'user') {
    return <UserMessage message={message} />
  }

  return <AIMessage message={message} isLast={isLast} />
})

function UserMessage({ message }: { message: ChatMessage }) {
  return (
    <div style={styles.userRow} className="animate-slide-up">
      <div style={styles.userContent}>
        <div style={styles.userBubble}>{message.content}</div>
      </div>
    </div>
  )
}

function AIMessage({ message, isLast, children }: { message: ChatMessage; isLast: boolean; children?: React.ReactNode }) {
  return (
    <div style={styles.aiRow} className="animate-slide-up">
      <div style={styles.aiAvatar}>
        <Bot size={16} />
      </div>
      <div style={styles.aiContent}>
        {children || (
          <div style={styles.aiText}>
            {message.content}
            {isLast && !message.content && message.streaming && (
              <span style={styles.typing}>
                <span className="thinking-dot">.</span>
                <span className="thinking-dot">.</span>
                <span className="thinking-dot">.</span>
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function ThinkingMessage({ message }: { message: ChatMessage }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div style={styles.aiRow} className="animate-fade-in">
      <div style={{ ...styles.aiAvatar, background: 'var(--warning)', color: '#fff' }}>
        <Bot size={16} />
      </div>
      <div style={styles.aiContent}>
        <button style={styles.thinkingToggle} onClick={() => setExpanded(!expanded)}>
          <span style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s', display: 'flex' }}>
            <ChevronRight size={14} />
          </span>
          <span style={styles.thinkingLabel}>AI 思考过程</span>
        </button>
        {expanded && (
          <div style={styles.thinkingContent}>
            <pre style={{ margin: 0, fontFamily: 'var(--font-mono)', fontSize: 12, lineHeight: 1.7, whiteSpace: 'pre-wrap' }}>
              {message.content}
            </pre>
          </div>
        )}
      </div>
    </div>
  )
}

function WelcomeMessage({ message }: { message: ChatMessage }) {
  return (
    <div style={{ ...styles.aiRow, justifyContent: 'center' }} className="animate-slide-up">
      <div style={styles.welcomeCard}>
        <div style={styles.welcomeIcon}>NeoCode</div>
        <p style={styles.welcomeText}>{message.content}</p>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  userRow: {
    display: 'flex',
    justifyContent: 'flex-end',
    padding: '8px 0',
  },
  userContent: {
    maxWidth: '85%',
  },
  userBubble: {
    background: 'var(--user-msg-bg)',
    color: 'var(--text-primary)',
    padding: '10px 14px',
    borderRadius: 'var(--radius-lg)',
    fontSize: 14,
    lineHeight: 1.6,
    textWrap: 'pretty' as any,
  },
  aiRow: {
    display: 'flex',
    gap: 10,
    padding: '8px 0',
  },
  aiAvatar: {
    width: 28,
    height: 28,
    borderRadius: 'var(--radius-md)',
    background: 'var(--accent-muted)',
    color: 'var(--accent)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    marginTop: 2,
  },
  aiContent: {
    flex: 1,
    minWidth: 0,
  },
  aiText: {
    fontSize: 14,
    lineHeight: 1.7,
    color: 'var(--text-primary)',
    textWrap: 'pretty' as any,
  },
  typing: {
    marginLeft: 4,
    color: 'var(--text-tertiary)',
  },
  thinkingToggle: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    padding: '4px 8px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-secondary)',
    fontSize: 12,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    marginBottom: 8,
  },
  thinkingLabel: {
    fontWeight: 500,
  },
  thinkingContent: {
    padding: '10px 12px',
    borderRadius: 'var(--radius-md)',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-secondary)',
    marginBottom: 8,
  },
  welcomeCard: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 12,
    padding: '24px 32px',
    textAlign: 'center',
    maxWidth: 480,
  },
  welcomeIcon: {
    width: 48,
    height: 48,
    borderRadius: 'var(--radius-xl)',
    background: 'var(--accent-muted)',
    color: 'var(--accent)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 11,
    fontWeight: 700,
    fontFamily: 'var(--font-mono)',
  },
  welcomeText: {
    fontSize: 14,
    lineHeight: 1.7,
    color: 'var(--text-secondary)',
    margin: 0,
  },
}

export default MessageItem
