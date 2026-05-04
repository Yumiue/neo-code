import type { Plugin, ViteDevServer, Connect } from 'vite'
import { spawn, spawnSync, type ChildProcess } from 'child_process'
import { existsSync, readFileSync, mkdirSync } from 'fs'
import { join, resolve, dirname } from 'path'
import { fileURLToPath } from 'url'
import { homedir, platform, arch } from 'os'

const DEFAULT_BASE_PORT = 8080
const MAX_PORT_ATTEMPTS = 10

export function gatewayDevPlugin(): Plugin {
  let gatewayProcess: ChildProcess | null = null
  let devConfig: { gatewayBaseURL: string; token: string; available: boolean } | null = null

  return {
    name: 'neocode-gateway-dev',
    apply: 'serve',

    configureServer(server: ViteDevServer) {
      // GET：返回当前 dev 配置（前端轮询用）
      server.middlewares.use('/__neocode_dev_config', (req: Connect.IncomingMessage, res: Connect.ServerResponse) => {
        if (req.method === 'POST') {
          handlePost(req, res)
          return
        }
        handleGet(res)
      })

      // 自动探测端口并启动 gateway
      startGatewayWithAutoPort()

      server.httpServer?.on('close', () => {
        if (gatewayProcess) {
          console.log('[neocode-dev] Stopping gateway')
          gatewayProcess.kill()
          gatewayProcess = null
        }
      })
    },
  }

  function handleGet(res: Connect.ServerResponse) {
    res.setHeader('Content-Type', 'application/json')
    if (devConfig) {
      res.end(JSON.stringify(devConfig))
    } else {
      res.statusCode = 503
      res.end(JSON.stringify({ available: true, error: 'gateway not ready' }))
    }
  }

  async function handlePost(req: Connect.IncomingMessage, res: Connect.ServerResponse) {
    const body = await readRequestBody(req)
    let port: number
    try {
      const parsed = JSON.parse(body) as { port?: unknown }
      port = typeof parsed.port === 'number' ? parsed.port : DEFAULT_BASE_PORT
    } catch {
      port = DEFAULT_BASE_PORT
    }

    const httpAddress = `127.0.0.1:${port}`

    // 如果该端口已有 gateway 在跑，直接返回
    if (await checkHealthz(httpAddress)) {
      const token = readTokenFromAuthFile()
      setDevConfig(httpAddress, token)
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify(devConfig))
      return
    }

    // 杀掉之前启动的 gateway（如果有）
    if (gatewayProcess) {
      gatewayProcess.kill()
      gatewayProcess = null
    }

    const started = await tryStartGateway(httpAddress)
    res.setHeader('Content-Type', 'application/json')
    if (started) {
      const token = readTokenFromAuthFile()
      setDevConfig(httpAddress, token)
      res.end(JSON.stringify(devConfig))
    } else {
      res.statusCode = 503
      res.end(JSON.stringify({ available: true, error: `端口 ${port} 被占用或启动失败` }))
    }
  }

  async function startGatewayWithAutoPort() {
    const explicitPort = findExplicitPort()
    if (explicitPort !== null) {
      console.log(`[neocode-dev] Using specified port ${explicitPort}`)
      const addr = `127.0.0.1:${explicitPort}`
      if (await checkHealthz(addr)) {
        console.log(`[neocode-dev] Gateway already running at ${addr}`)
      } else {
        const started = await tryStartGateway(addr)
        if (!started) {
          console.warn(`[neocode-dev] Failed to start gateway on port ${explicitPort}`)
          return
        }
      }
      const token = readTokenFromAuthFile()
      setDevConfig(addr, token)
      return
    }

    // 自动探测：从 8080 开始，逐个尝试
    for (let port = DEFAULT_BASE_PORT; port < DEFAULT_BASE_PORT + MAX_PORT_ATTEMPTS; port++) {
      const addr = `127.0.0.1:${port}`

      if (await checkHealthz(addr)) {
        console.log(`[neocode-dev] Gateway already running at ${addr}`)
        const token = readTokenFromAuthFile()
        setDevConfig(addr, token)
        return
      }

      console.log(`[neocode-dev] Trying port ${port}...`)
      const started = await tryStartGateway(addr)
      if (started) {
        const token = readTokenFromAuthFile()
        setDevConfig(addr, token)
        return
      }
      // 端口被占或启动失败，尝试下一个
      if (gatewayProcess) {
        gatewayProcess.kill()
        gatewayProcess = null
      }
    }

    console.warn(`[neocode-dev] All ports ${DEFAULT_BASE_PORT}-${DEFAULT_BASE_PORT + MAX_PORT_ATTEMPTS - 1} are unavailable`)
  }

  async function tryStartGateway(httpAddress: string): Promise<boolean> {
    const binary = findOrBuildBinary()
    if (!binary) return false

    const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')
    gatewayProcess = spawn(binary, ['--http-listen', httpAddress, '--workdir', projectRoot], {
      detached: false,
      stdio: 'pipe',
    })
    gatewayProcess.stdout?.on('data', (d: Buffer) => console.log(`[gateway] ${d.toString().trim()}`))
    gatewayProcess.stderr?.on('data', (d: Buffer) => console.error(`[gateway] ${d.toString().trim()}`))
    gatewayProcess.on('exit', (code) => {
      console.warn(`[neocode-dev] Gateway exited with code ${code}`)
      gatewayProcess = null
    })

    const ready = await waitForHealthz(httpAddress, 15000, 500)
    if (ready) {
      console.log(`[neocode-dev] Gateway is ready at ${httpAddress}`)
    }
    return ready
  }

  function setDevConfig(httpAddress: string, token: string) {
    const baseURL = /^https?:\/\//i.test(httpAddress) ? httpAddress : `http://${httpAddress}`
    devConfig = { gatewayBaseURL: baseURL.replace(/\/+$/, ''), token, available: true }
    console.log(`[neocode-dev] Dev config available at /__neocode_dev_config (gatewayBaseURL=${devConfig.gatewayBaseURL})`)
  }
}

