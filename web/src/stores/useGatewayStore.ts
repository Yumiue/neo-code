import { create } from 'zustand'
import { type WSConnectionState } from '@/api/wsClient'

/** Gateway 连接状态 */
interface GatewayState {
  /** WS 连接状态 */
  connectionState: WSConnectionState
  /** 认证 Token */
  token: string
  /** 当前 run_id */
  currentRunId: string
  /** 是否已认证 */
  authenticated: boolean

  // Actions
  setConnectionState: (state: WSConnectionState) => void
  setToken: (token: string) => void
  setCurrentRunId: (runId: string) => void
  setAuthenticated: (v: boolean) => void
  reset: () => void
}

const initialState = {
  connectionState: 'disconnected' as WSConnectionState,
  token: '',
  currentRunId: '',
  authenticated: false,
}

export const useGatewayStore = create<GatewayState>((set) => ({
  ...initialState,

  setConnectionState: (connectionState) => set({ connectionState }),
  setToken: (token) => set({ token }),
  setCurrentRunId: (currentRunId) => set({ currentRunId }),
  setAuthenticated: (authenticated) => set({ authenticated }),
  reset: () => set(initialState),
}))
