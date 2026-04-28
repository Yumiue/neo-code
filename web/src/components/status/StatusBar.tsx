import { useChatStore } from '@/store/useChatStore'
import { useUIStore } from '@/store/useUIStore'
import { useGatewayStore } from '@/store/useGatewayStore'
import { formatTokenCount } from '@/utils/format'
import { Sun, Moon, Wifi, WifiOff, Loader } from 'lucide-react'

/** 连接状态图标 */
function ConnectionIcon({ state }: { state: string }) {
  switch (state) {
    case 'connected':
      return <Wifi size={12} style={{ color: 'var(--success)' }} />
    case 'connecting':
      return <Loader size={12} style={{ color: 'var(--warning)' }} />
    case 'error':
      return <WifiOff size={12} style={{ color: 'var(--error)' }} />
    default:
      return <WifiOff size={12} />
  }
}

/** 底部状态栏 */
export default function StatusBar() {
  const tokenUsage = useChatStore((s) => s.tokenUsage)
  const theme = useUIStore((s) => s.theme)
  const setTheme = useUIStore((s) => s.setTheme)
  const connectionState = useGatewayStore((s) => s.connectionState)
  const authenticated = useGatewayStore((s) => s.authenticated)

  const totalTokens = tokenUsage ? tokenUsage.input_tokens + tokenUsage.output_tokens : 0
  const maxTokens = 8192

  return (
    <div style={styles.container}>
      <div style={styles.left}>
        <ConnectionIcon state={connectionState} />
        <span style={styles.connLabel}>
          {connectionState === 'connected' ? (authenticated ? '已连接' : '未认证') :
           connectionState === 'connecting' ? '连接中...' :
           connectionState === 'error' ? '连接失败' : '未连接'}
        </span>
      </div>
      <div style={styles.right}>
        {tokenUsage && (
          <>
            <span style={styles.tokenInfo}>
              Tokens: {formatTokenCount(totalTokens)} / {formatTokenCount(maxTokens)}
            </span>
            <span style={styles.divider} />
          </>
        )}
        <button
          style={styles.themeBtn}
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          title={theme === 'dark' ? '切换到浅色主题' : '切换到深色主题'}
        >
          {theme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
        </button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '4px 16px',
    borderTop: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    fontSize: 11,
    color: 'var(--text-tertiary)',
    flexShrink: 0,
    height: 28,
  },
  left: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  connLabel: {
    fontSize: 11,
    fontFamily: 'var(--font-ui)',
  },
  right: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  divider: {
    width: 1,
    height: 12,
    background: 'var(--border-primary)',
  },
  tokenInfo: {
    fontFamily: 'var(--font-mono)',
    fontSize: 11,
  },
  themeBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 22,
    height: 22,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
}
