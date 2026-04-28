import { useState } from 'react'
import { useSessionStore } from '@/store/useSessionStore'
import { useUIStore } from '@/store/useUIStore'
import {
  Plus,
  Search,
  PanelLeft,
  Filter,
  MessageSquare,
  Folder,
  ChevronRight,
  X,
  Server,
  Cpu,
  Blocks,
} from 'lucide-react'

interface SidebarProps {
  collapsed?: boolean
}

/** 左侧栏：会话列表 */
export default function Sidebar({ collapsed }: SidebarProps) {
  const projects = useSessionStore((s) => s.projects)
  const currentSessionId = useSessionStore((s) => s.currentSessionId)
  const switchSession = useSessionStore((s) => s.switchSession)
  const createSession = useSessionStore((s) => s.createSession)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const searchQuery = useUIStore((s) => s.searchQuery)
  const setSearchQuery = useUIStore((s) => s.setSearchQuery)
  const setCurrentProjectId = useSessionStore((s) => s.setCurrentProjectId)

  const [expandedProjects, setExpandedProjects] = useState<Set<string>>(new Set())
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; sessionId: string } | null>(null)
  const [mcpModalOpen, setMcpModalOpen] = useState(false)
  const [skillModalOpen, setSkillModalOpen] = useState(false)
  const [providerModalOpen, setProviderModalOpen] = useState(false)

  const toggleProject = (projectId: string) => {
    setExpandedProjects((prev) => {
      const next = new Set(prev)
      if (next.has(projectId)) next.delete(projectId)
      else next.add(projectId)
      return next
    })
  }

  const filteredProjects = searchQuery.trim()
    ? projects.map((p) => {
        const projectMatches = p.name.toLowerCase().includes(searchQuery.trim().toLowerCase())
        return {
          ...p,
          sessions: projectMatches
            ? p.sessions
            : p.sessions.filter((s) => s.title.toLowerCase().includes(searchQuery.trim().toLowerCase())),
        }
      }).filter((p) => p.sessions.length > 0)
    : projects

  async function handleSelectSession(sessionId: string, projectId: string) {
    setCurrentProjectId(projectId)
    try {
      await switchSession(sessionId)
    } catch (err) {
      console.error('Switch session failed:', err)
    }
  }

  async function handleNewSession() {
    try {
      await createSession()
    } catch (err) {
      console.error('Create session failed:', err)
    }
  }

  // Collapsed sidebar strip
  if (collapsed) {
    return (
      <>
        <button style={styles.collapseBtn} onClick={toggleSidebar} title="展开侧边栏">
          <PanelLeft size={16} />
        </button>
        <button style={styles.collapseBtn} onClick={handleNewSession} title="新对话">
          <Plus size={16} />
        </button>
      </>
    )
  }

  return (
    <div style={styles.container}>
      {/* Collapse Toggle */}
      <button style={styles.collapseBtn} onClick={toggleSidebar} title="收起侧边栏">
        <PanelLeft size={14} />
      </button>

      {/* Top Actions */}
      <div style={styles.topActions}>
        <button style={styles.newChatBtn} onClick={handleNewSession}>
          <Plus size={16} />
          <span>新对话</span>
          <kbd style={styles.shortcut}>Ctrl+N</kbd>
        </button>
        <div style={styles.actionGrid}>
          <ActionButton icon={<Blocks size={16} />} label="MCP" onClick={() => setMcpModalOpen(true)} />
          <ActionButton icon={<Cpu size={16} />} label="Skill" onClick={() => setSkillModalOpen(true)} />
          <ActionButton icon={<Server size={16} />} label="供应商" onClick={() => setProviderModalOpen(true)} />
        </div>
      </div>

      {/* Session List Header */}
      <div style={styles.listHeader}>
        <span style={styles.listTitle}>项目</span>
        <div style={styles.searchBox}>
          <Search size={12} />
          <input
            style={styles.searchInput}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="搜索项目或会话"
          />
        </div>
        <div style={styles.listActions}>
          <button style={styles.iconBtn} title="筛选">
            <Filter size={14} />
          </button>
        </div>
      </div>

      {/* Session List */}
      <div style={styles.scrollArea}>
        {filteredProjects.map((project) => (
          <div key={project.id} style={styles.projectGroup}>
            <button style={styles.projectHeader} onClick={() => toggleProject(project.id)}>
              <span style={{ ...styles.chevron, transform: expandedProjects.has(project.id) ? 'rotate(90deg)' : 'rotate(0deg)' }}>
                <ChevronRight size={14} />
              </span>
              <Folder size={14} />
              <span style={styles.projectName}>{project.name}</span>
            </button>
            {expandedProjects.has(project.id) && (
              <div style={styles.sessionsList}>
                {project.sessions.map((session) => (
                  <SessionItem
                    key={session.id}
                    session={session}
                    isActive={currentSessionId === session.id}
                    onClick={() => handleSelectSession(session.id, project.id)}
                    onContextMenu={(e) => {
                      e.preventDefault()
                      setContextMenu({ x: e.clientX, y: e.clientY, sessionId: session.id })
                    }}
                  />
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Context Menu */}
      {contextMenu && (
        <>
          <div style={styles.overlay} onClick={() => setContextMenu(null)} />
          <div style={{ ...styles.contextMenu, left: contextMenu.x, top: contextMenu.y }}>
            <button style={styles.contextItem} onClick={() => setContextMenu(null)}>重命名</button>
            <button style={styles.contextItem} onClick={() => setContextMenu(null)}>归档</button>
            <div style={styles.contextDivider} />
            <button style={{ ...styles.contextItem, color: 'var(--error)' }} onClick={() => setContextMenu(null)}>删除</button>
          </div>
        </>
      )}

      {mcpModalOpen && <McpModal onClose={() => setMcpModalOpen(false)} />}
      {skillModalOpen && <SkillModal onClose={() => setSkillModalOpen(false)} />}
      {providerModalOpen && <ProviderModal onClose={() => setProviderModalOpen(false)} />}
    </div>
  )
}

function ActionButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button style={styles.actionBtn} onClick={onClick}>
      {icon}
      <span style={styles.actionLabel}>{label}</span>
    </button>
  )
}

function SessionItem({
  session,
  isActive,
  onClick,
  onContextMenu,
}: {
  session: { id: string; title: string; time: string; messageCount: number }
  isActive: boolean
  onClick: () => void
  onContextMenu: (e: React.MouseEvent) => void
}) {
  return (
    <button
      style={{ ...styles.sessionItem, background: isActive ? 'var(--bg-active)' : 'transparent' }}
      onClick={onClick}
      onContextMenu={onContextMenu}
    >
      <div style={styles.sessionRow}>
        <MessageSquare size={14} />
        <span style={styles.sessionTitle} title={session.title}>{session.title}</span>
      </div>
      <div style={styles.sessionMeta}>
        <span style={styles.sessionTime}>{session.time}</span>
        <span style={styles.msgCount}>{session.messageCount} 条</span>
      </div>
    </button>
  )
}

// ---- Modals ----

function McpModal({ onClose }: { onClose: () => void }) {
  const servers = [
    { name: 'filesystem', command: 'npx @modelcontextprotocol/server-filesystem', scope: '工作区文件访问', enabled: true },
    { name: 'github', command: 'github-mcp-server', scope: 'Issue / PR / 仓库协作', enabled: true },
    { name: 'browser', command: 'browser-use-mcp', scope: '本地页面调试', enabled: false },
  ]

  return (
    <div style={modalStyles.overlay} onClick={onClose}>
      <div style={modalStyles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={modalStyles.header}>
          <h3 style={modalStyles.title}>MCP 配置</h3>
          <button style={modalStyles.closeBtn} onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        <div style={modalStyles.body}>
          {servers.map((server) => (
            <div key={server.name} style={modalStyles.providerCard}>
              <div style={modalStyles.providerHeader}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                  <Blocks size={16} />
                  <span style={modalStyles.providerName}>{server.name}</span>
                </div>
                <span style={{ ...modalStyles.statusBadge, background: server.enabled ? 'rgba(22,163,74,0.15)' : 'var(--bg-active)', color: server.enabled ? 'var(--success)' : 'var(--text-tertiary)' }}>
                  {server.enabled ? '已启用' : '未启用'}
                </span>
              </div>
              <div style={modalStyles.description}>{server.scope}</div>
              <div style={modalStyles.providerActions}>
                <input type="text" value={server.command} style={modalStyles.input} readOnly />
                <button style={modalStyles.actionBtn}>配置</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function SkillModal({ onClose }: { onClose: () => void }) {
  const skills = [
    { name: 'web-design-engineer', source: 'system', description: '前端界面与交互原型构建', enabled: true },
    { name: 'github-work', source: 'workspace', description: 'Issue、提交、推送与 PR 协作', enabled: true },
    { name: 'kb-retriever', source: 'workspace', description: '本地知识库检索与问答', enabled: false },
  ]

  return (
    <div style={modalStyles.overlay} onClick={onClose}>
      <div style={modalStyles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={modalStyles.header}>
          <h3 style={modalStyles.title}>Skill 配置</h3>
          <button style={modalStyles.closeBtn} onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        <div style={modalStyles.body}>
          {skills.map((skill) => (
            <div key={skill.name} style={modalStyles.providerCard}>
              <div style={modalStyles.providerHeader}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                  <Cpu size={16} />
                  <span style={modalStyles.providerName}>{skill.name}</span>
                </div>
                <span style={{ ...modalStyles.statusBadge, background: skill.enabled ? 'rgba(22,163,74,0.15)' : 'var(--bg-active)', color: skill.enabled ? 'var(--success)' : 'var(--text-tertiary)' }}>
                  {skill.enabled ? '已启用' : '未启用'}
                </span>
              </div>
              <div style={modalStyles.description}>{skill.description}</div>
              <div style={modalStyles.providerActions}>
                <input type="text" value={skill.source} style={modalStyles.input} readOnly />
                <button style={modalStyles.actionBtn}>配置</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function ProviderModal({ onClose }: { onClose: () => void }) {
  const providers = [
    { name: 'Anthropic', models: ['Claude 4 Sonnet', 'Claude 4 Opus'], apiKey: 'sk-ant-***', enabled: true },
    { name: 'OpenAI', models: ['GPT-4o', 'GPT-4o Mini'], apiKey: 'sk-***', enabled: true },
    { name: 'Google', models: ['Gemini 2.5 Pro'], apiKey: '', enabled: false },
    { name: 'DeepSeek', models: ['DeepSeek V3'], apiKey: '', enabled: false },
  ]

  return (
    <div style={modalStyles.overlay} onClick={onClose}>
      <div style={modalStyles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={modalStyles.header}>
          <h3 style={modalStyles.title}>供应商设置</h3>
          <button style={modalStyles.closeBtn} onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        <div style={modalStyles.body}>
          {providers.map((p) => (
            <div key={p.name} style={modalStyles.providerCard}>
              <div style={modalStyles.providerHeader}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Server size={16} />
                  <span style={modalStyles.providerName}>{p.name}</span>
                </div>
                <span style={{ ...modalStyles.statusBadge, background: p.enabled ? 'rgba(22,163,74,0.15)' : 'var(--bg-active)', color: p.enabled ? 'var(--success)' : 'var(--text-tertiary)' }}>
                  {p.enabled ? '已启用' : '未启用'}
                </span>
              </div>
              <div style={modalStyles.providerModels}>
                {p.models.map((m) => (
                  <span key={m} style={modalStyles.modelTag}>{m}</span>
                ))}
              </div>
              <div style={modalStyles.providerActions}>
                <input type="text" placeholder="API Key" defaultValue={p.apiKey} style={modalStyles.input} readOnly />
                <button style={modalStyles.actionBtn}>{p.apiKey ? '更新' : '配置'}</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100%',
    background: 'var(--bg-secondary)',
    position: 'relative',
  },
  collapseBtn: {
    position: 'absolute',
    top: 8,
    right: 8,
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
    zIndex: 2,
    opacity: 0.6,
    transition: 'opacity 0.15s',
  },
  topActions: {
    padding: '12px 32px 12px 12px',
    borderBottom: '1px solid var(--border-primary)',
    flexShrink: 0,
  },
  newChatBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    width: '100%',
    padding: '8px 12px',
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-primary)',
    fontSize: 13,
    fontWeight: 500,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    transition: 'all 0.15s',
    marginBottom: 8,
  },
  shortcut: {
    marginLeft: 'auto',
    fontSize: 10,
    padding: '2px 6px',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-active)',
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
  },
  actionGrid: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr 1fr',
    gap: 4,
  },
  actionBtn: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 4,
    padding: '8px 4px',
    borderRadius: 'var(--radius-md)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-secondary)',
    fontSize: 11,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    transition: 'all 0.15s',
  },
  actionLabel: {
    fontSize: 11,
  },
  listHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 6,
    padding: '10px 8px 6px 12px',
    flexShrink: 0,
  },
  listTitle: {
    flexShrink: 0,
    fontSize: 11,
    fontWeight: 600,
    color: 'var(--text-tertiary)',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  },
  searchBox: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    alignItems: 'center',
    gap: 5,
    height: 26,
    padding: '0 8px',
    borderRadius: 'var(--radius-sm)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-tertiary)',
  },
  searchInput: {
    flex: 1,
    minWidth: 0,
    border: 'none',
    outline: 'none',
    background: 'transparent',
    color: 'var(--text-primary)',
    fontSize: 11,
    fontFamily: 'var(--font-ui)',
  },
  listActions: {
    display: 'flex',
    gap: 2,
    flexShrink: 0,
  },
  iconBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 24,
    height: 24,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  scrollArea: {
    flex: 1,
    overflowY: 'auto',
    padding: '0 6px',
  },
  projectGroup: {
    marginBottom: 2,
  },
  projectHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    width: '100%',
    padding: '6px 8px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-secondary)',
    fontSize: 12,
    fontWeight: 600,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    textAlign: 'left',
    transition: 'all 0.15s',
  },
  chevron: {
    display: 'flex',
    transition: 'transform 0.2s',
    color: 'var(--text-tertiary)',
  },
  projectName: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  sessionsList: {
    paddingLeft: 8,
  },
  sessionItem: {
    display: 'flex',
    flexDirection: 'column',
    width: '100%',
    padding: '6px 8px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    color: 'var(--text-secondary)',
    fontSize: 12,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    textAlign: 'left',
    gap: 2,
    transition: 'all 0.15s',
    marginBottom: 1,
  },
  sessionRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  sessionTitle: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    flex: 1,
  },
  sessionMeta: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingLeft: 20,
  },
  sessionTime: {
    fontSize: 11,
    color: 'var(--text-tertiary)',
  },
  msgCount: {
    fontSize: 10,
    color: 'var(--text-tertiary)',
    padding: '1px 5px',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-active)',
  },
  overlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 100,
  },
  contextMenu: {
    position: 'fixed',
    zIndex: 101,
    background: 'var(--bg-tertiary)',
    border: '1px solid var(--border-primary)',
    borderRadius: 'var(--radius-md)',
    padding: '4px',
    minWidth: 140,
    boxShadow: 'var(--shadow-3)',
  },
  contextItem: {
    display: 'block',
    width: '100%',
    padding: '6px 10px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-primary)',
    fontSize: 12,
    textAlign: 'left',
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    transition: 'all 0.15s',
  },
  contextDivider: {
    height: 1,
    background: 'var(--border-primary)',
    margin: '4px 0',
  },
}

const modalStyles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  modal: {
    width: 480,
    maxHeight: '80vh',
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border-primary)',
    borderRadius: 'var(--radius-lg)',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    boxShadow: 'var(--shadow-3)',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 16px',
    borderBottom: '1px solid var(--border-primary)',
  },
  title: {
    fontSize: 15,
    fontWeight: 600,
    color: 'var(--text-primary)',
    margin: 0,
  },
  closeBtn: {
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
  },
  body: {
    padding: '12px',
    overflowY: 'auto',
  },
  providerCard: {
    padding: '12px',
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-tertiary)',
    marginBottom: 8,
  },
  providerHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 8,
  },
  providerName: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--text-primary)',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  statusBadge: {
    fontSize: 10,
    padding: '2px 8px',
    borderRadius: 'var(--radius-sm)',
    fontWeight: 500,
  },
  providerModels: {
    display: 'flex',
    gap: 6,
    marginBottom: 10,
    flexWrap: 'wrap',
  },
  description: {
    color: 'var(--text-secondary)',
    fontSize: 12,
    marginBottom: 10,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  modelTag: {
    fontSize: 11,
    padding: '2px 8px',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-active)',
    color: 'var(--text-secondary)',
    fontFamily: 'var(--font-mono)',
  },
  providerActions: {
    display: 'flex',
    gap: 8,
  },
  input: {
    flex: 1,
    padding: '6px 10px',
    borderRadius: 'var(--radius-sm)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
    fontSize: 12,
    fontFamily: 'var(--font-mono)',
    outline: 'none',
  },
  actionBtn: {
    padding: '6px 14px',
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    color: 'var(--text-secondary)',
    fontSize: 12,
    fontWeight: 500,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    transition: 'all 0.15s',
  },
}
