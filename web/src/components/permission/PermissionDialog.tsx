import { useChatStore } from '@/stores/useChatStore'
import { useGatewayAPI } from '@/context/RuntimeProvider'
import { PermissionDecision } from '@/api/protocol'
import { Shield, X, Check } from 'lucide-react'

/** 权限审批弹窗 */
export default function PermissionDialog() {
  const maybeAPI = useGatewayAPI()
  const permissionRequests = useChatStore((s) => s.permissionRequests)

  if (permissionRequests.length === 0) return null
  if (!maybeAPI) return null
  const gatewayAPI = maybeAPI

  const current = permissionRequests[0]

  async function handleDecision(decision: string) {
    try {
      await gatewayAPI.resolvePermission({
        request_id: current.request_id,
        decision,
      })
    } catch (err) {
      console.error('Resolve permission failed:', err)
    }
  }

  return (
    <div style={styles.overlay}>
      <div style={styles.modal}>
        {/* 标题 */}
        <div style={styles.header}>
          <Shield size={20} style={{ color: 'var(--warning)' }} />
          <h3 style={styles.title}>权限请求</h3>
        </div>

        {/* 详情 */}
        <div style={styles.details}>
          <div style={styles.detailRow}>
            <span style={styles.detailLabel}>工具</span>
            <span style={styles.detailValue}>{current.tool_name}</span>
          </div>
          <div style={styles.detailRow}>
            <span style={styles.detailLabel}>操作</span>
            <span style={styles.detailValue}>{current.operation}</span>
          </div>
          <div style={styles.detailRow}>
            <span style={styles.detailLabel}>目标</span>
            <span style={{ ...styles.detailValue, fontSize: 11 }}>{current.target}</span>
          </div>
        </div>

        {/* 按钮 */}
        <div style={styles.buttons}>
          <button
            onClick={() => handleDecision(PermissionDecision.Reject)}
            style={{ ...styles.btn, ...styles.btnReject }}
          >
            <X size={14} /> 拒绝
          </button>
          <button
            onClick={() => handleDecision(PermissionDecision.AllowOnce)}
            style={{ ...styles.btn, ...styles.btnPrimary }}
          >
            <Check size={14} /> 允许一次
          </button>
          <button
            onClick={() => handleDecision(PermissionDecision.AllowSession)}
            style={{ ...styles.btn, ...styles.btnSecondary }}
          >
            <Check size={14} /> 本会话允许
          </button>
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 200,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'rgba(0,0,0,0.5)',
  },
  modal: {
    width: 420,
    borderRadius: 'var(--radius-lg)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    padding: 20,
    boxShadow: 'var(--shadow-3)',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 12,
  },
  title: {
    fontSize: 14,
    fontWeight: 600,
    color: 'var(--text-primary)',
    margin: 0,
  },
  details: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    marginBottom: 16,
    fontSize: 13,
  },
  detailRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  detailLabel: {
    color: 'var(--text-tertiary)',
  },
  detailValue: {
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    fontWeight: 500,
  },
  buttons: {
    display: 'flex',
    gap: 8,
  },
  btn: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    padding: '8px 12px',
    borderRadius: 'var(--radius-md)',
    fontSize: 13,
    fontFamily: 'var(--font-ui)',
    cursor: 'pointer',
    transition: 'all 0.15s',
    border: '1px solid var(--border-primary)',
  },
  btnReject: {
    background: 'var(--bg-tertiary)',
    color: 'var(--text-primary)',
  },
  btnPrimary: {
    background: 'var(--accent)',
    color: '#fff',
    border: 'none',
  },
  btnSecondary: {
    background: 'var(--bg-active)',
    color: 'var(--text-primary)',
  },
}
