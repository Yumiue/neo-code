import { useEffect } from 'react'
import Sidebar from './Sidebar'
import ChatPanel from '@/components/chat/ChatPanel'
import FileChangePanel from '@/components/panels/FileChangePanel'
import FileTreePanel from '@/components/panels/FileTreePanel'
import StatusBar from '@/components/status/StatusBar'
import PermissionDialog from '@/components/permission/PermissionDialog'
import ToastContainer from '@/components/ui/ToastContainer'
import { useUIStore } from '@/store/useUIStore'
import { useSessionStore } from '@/store/useSessionStore'
import { useGatewayStore } from '@/store/useGatewayStore'
import { createSSEClient, type SSEClient } from '@/api/sseClient'
import { gatewayAPI } from '@/api/gateway'
import { handleGatewayEvent } from '@/utils/eventBridge'

/** 三栏布局外壳 */
let sseClient: SSEClient | null = null

export default function AppLayout() {
  const theme = useUIStore((s) => s.theme)
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)
  const sidebarWidth = useUIStore((s) => s.sidebarWidth)
  const changesPanelOpen = useUIStore((s) => s.changesPanelOpen)
  const changesPanelWidth = useUIStore((s) => s.changesPanelWidth)
  const fileTreePanelOpen = useUIStore((s) => s.fileTreePanelOpen)
  const fileTreePanelWidth = useUIStore((s) => s.fileTreePanelWidth)
  const setConnectionState = useGatewayStore((s) => s.setConnectionState)
  const setToken = useGatewayStore((s) => s.setToken)
  const setAuthenticated = useGatewayStore((s) => s.setAuthenticated)
  const fetchSessions = useSessionStore((s) => s.fetchSessions)

  // 初始化 SSE 连接和认证
  useEffect(() => {
    if (!sseClient) {
      sseClient = createSSEClient()
    }

    const unsubState = sseClient.onStateChange(setConnectionState)
    const unsubEvent = sseClient.onEvent(handleGatewayEvent)

    async function connect() {
      try {
        let token = ''
        if (window.electronAPI) {
          token = await window.electronAPI.getToken()
        }
        setToken(token)
        await gatewayAPI.authenticate(token)
        setAuthenticated(true)
        sseClient!.setToken(token)
        sseClient!.connect()

        // 认证成功后拉取会话列表
        await fetchSessions()
      } catch (err) {
        console.error('Gateway connection failed:', err)
        setAuthenticated(false)
      }
    }

    connect()

    return () => {
      unsubState()
      unsubEvent()
    }
  }, [setConnectionState, setToken, setAuthenticated, fetchSessions])

  // 主题初始化
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  return (
    <div style={layoutStyles.container}>
      <div style={layoutStyles.workspace}>
        {sidebarOpen ? (
          <div style={{ ...layoutStyles.sidebar, width: sidebarWidth }}>
            <Sidebar />
          </div>
        ) : (
          <div style={layoutStyles.sidebarCollapsed}>
            <Sidebar collapsed />
          </div>
        )}
        <div style={layoutStyles.main}>
          <ChatPanel />
        </div>
        {changesPanelOpen && (
          <div style={{ ...layoutStyles.rightPanel, width: `min(${changesPanelWidth}px, 24vw)` }}>
            <FileChangePanel />
          </div>
        )}
        {fileTreePanelOpen && (
          <div style={{ ...layoutStyles.rightPanel, width: `min(${fileTreePanelWidth}px, 22vw)` }}>
            <FileTreePanel />
          </div>
        )}
      </div>
      <StatusBar />
      <PermissionDialog />
      <ToastContainer />
    </div>
  )
}

const layoutStyles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100vh',
    width: '100vw',
    overflow: 'hidden',
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
  },
  workspace: {
    flex: 1,
    minHeight: 0,
    display: 'flex',
    overflow: 'hidden',
  },
  sidebar: {
    flexShrink: 0,
    borderRight: '1px solid var(--border-primary)',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  sidebarCollapsed: {
    width: 44,
    flexShrink: 0,
    borderRight: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 8,
    gap: 4,
  },
  main: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  rightPanel: {
    flexShrink: 0,
    borderLeft: '1px solid var(--border-primary)',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
}
