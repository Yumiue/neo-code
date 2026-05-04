import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import CodeBlock from './CodeBlock'

interface MarkdownContentProps {
  content: string
  streaming?: boolean
}

/** 轻量级代码块：融入文本流的行内样式 */
function InlineCodeBlock({ code }: { code: string }) {
  return (
    <code
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: '0.9em',
        padding: '0.15em 0.35em',
        borderRadius: 'var(--radius-sm)',
        background: 'var(--bg-tertiary)',
        color: 'var(--text-primary)',
      }}
    >
      {code}
    </code>
  )
}

/** 将代码块映射到 CodeBlock 组件 */
function CodeComponent({
  inline,
  className,
  children,
  ...props
}: {
  inline?: boolean
  className?: string
  children?: React.ReactNode
}) {
  if (inline) {
    return (
      <code className="markdown-inline-code" {...props}>
        {children}
      </code>
    )
  }

  const match = /language-(\w+)/.exec(className || '')
  const language = match ? match[1] : 'text'
  const code = String(children).replace(/\n$/, '')
  const lines = code.split('\n')

  // 单行且无明确语言的代码块降级为轻量渲染
  if (lines.length <= 1 && language === 'text') {
    return <InlineCodeBlock code={code} />
  }

  return <CodeBlock code={code} language={language} />
}

/** Markdown 渲染器，支持 GFM；流式输出时降级为纯文本 */
export default function MarkdownContent({ content, streaming }: MarkdownContentProps) {
  if (streaming) {
    return (
      <div className="markdown-body" style={{ whiteSpace: 'pre-wrap' }}>
        {content}
      </div>
    )
  }

  return (
    <div className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code: CodeComponent as any,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
