import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import ChatPage from './pages/ChatPage'
import ConnectPage from './pages/ConnectPage'
import { useRuntime } from './context/RuntimeProvider'
import { ErrorBoundary } from './components/ErrorBoundary'

/** 加载/错误状态全屏遮罩 */
function LoadingScreen({ message }: { message?: string }) {
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: '100dvh',
      background: 'var(--bg-primary)',
      color: 'var(--text-tertiary)',
      fontSize: 14,
    }}>
      {message || '正在连接 Gateway...'}
    </div>
  )
}

/** Electron 错误状态恢复界面 */
function ElectronErrorScreen({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: '100dvh',
      background: 'var(--bg-primary)',
      color: 'var(--text-primary)',
      gap: 16,
      padding: 32,
    }}>
      <div style={{ fontSize: 16, fontWeight: 600 }}>Gateway 连接失败</div>
      <div style={{
        fontSize: 13,
        color: 'var(--text-tertiary)',
        maxWidth: 400,
        textAlign: 'center',
        wordBreak: 'break-word',
      }}>
        {error || '无法连接到 Gateway 服务'}
      </div>
      <button
        onClick={onRetry}
        style={{
          marginTop: 8,
          padding: '10px 32px',
          borderRadius: 'var(--radius-md)',
          border: 'none',
          background: 'var(--accent)',
          color: '#fff',
          fontSize: 14,
          fontWeight: 500,
          cursor: 'pointer',
        }}
      >
        重试连接
      </button>
    </div>
  )
}

function AppRoutes() {
  const { status, mode, error, retry, loadingMessage } = useRuntime()

  if (status === 'loading') {
    return <LoadingScreen message={loadingMessage} />
  }

  if (status === 'needs_config' && mode === 'browser') {
    return <ConnectPage />
  }

  if (status === 'connecting') {
    return <LoadingScreen message="正在连接 Gateway..." />
  }

  if (status === 'error' && mode === 'browser') {
    return <ConnectPage />
  }

  if (status === 'error' && mode === 'electron') {
    return <ElectronErrorScreen error={error} onRetry={retry} />
  }

  if (status !== 'connected') {
    return <LoadingScreen message="等待连接..." />
  }

  return (
    <Routes>
      <Route path="/" element={<ChatPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function App() {
  return (
    <ErrorBoundary>
      <HashRouter>
        <AppRoutes />
      </HashRouter>
    </ErrorBoundary>
  )
}

export default App
