import {
  Method,
  type JSONRPCNotification,
  type MessageFrame,
} from './protocol'

/** SSE 事件回调 */
export type SSEEventHandler = (frame: MessageFrame) => void

/** SSE 连接状态 */
export type SSEConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error'

/** SSE 连接状态变更回调 */
export type SSEStateHandler = (state: SSEConnectionState) => void

/** SSE 客户端配置 */
export interface SSEClientConfig {
  /** SSE 端点路径，默认 /sse */
  endpoint?: string
  /** 重连间隔（毫秒），默认 3000 */
  reconnectInterval?: number
  /** 最大重连次数，默认 10 */
  maxReconnectAttempts?: number
}

/**
 * 管理 Gateway SSE 事件流连接。
 * 负责建立 EventSource、解析 gateway.event 通知、自动重连。
 */
export function createSSEClient(config: SSEClientConfig = {}) {
  const endpoint = config.endpoint ?? '/sse'
  const reconnectInterval = config.reconnectInterval ?? 3000
  const maxReconnectAttempts = config.maxReconnectAttempts ?? 10

  let es: EventSource | null = null
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let state: SSEConnectionState = 'disconnected'
  let token = ''

  const eventHandlers: SSEEventHandler[] = []
  const stateHandlers: SSEStateHandler[] = []

  function setState(s: SSEConnectionState) {
    state = s
    stateHandlers.forEach((h) => h(s))
  }

  function getState(): SSEConnectionState {
    return state
  }

  /** 设置认证 Token */
  function setToken(t: string) {
    token = t
  }

  /** 注册事件处理器 */
  function onEvent(handler: SSEEventHandler) {
    eventHandlers.push(handler)
    return () => {
      const idx = eventHandlers.indexOf(handler)
      if (idx >= 0) eventHandlers.splice(idx, 1)
    }
  }

  /** 注册状态变更处理器 */
  function onStateChange(handler: SSEStateHandler) {
    stateHandlers.push(handler)
    return () => {
      const idx = stateHandlers.indexOf(handler)
      if (idx >= 0) stateHandlers.splice(idx, 1)
    }
  }

  /** 解析 SSE 推送的 gateway.event 通知 */
  function parseEvent(data: string): MessageFrame | null {
    try {
      const notification: JSONRPCNotification = JSON.parse(data)
      if (notification.method === Method.Event && notification.params) {
        return notification.params as MessageFrame
      }
    } catch {
      // 忽略解析失败
    }
    return null
  }

  /** 建立连接 */
  function connect() {
    if (es) {
      es.close()
      es = null
    }

    setState('connecting')

    const url = token ? `${endpoint}?token=${encodeURIComponent(token)}` : endpoint
    es = new EventSource(url)

    es.onopen = () => {
      reconnectAttempts = 0
      setState('connected')
    }

    es.addEventListener('gateway.event', (e: MessageEvent) => {
      const frame = parseEvent(e.data)
      if (frame) {
        eventHandlers.forEach((h) => h(frame))
      }
    })

    es.onerror = () => {
      setState('error')
      scheduleReconnect()
    }
  }

  /** 安排重连 */
  function scheduleReconnect() {
    if (reconnectAttempts >= maxReconnectAttempts) {
      setState('error')
      return
    }
    reconnectAttempts++
    reconnectTimer = setTimeout(() => {
      connect()
    }, reconnectInterval)
  }

  /** 断开连接 */
  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (es) {
      es.close()
      es = null
    }
    reconnectAttempts = 0
    setState('disconnected')
  }

  return {
    connect,
    disconnect,
    setToken,
    onEvent,
    onStateChange,
    getState,
  }
}

export type SSEClient = ReturnType<typeof createSSEClient>
