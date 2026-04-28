import axios, { type AxiosInstance } from 'axios'
import { JSONRPC_VERSION, type JSONRPCRequest, type JSONRPCResponse } from './protocol'

let nextId = 1

/** 创建 JSON-RPC 2.0 请求对象 */
function createRequest(method: string, params?: unknown): JSONRPCRequest {
  return {
    jsonrpc: JSONRPC_VERSION,
    id: nextId++,
    method,
    params,
  }
}

/** 从 JSON-RPC 响应中提取结果或抛出错误 */
function unwrapResponse<T = unknown>(response: JSONRPCResponse): T {
  if (response.error) {
    const err = response.error
    const gatewayCode = err.data?.gateway_code ?? ''
    throw new Error(`RPC ${err.code}: ${err.message}${gatewayCode ? ` (${gatewayCode})` : ''}`)
  }
  return response.result as T
}

/** 创建 Gateway RPC 客户端 */
export function createRPCClient(baseURL = ''): {
  call: <T = unknown>(method: string, params?: unknown) => Promise<T>
  axios: AxiosInstance
} {
  const instance = axios.create({
    baseURL: baseURL || undefined,
    headers: { 'Content-Type': 'application/json' },
    timeout: 30000,
  })

  async function call<T = unknown>(method: string, params?: unknown): Promise<T> {
    const request = createRequest(method, params)
    const { data } = await instance.post<JSONRPCResponse>('/rpc', request)
    return unwrapResponse<T>(data)
  }

  return { call, axios: instance }
}

export type RPCClient = ReturnType<typeof createRPCClient>
