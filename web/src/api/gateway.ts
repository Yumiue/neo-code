import { createRPCClient, type RPCClient } from './client'
import {
  Method,
  type AuthenticateParams,
  type BindStreamParams,
  type RunParams,
  type CancelParams,
  type LoadSessionParams,
  type ResolvePermissionParams,
  type Session,
  type RunAckResult,
  type ListSessionsResult,
  type CancelResult,
} from './protocol'

/** Gateway 业务 API 客户端 */
export class GatewayAPI {
  private rpc: RPCClient

  constructor(baseURL = '') {
    this.rpc = createRPCClient(baseURL)
  }

  /** 认证，返回 ack 结果 */
  async authenticate(token: string) {
    return this.rpc.call(Method.Authenticate, { token } satisfies AuthenticateParams)
  }

  /** 绑定事件流到指定会话 */
  async bindStream(params: BindStreamParams) {
    return this.rpc.call(Method.BindStream, params)
  }

  /** 发起一次 run，返回 ack 含 session_id 和 run_id */
  async run(params: RunParams) {
    return this.rpc.call<RunAckResult>(Method.Run, params)
  }

  /** 取消运行，返回取消结果 */
  async cancel(params: CancelParams) {
    return this.rpc.call<CancelResult>(Method.Cancel, params)
  }

  /** 压缩上下文 */
  async compact(sessionId: string, runId: string) {
    return this.rpc.call(Method.Compact, { session_id: sessionId, run_id: runId })
  }

  /** 列出所有会话，返回含 sessions 数组的结构 */
  async listSessions() {
    return this.rpc.call<ListSessionsResult>(Method.ListSessions)
  }

  /** 加载会话详情 */
  async loadSession(sessionId: string) {
    return this.rpc.call<Session>(Method.LoadSession, { session_id: sessionId } satisfies LoadSessionParams)
  }

  /** 解析权限请求 */
  async resolvePermission(params: ResolvePermissionParams) {
    return this.rpc.call(Method.ResolvePermission, params)
  }

  /** 执行系统工具 */
  async executeSystemTool(sessionId: string, runId: string, toolName: string, args: string, workdir?: string) {
    return this.rpc.call(Method.ExecuteSystemTool, {
      session_id: sessionId,
      run_id: runId,
      tool_name: toolName,
      arguments: args,
      workdir,
    })
  }

  /** Ping 网关 */
  async ping() {
    return this.rpc.call(Method.Ping)
  }
}

/** 全局 Gateway API 实例 */
export const gatewayAPI = new GatewayAPI()
