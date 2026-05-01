import { useEffect, useMemo, useState } from 'react'
import { type ChatMessage } from '@/stores/useChatStore'
import { Loader2, Wrench, CheckCircle2, XCircle, ChevronRight } from 'lucide-react'

interface ToolCallCardProps {
  message: ChatMessage
  /** 是否与上一条 AI/工具消息属于同一回合（同回合则压缩上下间距） */
  groupedWithPrev?: boolean
}

/** 工具调用 — 内联到 AI 回合的折叠行 */
export default function ToolCallCard({ message, groupedWithPrev = false }: ToolCallCardProps) {
  const isRunning = message.toolStatus === 'running'
  const isDone = message.toolStatus === 'done'
  const isError = message.toolStatus === 'error'

  const [expanded, setExpanded] = useState(isRunning)
  const [userToggled, setUserToggled] = useState(false)

  useEffect(() => {
    if (!userToggled && !isRunning) {
      setExpanded(false)
    }
  }, [isRunning, userToggled])

  const argsSummary = useMemo(() => parseArgsSummary(message.toolArgs), [message.toolArgs])
  const resultStats = useMemo(() => formatResultStats(message.toolResult), [message.toolResult])

  function toggle() {
    setUserToggled(true)
    setExpanded((v) => !v)
  }

  return (
    <div style={groupedWithPrev ? styles.rowGrouped : styles.row} className="animate-fade-in">
      <div style={styles.avatarSpacer} aria-hidden />
      <div style={styles.body}>
        <button style={styles.head} onClick={toggle} aria-expanded={expanded}>
          <span style={{ ...styles.chevron, transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}>
            <ChevronRight size={12} />
          </span>
          <Wrench size={12} style={styles.icon} />
          <span style={styles.toolName}>{message.toolName || 'tool'}</span>
          {isRunning && <Loader2 size={12} className="animate-spin" style={styles.iconMuted} />}
          {isDone && <CheckCircle2 size={12} style={styles.iconDone} />}
          {isError && <XCircle size={12} style={styles.iconError} />}
          {argsSummary && <span style={styles.summary}>{argsSummary}</span>}
          {resultStats && <span style={styles.stats}>· {resultStats}</span>}
        </button>
        {expanded && (
          <div style={styles.detail}>
            {message.toolArgs && (
              <div>
                <div style={styles.sectionLabel}>参数</div>
                <pre style={styles.pre}>{message.toolArgs}</pre>
              </div>
            )}
            {message.toolResult && (
              <div>
                <div style={styles.sectionLabel}>结果</div>
                <pre style={styles.pre}>{message.toolResult}</pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function parseArgsSummary(args?: string): string {
  if (!args) return ''
  const trimmed = args.trim()
  if (!trimmed) return ''
  try {
    const obj = JSON.parse(trimmed)
    if (obj && typeof obj === 'object' && !Array.isArray(obj)) {
      for (const key of Object.keys(obj)) {
        const v = (obj as Record<string, unknown>)[key]
        if (typeof v === 'string' && v.length > 0) return truncate(v, 80)
        if (typeof v === 'number' || typeof v === 'boolean') return String(v)
      }
      return ''
    }
    return truncate(trimmed, 80)
  } catch {
    return truncate(trimmed, 80)
  }
}

function formatResultStats(result?: string): string {
  if (!result) return ''
  const lines = result.split(/\r?\n/).length
  if (lines >= 3) return `${lines} lines`
  return `${result.length} chars`
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s
  return s.slice(0, n) + '…'
}

const styles: Record<string, React.CSSProperties> = {
  row: {
    display: 'flex',
    gap: 10,
    padding: '2px 0',
  },
  rowGrouped: {
    display: 'flex',
    gap: 10,
    padding: '0',
  },
  avatarSpacer: {
    width: 28,
    flexShrink: 0,
  },
  body: {
    flex: 1,
    minWidth: 0,
    borderLeft: '2px solid var(--border-primary)',
    paddingLeft: 10,
  },
  head: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    width: '100%',
    padding: '4px 0',
    background: 'transparent',
    border: 'none',
    cursor: 'pointer',
    color: 'var(--text-secondary)',
    fontFamily: 'var(--font-ui)',
    textAlign: 'left',
  },
  chevron: {
    display: 'flex',
    color: 'var(--text-tertiary)',
    transition: 'transform 0.15s',
    flexShrink: 0,
  },
  icon: {
    color: 'var(--text-tertiary)',
    flexShrink: 0,
  },
  iconMuted: {
    color: 'var(--text-tertiary)',
    flexShrink: 0,
  },
  iconDone: {
    color: 'var(--success)',
    flexShrink: 0,
  },
  iconError: {
    color: 'var(--error)',
    flexShrink: 0,
  },
  toolName: {
    fontSize: 12,
    fontWeight: 500,
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    flexShrink: 0,
  },
  summary: {
    fontSize: 12,
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    minWidth: 0,
    marginLeft: 2,
  },
  stats: {
    fontSize: 11,
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
    flexShrink: 0,
  },
  detail: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    padding: '4px 0 8px',
  },
  sectionLabel: {
    fontSize: 10,
    color: 'var(--text-tertiary)',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
    marginBottom: 4,
    fontFamily: 'var(--font-ui)',
  },
  pre: {
    fontSize: 11,
    fontFamily: 'var(--font-mono)',
    color: 'var(--text-secondary)',
    background: 'var(--bg-tertiary)',
    padding: '8px 10px',
    borderRadius: 'var(--radius-sm)',
    margin: 0,
    overflow: 'auto',
    maxHeight: 280,
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-all',
    lineHeight: 1.5,
  },
}
