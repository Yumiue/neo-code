import { useEffect } from 'react'
import { useUIStore, type Toast } from '@/stores/useUIStore'
import { X, AlertCircle, CheckCircle, Info } from 'lucide-react'

/** Toast 图标映射 */
const iconMap: Record<Toast['type'], typeof Info> = {
  info: Info,
  error: AlertCircle,
  success: CheckCircle,
}

/** Toast 颜色映射 */
const colorMap: Record<Toast['type'], string> = {
  info: 'var(--accent)',
  error: 'var(--error)',
  success: 'var(--success)',
}

/** 单条 Toast */
function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const Icon = iconMap[toast.type]

  useEffect(() => {
    const timer = setTimeout(onDismiss, 4000)
    return () => clearTimeout(timer)
  }, [onDismiss])

  return (
    <div style={styles.item} className="animate-slide-up">
      <Icon size={16} style={{ color: colorMap[toast.type], flexShrink: 0 }} />
      <span style={styles.message}>{toast.message}</span>
      <button style={styles.closeBtn} onClick={onDismiss}>
        <X size={12} />
      </button>
    </div>
  )
}

/** Toast 容器：消费 useUIStore.toasts 并渲染浮动通知 */
export default function ToastContainer() {
  const toasts = useUIStore((s) => s.toasts)
  const dismissToast = useUIStore((s) => s.dismissToast)

  if (toasts.length === 0) return null

  return (
    <div style={styles.container}>
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} onDismiss={() => dismissToast(t.id)} />
      ))}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    position: 'fixed',
    bottom: 48,
    right: 20,
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    zIndex: 1000,
    maxWidth: 380,
  },
  item: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    padding: '10px 14px',
    borderRadius: 'var(--radius-lg)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
    boxShadow: 'var(--shadow-3)',
    fontSize: 13,
    fontFamily: 'var(--font-ui)',
    color: 'var(--text-primary)',
  },
  message: {
    flex: 1,
    lineHeight: 1.4,
  },
  closeBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 20,
    height: 20,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    flexShrink: 0,
  },
}
