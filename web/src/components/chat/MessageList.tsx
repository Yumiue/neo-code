import { useEffect, useRef } from 'react'
import { useChatStore } from '@/store/useChatStore'
import MessageItem from './MessageItem'

/** 消息列表，自动滚动到底部 */
export default function MessageList() {
  const messages = useChatStore((s) => s.messages)
  const isGenerating = useChatStore((s) => s.isGenerating)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, isGenerating])

  const styles: Record<string, React.CSSProperties> = {
    container: {
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      padding: '16px 0',
      maxWidth: 800,
      margin: '0 auto',
      width: '100%',
    },
    empty: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100%',
      gap: 12,
      padding: '40px 20px',
    },
    emptyIcon: {
      width: 56,
      height: 56,
      borderRadius: 'var(--radius-xl)',
      background: 'var(--accent-muted)',
      color: 'var(--accent)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: 13,
      fontWeight: 700,
      fontFamily: 'var(--font-mono)',
    },
    emptyTitle: {
      fontSize: 18,
      fontWeight: 600,
      color: 'var(--text-primary)',
      margin: 0,
    },
    emptyDesc: {
      fontSize: 13,
      color: 'var(--text-secondary)',
      textAlign: 'center',
      maxWidth: 320,
      lineHeight: 1.6,
    },
    suggestions: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 8,
      justifyContent: 'center',
      marginTop: 8,
    },
    suggestionChip: {
      padding: '6px 14px',
      borderRadius: 'var(--radius-lg)',
      border: '1px solid var(--border-primary)',
      background: 'var(--bg-secondary)',
      color: 'var(--text-secondary)',
      fontSize: 12,
      cursor: 'pointer',
      fontFamily: 'var(--font-ui)',
      transition: 'all 0.15s',
    },
  }

  if (messages.length === 0) {
    return (
      <div style={styles.empty}>
        <div style={styles.emptyIcon}>NeoCode</div>
        <h3 style={styles.emptyTitle}>开始你的 AI 编程之旅</h3>
        <p style={styles.emptyDesc}>输入问题、描述需求或粘贴代码，我将帮你分析和处理。</p>
        <div style={styles.suggestions}>
          {['帮我重构这段代码', '解释一下这个报错', '生成一个 REST API', '优化数据库查询'].map((s) => (
            <button key={s} style={styles.suggestionChip} onClick={() => {}}>
              {s}
            </button>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div style={styles.container}>
      {messages.map((msg, i) => (
        <MessageItem key={msg.id} message={msg} isLast={i === messages.length - 1} />
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
