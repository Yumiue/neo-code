/**
 * Gateway JSON-RPC 协议常量，从 Go internal/gateway/protocol/jsonrpc.go 对齐。
 */

// JSON-RPC 版本
export const JSONRPC_VERSION = '2.0'

// RPC 方法名
export const Method = {
  Authenticate: 'gateway.authenticate',
  Ping: 'gateway.ping',
  BindStream: 'gateway.bindStream',
  Run: 'gateway.run',
  Cancel: 'gateway.cancel',
  Compact: 'gateway.compact',
  ListSessions: 'gateway.listSessions',
  LoadSession: 'gateway.loadSession',
  ResolvePermission: 'gateway.resolvePermission',
  ExecuteSystemTool: 'gateway.executeSystemTool',
  ActivateSessionSkill: 'gateway.activateSessionSkill',
  DeactivateSessionSkill: 'gateway.deactivateSessionSkill',
  ListSessionSkills: 'gateway.listSessionSkills',
  ListAvailableSkills: 'gateway.listAvailableSkills',
  Event: 'gateway.event',
} as const

// 帧类型
export const FrameType = {
  Ack: 'ack',
  Error: 'error',
  Event: 'event',
} as const

// 帧动作
export const FrameAction = {
  Run: 'run',
} as const

// 运行时事件类型（从 Go internal/tui/services/runtime_contract.go 对齐）
export const EventType = {
  UserMessage: 'user_message',
  AgentChunk: 'agent_chunk',
  AgentDone: 'agent_done',
  ToolStart: 'tool_start',
  ToolResult: 'tool_result',
  ToolChunk: 'tool_chunk',
  ToolCallThinking: 'tool_call_thinking',
  RunCanceled: 'run_canceled',
  Error: 'error',
  PermissionRequested: 'permission_requested',
  PermissionResolved: 'permission_resolved',
  CompactStart: 'compact_start',
  CompactApplied: 'compact_applied',
  CompactError: 'compact_error',
  TokenUsage: 'token_usage',
  PhaseChanged: 'phase_changed',
  StopReasonDecided: 'stop_reason_decided',
  InputNormalized: 'input_normalized',
  SkillActivated: 'skill_activated',
  SkillDeactivated: 'skill_deactivated',
  SkillMissing: 'skill_missing',
  TodoUpdated: 'todo_updated',
  TodoConflict: 'todo_conflict',
  TodoSummaryInjected: 'todo_summary_injected',
  AssetSaved: 'asset_saved',
  AssetSaveFailed: 'asset_save_failed',
  ProgressEvaluated: 'progress_evaluated',
  VerificationStarted: 'verification_started',
  VerificationStageFinished: 'verification_stage_finished',
  VerificationFinished: 'verification_finished',
  VerificationCompleted: 'verification_completed',
  VerificationFailed: 'verification_failed',
  AcceptanceDecided: 'acceptance_decided',
} as const

// 权限审批决策
export const PermissionDecision = {
  AllowOnce: 'allow_once',
  AllowSession: 'allow_session',
  Reject: 'reject',
} as const

// 停止原因
export const StopReason = {
  UserInterrupt: 'user_interrupt',
  FatalError: 'fatal_error',
  BudgetExceeded: 'budget_exceeded',
  MaxTurnExceeded: 'max_turn_exceeded',
  Accepted: 'accepted',
  RetryExhausted: 'retry_exhausted',
} as const

// --- 类型定义 ---

/** JSON-RPC 请求 */
export interface JSONRPCRequest {
  jsonrpc: typeof JSONRPC_VERSION
  id: string | number
  method: string
  params?: unknown
}

/** JSON-RPC 响应 */
export interface JSONRPCResponse {
  jsonrpc: typeof JSONRPC_VERSION
  id: string | number
  result?: unknown
  error?: JSONRPCError | null
}

/** JSON-RPC 错误 */
export interface JSONRPCError {
  code: number
  message: string
  data?: { gateway_code?: string }
}

/** JSON-RPC 通知（服务端推送） */
export interface JSONRPCNotification {
  jsonrpc: typeof JSONRPC_VERSION
  method: string
  params?: unknown
}

/** 网关消息帧 */
export interface MessageFrame {
  type: string
  action?: string
  session_id?: string
  run_id?: string
  payload?: unknown
}

/** 运行时事件包裹 */
export interface RuntimeEventEnvelope {
  runtime_event_type: string
  turn?: number
  phase?: string
  timestamp?: string
  payload_version?: number
  payload?: unknown
}

/** gateway.authenticate 参数 */
export interface AuthenticateParams {
  token: string
}

/** gateway.bindStream 参数 */
export interface BindStreamParams {
  session_id: string
  run_id?: string
  channel?: string
}

/** gateway.run 参数 */
export interface RunParams {
  session_id?: string
  run_id?: string
  input_text?: string
  input_parts?: RunInputPart[]
  workdir?: string
}

/** gateway.run 输入分片 */
export interface RunInputPart {
  type: string
  text?: string
  media?: { uri: string; mime_type: string; file_name?: string }
}

/** gateway.cancel 参数 */
export interface CancelParams {
  session_id?: string
  run_id: string
}

/** gateway.loadSession 参数 */
export interface LoadSessionParams {
  session_id: string
}

/** gateway.resolvePermission 参数 */
export interface ResolvePermissionParams {
  request_id: string
  decision: string
}

/** 会话摘要 */
export interface SessionSummary {
  id: string
  title: string
  created_at: string
  updated_at: string
}

/** 会话消息 */
export interface SessionMessage {
  role: string
  content: string
  tool_calls?: ToolCall[]
  tool_call_id?: string
  is_error?: boolean
}

/** 工具调用 */
export interface ToolCall {
  id: string
  name: string
  arguments: string
}

/** 会话详情 */
export interface Session {
  id: string
  title: string
  created_at: string
  updated_at: string
  workdir?: string
  messages?: SessionMessage[]
}

/** Token 用量 */
export interface TokenUsage {
  input_tokens: number
  output_tokens: number
  input_source?: string
  output_source?: string
  has_unknown_usage?: boolean
  session_input_tokens: number
  session_output_tokens: number
}

/** gateway.run ack 响应载荷 */
export interface RunAckResult {
  message: string
  session_id?: string
  run_id?: string
}

/** gateway.listSessions 响应载荷 */
export interface ListSessionsResult {
  sessions: SessionSummary[]
}

/** gateway.cancel 响应载荷 */
export interface CancelResult {
  canceled: boolean
  run_id: string
}

/** 权限请求载荷 */
export interface PermissionRequestPayload {
  request_id: string
  tool_call_id: string
  tool_name: string
  tool_category: string
  action_type: string
  operation: string
  target_type: string
  target: string
  decision: string
  reason: string
}
