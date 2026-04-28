import { useState, useRef, useEffect } from 'react'
import { useUIStore } from '@/store/useUIStore'
import { useSessionStore } from '@/store/useSessionStore'
import MessageList from './MessageList'
import ChatInput from './ChatInput'
import ModelSelector from './ModelSelector'
import {
  PanelRightOpen,
  FileDiff,
  FolderTree,
  MoreHorizontal,
  Edit3,
} from 'lucide-react'

/** 聊天主区域 */
export default function ChatPanel() {
  const sidebarOpen = useUIStore((s) => s.sidebarOpen)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const changesPanelOpen = useUIStore((s) => s.changesPanelOpen)
  const fileTreePanelOpen = useUIStore((s) => s.fileTreePanelOpen)
  const toggleChangesPanel = useUIStore((s) => s.toggleChangesPanel)
  const toggleFileTreePanel = useUIStore((s) => s.toggleFileTreePanel)

  const currentSessionId = useSessionStore((s) => s.currentSessionId)
  const projects = useSessionStore((s) => s.projects)

  const [editingTitle, setEditingTitle] = useState(false)
  const [moreMenuOpen, setMoreMenuOpen] = useState(false)
  const titleRef = useRef<HTMLDivElement>(null)
  const moreMenuRef = useRef<HTMLDivElement>(null)

  // Find current session title
  const currentSession = projects.flatMap((p) => p.sessions).find((s) => s.id === currentSessionId)
  const title = currentSession?.title || '新对话'

  useEffect(() => {
    function handleClick(event: MouseEvent) {
      if (moreMenuRef.current && !moreMenuRef.current.contains(event.target as Node)) {
        setMoreMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  useEffect(() => {
    if (editingTitle && titleRef.current) {
      titleRef.current.focus()
      const range = document.createRange()
      range.selectNodeContents(titleRef.current)
      const selection = window.getSelection()
      if (selection) {
        selection.removeAllRanges()
        selection.addRange(range)
      }
    }
  }, [editingTitle])

  const handleTitleSave = () => {
    const newTitle = titleRef.current?.innerText.trim()
    if (newTitle && newTitle !== title) {
      // TODO: dispatch rename action
    }
    setEditingTitle(false)
  }

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <div style={styles.headerLeft}>
          {!sidebarOpen && (
            <button style={styles.expandSidebarBtn} onClick={toggleSidebar} title="展开侧边栏">
              <PanelRightOpen size={16} />
            </button>
          )}
          {editingTitle ? (
            <div
              ref={titleRef}
              contentEditable
              suppressContentEditableWarning
              style={styles.titleEditable}
              onBlur={handleTitleSave}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleTitleSave() } }}
            >
              {title}
            </div>
          ) : (
            <div style={styles.titleRow} onClick={() => setEditingTitle(true)}>
              <h2 style={styles.title}>{title}</h2>
              <span style={styles.editHint}><Edit3 size={12} /></span>
            </div>
          )}
        </div>
        <div style={styles.headerRight}>
          <ModelSelector />
          <div ref={moreMenuRef} style={{ position: 'relative' }}>
            <button
              style={{
                ...styles.headerBtn,
                color: moreMenuOpen ? 'var(--text-primary)' : 'var(--text-tertiary)',
                background: moreMenuOpen ? 'var(--bg-hover)' : 'transparent',
              }}
              title="更多"
              onClick={() => setMoreMenuOpen(!moreMenuOpen)}
            >
              <MoreHorizontal size={16} />
            </button>
            {moreMenuOpen && (
              <div style={styles.moreMenu} className="animate-slide-up">
                <button style={styles.moreMenuItem} onClick={() => { setMoreMenuOpen(false); setEditingTitle(true) }}>
                  重命名会话
                </button>
                <button style={styles.moreMenuItem} onClick={() => { setMoreMenuOpen(false) }}>
                  归档会话
                </button>
              </div>
            )}
          </div>
          <button
            style={{ ...styles.headerBtn, color: changesPanelOpen ? 'var(--accent)' : 'var(--text-tertiary)' }}
            title="文件更改"
            onClick={toggleChangesPanel}
          >
            <FileDiff size={16} />
          </button>
          <button
            style={{ ...styles.headerBtn, color: fileTreePanelOpen ? 'var(--accent)' : 'var(--text-tertiary)' }}
            title="文件目录"
            onClick={toggleFileTreePanel}
          >
            <FolderTree size={16} />
          </button>
        </div>
      </div>

      {/* Messages */}
      <div style={styles.messagesArea}>
        <MessageList />
      </div>

      {/* Input */}
      <ChatInput />
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100%',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '10px 16px',
    borderBottom: '1px solid var(--border-primary)',
    flexShrink: 0,
    gap: 12,
  },
  headerLeft: {
    minWidth: 0,
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  expandSidebarBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 28,
    height: 28,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    flexShrink: 0,
  },
  titleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    cursor: 'text',
    padding: '2px 6px',
    marginLeft: -6,
    borderRadius: 'var(--radius-sm)',
    transition: 'background 0.15s',
  },
  title: {
    fontSize: 15,
    fontWeight: 600,
    color: 'var(--text-primary)',
    margin: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    fontFamily: 'var(--font-ui)',
  },
  editHint: {
    color: 'var(--text-tertiary)',
    opacity: 0,
    transition: 'opacity 0.15s',
  },
  titleEditable: {
    fontSize: 15,
    fontWeight: 600,
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-ui)',
    padding: '2px 6px',
    borderRadius: 'var(--radius-sm)',
    outline: 'none',
    border: '1px solid var(--accent)',
    background: 'var(--bg-primary)',
    minWidth: 200,
  },
  headerRight: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    flexShrink: 0,
  },
  headerBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 30,
    height: 30,
    borderRadius: 'var(--radius-md)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  moreMenu: {
    position: 'absolute',
    top: 'calc(100% + 6px)',
    right: 0,
    width: 142,
    padding: 4,
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
    boxShadow: 'var(--shadow-3)',
    zIndex: 60,
  },
  moreMenuItem: {
    display: 'block',
    width: '100%',
    padding: '7px 9px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-primary)',
    fontSize: 12,
    fontFamily: 'var(--font-ui)',
    textAlign: 'left',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  messagesArea: {
    flex: 1,
    overflowY: 'auto',
    overflowX: 'hidden',
    padding: '0 16px',
  },
}
