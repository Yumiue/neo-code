import {
  JSONRPC_VERSION,
  Method,
  type JSONRPCNotification,
  type JSONRPCResponse,
  type MessageFrame,
} from './protocol'

/** WS 连接状态 */
export type WSConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error' | 'permanent_error'

/** WS 事件回调 */
export type WSEventHandler = (frame: MessageFrame) => void

/** WS 连接状态变更回调 */
export type WSStateHandler = (state: WSConnectionState) => void

/** WS 重连成功回调 */
export type WSReconnectHandler = () => void

/** WS 客户端配置 */
export interface WSClientConfig {
  /** Gateway 基础地址，例如 http://127.0.0.1:8080 */
  baseURL?: string
  /** WS 端点路径，默认 /ws */
  endpoint?: string
  /** 认证 Token */
  token?: string
  /** 重连基础间隔（毫秒），默认 1000 */
  reconnectBaseInterval?: number
  /** 重连最大间隔（毫秒），默认 30000 */
  reconnectMaxInterval?: number
  /** 最大重连次数，默认 30 */
  maxReconnectAttempts?: number
  /** 心跳超时（毫秒），默认 15000（5 个心跳周期） */
  heartbeatTimeout?: number
  /** 心跳检测间隔（毫秒），默认 5000 */
  heartbeatCheckInterval?: number
  /** 单次 RPC 请求超时（毫秒），默认 30000 */
  rpcTimeout?: number
  /** 认证超时（毫秒），默认 5000（需小于服务端 3s+Wiggle room） */
  authTimeout?: number
}

interface PendingRPC {
  resolve: (value: unknown) => void
  reject: (reason: unknown) => void
  timer: ReturnType<typeof setTimeout>
}

/**
 * 管理 Gateway WebSocket 连接。
 * 全双工单通道：RPC 请求/响应 + 事件推送共用同一连接。
 */
