import { create } from 'zustand'
import { gatewayAPI } from '@/api/gateway'
import { type SessionSummary as APISessionSummary } from '@/api/protocol'
import { useChatStore } from '@/store/useChatStore'
import { useGatewayStore } from '@/store/useGatewayStore'

/** 会话摘要（UI 层展示用） */
export interface SessionSummary {
  id: string
  title: string
  time: string
  messageCount: number
}

/** 项目分组 */
export interface Project {
  id: string
  name: string
  sessions: SessionSummary[]
}

/** 会话状态 */
interface SessionState {
  /** 项目列表 */
  projects: Project[]
  /** 当前活跃会话 ID */
  currentSessionId: string
  /** 当前活跃会话所在项目 ID */
  currentProjectId: string
  /** 是否正在加载 */
  loading: boolean

  // Actions
  setProjects: (projects: Project[]) => void
  setCurrentSessionId: (id: string) => void
  setCurrentProjectId: (id: string) => void
  setLoading: (loading: boolean) => void
  /** 从后端拉取会话列表并映射为项目分组 */
  fetchSessions: () => Promise<void>
  /** 切换到指定会话：绑定流 + 加载历史消息 */
  switchSession: (sessionId: string) => Promise<void>
  /** 创建新会话：绑定空 session 并准备接收事件 */
  createSession: () => Promise<string>
}

/** 将后端扁平会话列表映射为项目分组结构 */
function mapSessionsToProjects(apiSessions: APISessionSummary[]): Project[] {
  // 后端暂无 project 字段，按日期分组作为兜底
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today.getTime() - 86400000)
  const weekAgo = new Date(today.getTime() - 7 * 86400000)

  const groups: Record<string, APISessionSummary[]> = {
    '今天': [],
    '昨天': [],
    '最近7天': [],
    '更早': [],
  }

  for (const s of apiSessions) {
    const updated = new Date(s.updated_at)
    if (updated >= today) {
      groups['今天'].push(s)
    } else if (updated >= yesterday) {
      groups['昨天'].push(s)
    } else if (updated >= weekAgo) {
      groups['最近7天'].push(s)
    } else {
      groups['更早'].push(s)
    }
  }

  const projects: Project[] = []
  for (const [name, sessions] of Object.entries(groups)) {
    if (sessions.length === 0) continue
    projects.push({
      id: `group_${name}`,
      name,
      sessions: sessions.map((s) => ({
        id: s.id,
        title: s.title || '未命名会话',
        time: s.updated_at,
        messageCount: 0,
      })),
    })
  }

  return projects
}

export const useSessionStore = create<SessionState>((set, get) => ({
  projects: [],
  currentSessionId: '',
  currentProjectId: '',
  loading: false,

  setProjects: (projects) => set({ projects }),
  setCurrentSessionId: (currentSessionId) => set({ currentSessionId }),
  setCurrentProjectId: (currentProjectId) => set({ currentProjectId }),
  setLoading: (loading) => set({ loading }),

  fetchSessions: async () => {
    set({ loading: true })
    try {
      const result = await gatewayAPI.listSessions()
      const projects = mapSessionsToProjects(result.sessions)
      set({ projects, loading: false })

      // 如果当前没有选中会话且有可用会话，自动选中第一个
      const state = get()
      if (!state.currentSessionId && result.sessions.length > 0) {
        const firstSession = result.sessions[0]
        get().setCurrentSessionId(firstSession.id)
      }
    } catch (err) {
      console.error('fetchSessions failed:', err)
      set({ loading: false })
    }
  },

  switchSession: async (sessionId: string) => {
    set({ currentSessionId: sessionId, loading: true })
    try {
      // 绑定事件流到目标会话
      await gatewayAPI.bindStream({ session_id: sessionId, channel: 'all' })
      useGatewayStore.getState().setBoundSession(sessionId)

      // 加载历史消息
      const session = await gatewayAPI.loadSession(sessionId)
      if (session.messages && session.messages.length > 0) {
        const chatStore = useChatStore.getState()
        chatStore.clearMessages()
        for (const msg of session.messages) {
          chatStore.addMessage({
            id: `hist_${msg.role}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
            role: (msg.role as 'user' | 'assistant' | 'tool') || 'assistant',
            type: 'text',
            content: msg.content,
            timestamp: Date.now(),
            ...(msg.tool_calls && msg.tool_calls.length > 0
              ? { toolName: msg.tool_calls[0].name, toolCallId: msg.tool_calls[0].id, toolArgs: msg.tool_calls[0].arguments, type: 'tool_call' as const }
              : {}),
          })
        }
      } else {
        useChatStore.getState().clearMessages()
      }
    } catch (err) {
      console.error('switchSession failed:', err)
    } finally {
      set({ loading: false })
    }
  },

  createSession: async () => {
    // 新会话：清空当前消息，准备接收后端事件
    useChatStore.getState().clearMessages()
    // 生成临时 session ID，后端会在 run ack 中返回真实 ID
    const tempId = `new_${Date.now()}`
    set({ currentSessionId: tempId })
    return tempId
  },
}))
