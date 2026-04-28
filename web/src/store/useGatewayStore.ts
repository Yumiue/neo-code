import { create } from 'zustand'
import { type SSEConnectionState } from '@/api/sseClient'

/** Gateway 连接状态 */
interface GatewayState {
  /** SSE 连接状态 */
  connectionState: SSEConnectionState
  /** 认证 Token */
  token: string
  /** 当前绑定的 session_id */
  boundSessionId: string
  /** 当前 run_id */
  currentRunId: string
  /** 是否已认证 */
  authenticated: boolean

  // Actions
  setConnectionState: (state: SSEConnectionState) => void
  setToken: (token: string) => void
  setBoundSession: (sessionId: string) => void
  setCurrentRunId: (runId: string) => void
  setAuthenticated: (v: boolean) => void
  reset: () => void
}

const initialState = {
  connectionState: 'disconnected' as SSEConnectionState,
  token: '',
  boundSessionId: '',
  currentRunId: '',
  authenticated: false,
}

export const useGatewayStore = create<GatewayState>((set) => ({
  ...initialState,

  setConnectionState: (connectionState) => set({ connectionState }),
  setToken: (token) => set({ token }),
  setBoundSession: (boundSessionId) => set({ boundSessionId }),
  setCurrentRunId: (currentRunId) => set({ currentRunId }),
  setAuthenticated: (authenticated) => set({ authenticated }),
  reset: () => set(initialState),
}))