export function createWSClient(config: WSClientConfig = {}) {
  const baseURL = normalizeBaseURL(config.baseURL)
  const endpoint = config.endpoint ?? '/ws'
  const reconnectBaseInterval = config.reconnectBaseInterval ?? 1000
  const reconnectMaxInterval = config.reconnectMaxInterval ?? 30000
  const maxReconnectAttempts = config.maxReconnectAttempts ?? 30
  const heartbeatTimeout = config.heartbeatTimeout ?? 15000
  const heartbeatCheckInterval = config.heartbeatCheckInterval ?? 5000
  const rpcTimeout = config.rpcTimeout ?? 30000
  const authTimeout = config.authTimeout ?? 5000

  let ws: WebSocket | null = null
  let nextId = 1
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let state: WSConnectionState = 'disconnected'
  let token = config.token?.trim() ?? ''
  let lastMessageAt = 0
  let heartbeatCheckTimer: ReturnType<typeof setInterval> | null = null

  // Promise that resolves when WS opens, rejects on close/error before open
  let openResolve: (() => void) | null = null
  let openReject: ((reason: unknown) => void) | null = null
  let openPromise: Promise<void> | null = null

  const pendingRPCs = new Map<string | number, PendingRPC>()
  const eventHandlers: WSEventHandler[] = []
  const stateHandlers: WSStateHandler[] = []
  const reconnectHandlers: WSReconnectHandler[] = []

  function setState(s: WSConnectionState) {
    state = s
    stateHandlers.forEach((h) => h(s))
  }

  function getState(): WSConnectionState {
    return state
  }

  function setToken(t: string) {
    token = t
  }

  function onEvent(handler: WSEventHandler) {
    eventHandlers.push(handler)
    return () => {
      const idx = eventHandlers.indexOf(handler)
      if (idx >= 0) eventHandlers.splice(idx, 1)
    }
  }

  function onStateChange(handler: WSStateHandler) {
    stateHandlers.push(handler)
    return () => {
      const idx = stateHandlers.indexOf(handler)
      if (idx >= 0) stateHandlers.splice(idx, 1)
    }
  }

  function onReconnect(handler: WSReconnectHandler) {
    reconnectHandlers.push(handler)
    return () => {
      const idx = reconnectHandlers.indexOf(handler)
      if (idx >= 0) reconnectHandlers.splice(idx, 1)
    }
  }

  /** 构建 WebSocket URL */
  function buildWSURL(): string {
    const wsBase = baseURL
      .replace(/^http:\/\//, 'ws://')
      .replace(/^https:\/\//, 'wss://')
    const normalizedEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`
    let url = `${wsBase}${normalizedEndpoint}`
    if (token) {
      const separator = url.includes('?') ? '&' : '?'
      url = `${url}${separator}token=${encodeURIComponent(token)}`
    }
    return url
  }

  /** 建立连接 */
  function connect() {
    if (ws) {
      ws.close()
      ws = null
    }

    setState('connecting')
    lastMessageAt = Date.now()

    // Create open promise so call() can wait for the socket to open
    openPromise = new Promise<void>((resolve, reject) => {
      openResolve = resolve
      openReject = reject
    })

    const url = buildWSURL()
    ws = new WebSocket(url)

    ws.onopen = () => {
      lastMessageAt = Date.now()
      const wasReconnect = reconnectAttempts > 0
      reconnectAttempts = 0
      setState('connected')
      startHeartbeatCheck()

      // Resolve the open promise so any pending call() can proceed
      openResolve?.()
      openResolve = null
      openReject = null

      if (wasReconnect) {
        reconnectHandlers.forEach((h) => h())
      }
    }

    ws.onmessage = (event: MessageEvent) => {
      lastMessageAt = Date.now()
      handleMessage(event.data)
    }

    ws.onclose = () => {
      stopHeartbeatCheck()

      // Reject all pending RPCs immediately (same as disconnect)
      for (const [, pending] of pendingRPCs) {
        clearTimeout(pending.timer)
        pending.reject(new Error('WebSocket connection closed'))
      }
      pendingRPCs.clear()

      // Reject the open promise if it hasn't resolved yet
      openReject?.(new Error('WebSocket connection closed before opening'))
      openResolve = null
      openReject = null
      openPromise = null

      if (state !== 'disconnected') {
        setState('error')
        scheduleReconnect()
      }
    }

    ws.onerror = () => {
      // onclose will fire after onerror, reconnect logic is in onclose
    }
  }

  /** 处理收到的消息 */
  function handleMessage(data: string) {
    let parsed: unknown
    try {
      parsed = JSON.parse(data)
    } catch {
      console.warn('[WSClient] Failed to parse message:', data)
      return
    }

    // JSON-RPC response (has id)
    if (isJSONRPCResponse(parsed)) {
      const resp = parsed as JSONRPCResponse
      const pending = pendingRPCs.get(resp.id)
      if (pending) {
        clearTimeout(pending.timer)
        pendingRPCs.delete(resp.id)
        if (resp.error) {
          const gatewayCode = resp.error.data?.gateway_code ?? ''
          pending.reject(new Error(`RPC ${resp.error.code}: ${resp.error.message}${gatewayCode ? ` (${gatewayCode})` : ''}`))
        } else {
          pending.resolve(resp.result)
        }
      }
      return
    }

    // JSON-RPC notification (no id, has method)
    if (isJSONRPCNotification(parsed)) {
      const notification = parsed as JSONRPCNotification
      if (notification.method === Method.Event && notification.params) {
        const frame = notification.params as MessageFrame
        eventHandlers.forEach((h) => h(frame))
      }
      // heartbeat notifications are handled implicitly by lastMessageAt update
      return
    }

    // Heartbeat payload (simple object with type: "heartbeat")
    if (isHeartbeatPayload(parsed)) {
      return
    }
  }

  /** RPC 调用，自动等待 WS 打开后发送 */
  async function call<T = unknown>(method: string, params?: unknown): Promise<T> {
    // Wait for the socket to open (with a short timeout)
    if (openPromise && ws?.readyState !== WebSocket.OPEN) {
      const waitTimeout = new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error('WebSocket open timeout')), authTimeout)
      )
      await Promise.race([openPromise, waitTimeout])
    }

    if (!ws || ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected')
    }

    const socket = ws

    return new Promise<T>((resolve, reject) => {
      const id = nextId++
      const request = {
        jsonrpc: JSONRPC_VERSION,
        id,
        method,
        ...(params !== undefined ? { params } : {}),
      }

      const timer = setTimeout(() => {
        pendingRPCs.delete(id)
        reject(new Error(`RPC timeout: ${method}`))
      }, rpcTimeout)

      pendingRPCs.set(id, { resolve: resolve as (v: unknown) => void, reject, timer })

      try {
        socket.send(JSON.stringify(request))
      } catch (err) {
        clearTimeout(timer)
        pendingRPCs.delete(id)
        reject(err)
      }
    })
  }

  /** 安排重连（指数退避 + 抖动） */
  function scheduleReconnect() {
    if (reconnectAttempts >= maxReconnectAttempts) {
      setState('permanent_error')
      return
    }

    const delay = Math.min(reconnectMaxInterval, reconnectBaseInterval * Math.pow(2, reconnectAttempts))
    const jitter = delay * 0.2 * Math.random()
    const finalDelay = delay + jitter

    reconnectAttempts++
    reconnectTimer = setTimeout(() => {
      connect()
    }, finalDelay)
  }

  /** 主动重连（用于 visibilitychange/online 事件） */
  function reconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.close()
      ws = null
    }
    reconnectAttempts = 0
    connect()
  }

  /** 断开连接 */
  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    stopHeartbeatCheck()

    // Reject all pending RPCs
    for (const [, pending] of pendingRPCs) {
      clearTimeout(pending.timer)
      pending.reject(new Error('WebSocket disconnected'))
    }
    pendingRPCs.clear()

    if (ws) {
      ws.onclose = null
      ws.onerror = null
      ws.onmessage = null
      ws.onopen = null
      ws.close()
      ws = null
    }

    reconnectAttempts = 0
    setState('disconnected')
  }

  /** 启动心跳检测 */
  function startHeartbeatCheck() {
    stopHeartbeatCheck()
    heartbeatCheckTimer = setInterval(() => {
      if (state === 'connected' && Date.now() - lastMessageAt > heartbeatTimeout) {
        // No message received for too long, connection is likely dead
        if (ws) {
          ws.close()
        }
        // onclose handler will trigger reconnect
      }
    }, heartbeatCheckInterval)
  }

  /** 停止心跳检测 */
  function stopHeartbeatCheck() {
    if (heartbeatCheckTimer) {
      clearInterval(heartbeatCheckTimer)
      heartbeatCheckTimer = null
    }
  }

  /** 注册 visibilitychange / online / offline 事件 */
  function setupVisibilityAndNetworkHandlers() {
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('online', handleOnline)
  }

  function teardownVisibilityAndNetworkHandlers() {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    window.removeEventListener('online', handleOnline)
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible' && state === 'error') {
      reconnect()
    }
  }

  function handleOnline() {
    if (state === 'error') {
      reconnect()
    }
  }

  let visibilityHandlersInstalled = false

  function connectWithHandlers() {
    if (!visibilityHandlersInstalled) {
      setupVisibilityAndNetworkHandlers()
      visibilityHandlersInstalled = true
    }
    connect()
  }

  function disconnectWithHandlers() {
    if (visibilityHandlersInstalled) {
      teardownVisibilityAndNetworkHandlers()
      visibilityHandlersInstalled = false
    }
    disconnect()
  }

  return {
    connect: connectWithHandlers,
    disconnect: disconnectWithHandlers,
    reconnect,
    call,
    onEvent,
    onStateChange,
    onReconnect,
    getState,
    setToken,
  }
}

export type WSClient = ReturnType<typeof createWSClient>

// --- Helpers ---

function normalizeBaseURL(baseURL?: string) {
  return (baseURL ?? '').trim().replace(/\/+$/, '')
}

function isJSONRPCResponse(data: unknown): data is JSONRPCResponse {
  if (typeof data !== 'object' || data === null) return false
  const obj = data as Record<string, unknown>
  return 'id' in obj && ('result' in obj || 'error' in obj)
}

function isJSONRPCNotification(data: unknown): data is JSONRPCNotification {
  if (typeof data !== 'object' || data === null) return false
  const obj = data as Record<string, unknown>
  return 'method' in obj && !('id' in obj)
}

function isHeartbeatPayload(data: unknown): boolean {
  if (typeof data !== 'object' || data === null) return false
  const obj = data as Record<string, unknown>
  return obj.type === 'heartbeat'
}
