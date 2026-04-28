import { useState } from 'react'
import { Copy, Check, Download, FileText } from 'lucide-react'

const langColors: Record<string, string> = {
  javascript: '#f7df1e',
  typescript: '#3178c6',
  go: '#00add8',
  python: '#3776ab',
  rust: '#dea584',
  html: '#e34c26',
  css: '#264de4',
  bash: '#4eaa25',
  json: '#292929',
  yaml: '#cb171e',
  markdown: '#083fa1',
}

interface CodeBlockProps {
  code: string
  language?: string
  filename?: string
}

export default function CodeBlock({ code, language = 'text', filename }: CodeBlockProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const lines = code.split('\n')

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <div style={styles.headerLeft}>
          <FileText size={12} />
          <span style={styles.filename}>{filename || language}</span>
          <span style={{ ...styles.langBadge, background: langColors[language] || 'var(--border-primary)' }}>
            {language}
          </span>
        </div>
        <div style={styles.headerRight}>
          <button style={styles.headerBtn} onClick={handleCopy} title="复制">
            {copied ? <Check size={14} /> : <Copy size={14} />}
          </button>
          <button style={styles.headerBtn} title="下载">
            <Download size={14} />
          </button>
        </div>
      </div>
      <div style={styles.codeWrap}>
        <pre style={styles.pre}>
          <code style={styles.code}>
            {lines.map((line, i) => (
              <div key={i} style={styles.line}>
                <span style={styles.lineNum}>{i + 1}</span>
                <span style={styles.lineContent}>{line || ' '}</span>
              </div>
            ))}
          </code>
        </pre>
      </div>
      <div style={styles.footer}>
        <button style={styles.actionBtn}>应用到文件</button>
        <button style={styles.actionBtn}>在 Diff 中查看</button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    margin: '8px 0',
    borderRadius: 'var(--radius-lg)',
    border: '1px solid var(--border-primary)',
    overflow: 'hidden',
    background: 'var(--code-bg)',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '8px 12px',
    borderBottom: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
  },
  headerLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    color: 'var(--text-secondary)',
    fontSize: 12,
  },
  filename: {
    fontWeight: 500,
    fontFamily: 'var(--font-mono)',
  },
  langBadge: {
    fontSize: 10,
    padding: '1px 6px',
    borderRadius: 'var(--radius-sm)',
    color: '#fff',
    fontWeight: 600,
    textTransform: 'uppercase',
  },
  headerRight: {
    display: 'flex',
    gap: 2,
  },
  headerBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 26,
    height: 26,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  codeWrap: {
    overflowX: 'auto',
    maxHeight: 400,
    overflowY: 'auto',
  },
  pre: {
    margin: 0,
    padding: '10px 0',
    fontFamily: 'var(--font-mono)',
    fontSize: 12.5,
    lineHeight: 1.7,
    background: 'var(--code-bg)',
  },
  code: {
    display: 'block',
  },
  line: {
    display: 'flex',
    padding: '0 12px',
  },
  lineNum: {
    width: 36,
    flexShrink: 0,
    textAlign: 'right',
    paddingRight: 12,
    color: 'var(--text-tertiary)',
    userSelect: 'none',
    fontSize: 11,
  },
  lineContent: {
    color: 'var(--text-primary)',
    whiteSpace: 'pre',
    tabSize: 2,
  },
  footer: {
    display: 'flex',
    gap: 8,
    padding: '8px 12px',
    borderTop: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
  },
  actionBtn: {
    padding: '5px 12px',
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    color: 'var(--text-secondary)',
    fontSize: 11,
    fontWeight: 500,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    transition: 'all 0.15s',
  },
}
