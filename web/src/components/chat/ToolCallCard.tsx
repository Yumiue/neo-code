import { type ChatMessage } from '@/store/useChatStore'
import { Loader2, Wrench, CheckCircle2, XCircle } from 'lucide-react'

interface ToolCallCardProps {
  message: ChatMessage
}

/** 工具调用卡片 */
export default function ToolCallCard({ message }: ToolCallCardProps) {
  const isRunning = message.toolStatus === 'running'
  const isDone = message.toolStatus === 'done'
  const isError = message.toolStatus === 'error'

  return (
    <div style={styles.container}>
      {/* 工具名称和状态 */}
      <div style={styles.header}>
        <Wrench size={14} style={{ color: 'var(--text-tertiary)' }} />
        <span style={styles.toolName}>{message.toolName}</span>
        {isRunning && <Loader2 size={14} className="animate-spin" style={{ color: 'var(--text-tertiary)' }} />}
        {isDone && <CheckCircle2 size={14} style={{ color: 'var(--success)' }} />}
        {isError && <XCircle size={14} style={{ color: 'var(--error)' }} />}
      </div>

      {/* 工具参数（折叠） */}
      {message.toolArgs && (
        <details style={styles.details}>
          <summary style={styles.summary}>参数</summary>
          <pre style={styles.pre}>{message.toolArgs}</pre>
        </details>
      )}

      {/* 工具结果 */}
      {message.toolResult && (
        <div style={styles.result}>{message.toolResult}</div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    borderRadius: 'var(--radius-lg)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    padding: 12,
    fontSize: 13,
    marginLeft: 38,
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 8,
  },
  toolName: {
    fontWeight: 500,
    color: 'var(--text-primary)',
  },
  details: {
    marginBottom: 8,
  },
  summary: {
    cursor: 'pointer',
    fontSize: 11,
    color: 'var(--text-tertiary)',
    userSelect: 'none',
  },
  pre: {
    marginTop: 4,
    overflowX: 'auto',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-tertiary)',
    padding: 8,
    fontSize: 11,
    fontFamily: 'var(--font-mono)',
    color: 'var(--text-secondary)',
  },
  result: {
    marginTop: 8,
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-tertiary)',
    padding: 8,
    fontSize: 11,
    fontFamily: 'var(--font-mono)',
    overflowX: 'auto',
    whiteSpace: 'pre-wrap',
    color: 'var(--text-secondary)',
  },
}