function findExplicitPort(): number | null {
  // 环境变量优先
  const envPort = process.env['NEOCODE_GATEWAY_PORT']
  if (envPort) {
    const p = parseInt(envPort, 10)
    if (p > 0 && p < 65536) return p
  }
  // 旧环境变量兼容（完整地址格式 127.0.0.1:8080）
  const envAddr = process.env['NEOCODE_GATEWAY']
  if (envAddr) {
    const m = envAddr.match(/:(\d+)$/)
    if (m) {
      const p = parseInt(m[1], 10)
      if (p > 0 && p < 65536) return p
    }
  }
  // CLI 参数 --gateway-port
  const argIdx = process.argv.indexOf('--gateway-port')
  if (argIdx !== -1 && argIdx + 1 < process.argv.length) {
    const p = parseInt(process.argv[argIdx + 1], 10)
    if (p > 0 && p < 65536) return p
  }
  return null
}

function readRequestBody(req: Connect.IncomingMessage): Promise<string> {
  return new Promise((resolve) => {
    const chunks: Buffer[] = []
    req.on('data', (chunk: Buffer) => chunks.push(chunk))
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf-8')))
  })
}

function findOrBuildBinary(): string | null {
  const __dirname = dirname(fileURLToPath(import.meta.url))
  const projectRoot = resolve(__dirname, '..', '..')
  const outputDir = resolve(__dirname, '..', 'build')

  const binaryName = platform() === 'windows' ? 'neocode-gateway.exe' : 'neocode-gateway'
  const existingPath = join(outputDir, binaryName)
  if (existsSync(existingPath)) return existingPath

  const targetMap: Record<string, Record<string, string>> = {
    win32: { x64: 'windows/amd64' },
    darwin: { x64: 'darwin/amd64', arm64: 'darwin/arm64' },
    linux: { x64: 'linux/amd64', arm64: 'linux/arm64' },
  }
  const goosGoarch = targetMap[platform()]?.[arch()]
  if (!goosGoarch) return null

  const [goos, goarch] = goosGoarch.split('/')
  console.log(`[neocode-dev] Building neocode-gateway for ${goos}/${goarch}...`)

  if (!existsSync(outputDir)) mkdirSync(outputDir, { recursive: true })

  const result = spawnSync('go', ['build', '-o', existingPath, './cmd/neocode-gateway'], {
    cwd: projectRoot,
    env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' },
    stdio: 'inherit',
  })

  if (result.status !== 0) {
    console.error('[neocode-dev] Go build failed')
    return null
  }
  console.log(`[neocode-dev] Built: ${existingPath}`)
  return existingPath
}

function readTokenFromAuthFile(): string {
  try {
    const authPath = join(homedir(), '.neocode', 'auth.json')
    const raw = readFileSync(authPath, 'utf-8')
    const auth = JSON.parse(raw) as { token?: string }
    return auth.token ?? ''
  } catch {
    return ''
  }
}

async function checkHealthz(address: string): Promise<boolean> {
  const url = /^https?:\/\//i.test(address) ? address : `http://${address}`
  try {
    const res = await fetch(`${url}/healthz`, { signal: AbortSignal.timeout(2000) })
    return res.ok
  } catch {
    return false
  }
}

async function waitForHealthz(address: string, timeoutMs: number, intervalMs: number): Promise<boolean> {
  const url = /^https?:\/\//i.test(address) ? address : `http://${address}`
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${url}/healthz`, { signal: AbortSignal.timeout(2000) })
      if (res.ok) return true
    } catch {
      // continue polling
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return false
}
