import { app, BrowserWindow, ipcMain, shell, dialog } from 'electron'
import { spawn, type ChildProcess } from 'child_process'
import { join } from 'path'
import { existsSync, readFileSync } from 'fs'
import { homedir } from 'os'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'

let mainWindow: BrowserWindow | null = null
let gatewayProcess: ChildProcess | null = null
let gatewayReady = false
let gatewayAddress = ''
let gatewayToken = ''
let currentWorkdir = process.env['NEOCODE_WORKDIR'] ?? app.getPath('home')

/** 创建主窗口 */
function createWindow(): void {
	mainWindow = new BrowserWindow({
		width: 1200,
		height: 800,
		minWidth: 800,
		minHeight: 600,
		show: false,
		title: 'NeoCode',
		titleBarStyle: 'hiddenInset',
		webPreferences: {
			preload: join(__dirname, 'preload.js'),
			sandbox: false,
			contextIsolation: true,
			nodeIntegration: false,
		},
	})

	mainWindow.on('ready-to-show', () => {
		mainWindow?.show()
	})

	mainWindow.webContents.setWindowOpenHandler((details) => {
		shell.openExternal(details.url)
		return { action: 'deny' }
	})

	// Block unexpected in-page navigations that could lead to blank pages
	mainWindow.webContents.on('will-navigate', (event, url) => {
		const devUrl = process.env['ELECTRON_RENDERER_URL'] ?? ''
		const isDevServer = is.dev && devUrl !== '' && url.startsWith(devUrl)
		const isFileProtocol = url.startsWith('file://')

		if (isDevServer || isFileProtocol) {
			return // allow
		}

		event.preventDefault()
		console.warn(`[Electron] Blocked navigation to: ${url}`)
	})

	// Intercept browser back/forward to prevent navigating to invalid URLs
	mainWindow.on('app-command', (e, cmd) => {
		if (cmd === 'browser-backward' || cmd === 'browser-forward') {
			e.preventDefault()
		}
	})

	// 开发模式加载 Vite dev server，生产模式加载打包文件
	if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
		mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
	} else {
		mainWindow.loadFile(join(__dirname, '../dist/index.html'))
	}
}

// ---- Gateway 进程管理 ----

/** 查找 Gateway 可执行文件路径 */
function findGatewayBinary(): string | null {
	const explicit = process.env['NEOCODE_GATEWAY_BIN']
	if (explicit) return explicit
	const candidates = [
		// 开发模式：build-gateway.js 的输出目录
		...(is.dev ? [
			join(__dirname, '..', 'build', 'neocode-gateway'),
			join(__dirname, '..', 'build', 'neocode-gateway.exe'),
		] : []),
		// 打包模式：resources 目录
		join(process.resourcesPath, 'neocode-gateway'),
		join(process.resourcesPath, 'neocode-gateway.exe'),
		// 打包模式：可执行文件同目录
		join(app.getPath('exe'), '..', 'neocode-gateway'),
		join(app.getPath('exe'), '..', 'neocode-gateway.exe'),
	]
	for (const p of candidates) {
		if (existsSync(p)) return p
	}
	return null
}

/** 从 ~/.neocode/auth.json 读取认证 token */
function loadGatewayToken(): string {
	try {
		const authPath = join(homedir(), '.neocode', 'auth.json')
		const raw = readFileSync(authPath, 'utf-8')
		const auth = JSON.parse(raw) as { token?: string }
		return auth.token ?? ''
	} catch {
		return ''
	}
}

/** 启动 Gateway 子进程并等待就绪 */
async function startGateway(): Promise<void> {
	const binary = findGatewayBinary()
	if (!binary) {
		console.warn('[Electron] Gateway binary not found, checking for external gateway')
		gatewayAddress = process.env['NEOCODE_GATEWAY'] ?? '127.0.0.1:8080'
		gatewayToken = process.env['NEOCODE_TOKEN'] ?? ''
		try {
			const res = await fetch(`http://${gatewayAddress}/healthz`, { signal: AbortSignal.timeout(3000) })
			gatewayReady = res.ok
		} catch {
			gatewayReady = false
		}
		if (!gatewayReady) {
			mainWindow?.webContents.send('gateway:status', { ready: false, error: 'Gateway binary not found and no external gateway detected' })
		}
		return
	}

	const httpAddress = process.env['NEOCODE_GATEWAY'] ?? '127.0.0.1:8080'

	console.log(`[Electron] Starting Gateway: ${binary}`)
	gatewayProcess = spawn(binary, ['--http-listen', httpAddress, '--workdir', currentWorkdir], {
		detached: false,
		stdio: 'pipe',
	})

	gatewayProcess.stdout?.on('data', (data: Buffer) => {
		console.log(`[Gateway stdout] ${data.toString().trim()}`)
	})
	gatewayProcess.stderr?.on('data', (data: Buffer) => {
		console.error(`[Gateway stderr] ${data.toString().trim()}`)
	})
	gatewayProcess.on('exit', (code) => {
		console.warn(`[Electron] Gateway exited with code ${code}`)
		gatewayProcess = null
		gatewayReady = false
		mainWindow?.webContents.send('gateway:status', {
			ready: false,
			error: code === 0 ? 'Gateway process exited' : `Gateway process crashed (exit code ${code})`,
		})
	})

	// 轮询等待 Gateway HTTP 端口就绪
	const maxRetries = 30
	for (let i = 0; i < maxRetries; i++) {
		await new Promise((r) => setTimeout(r, 1000))
		try {
			const res = await fetch(`http://${httpAddress}/healthz`, { signal: AbortSignal.timeout(2000) })
			if (res.ok) {
				gatewayAddress = httpAddress
				gatewayToken = loadGatewayToken()
				gatewayReady = true
				console.log('[Electron] Gateway is ready')
				return
			}
		} catch {
			// continue polling
		}
	}

	console.error('[Electron] Gateway health check timed out')
	gatewayAddress = httpAddress
	gatewayToken = loadGatewayToken()
	gatewayReady = false
	mainWindow?.webContents.send('gateway:status', { ready: false, error: 'Gateway health check timed out' })
}

/** 停止 Gateway 子进程 */
function stopGateway(): void {
	if (gatewayProcess) {
		console.log('[Electron] Stopping Gateway')
		gatewayProcess.kill()
		gatewayProcess = null
	}
}

// ---- IPC 处理 ----

/** 获取认证 Token */
ipcMain.handle('gateway:getToken', () => {
	return gatewayToken || process.env['NEOCODE_TOKEN'] || ''
})

/** 获取 Gateway 地址 */
ipcMain.handle('gateway:getAddress', () => {
	return gatewayAddress || process.env['NEOCODE_GATEWAY'] || '127.0.0.1:8080'
})

/** 获取当前工作区目录 */
ipcMain.handle('gateway:getWorkdir', () => currentWorkdir)

/** 选择新工作区目录并重启 Gateway */
ipcMain.handle('gateway:selectWorkdir', async () => {
	if (!mainWindow) return { canceled: true, workdir: currentWorkdir }
	const result = await dialog.showOpenDialog(mainWindow, {
		properties: ['openDirectory'],
		defaultPath: currentWorkdir,
	})
	if (result.canceled || result.filePaths.length === 0) {
		return { canceled: true, workdir: currentWorkdir }
	}
	const newWorkdir = result.filePaths[0]
	if (newWorkdir === currentWorkdir) {
		return { canceled: false, workdir: currentWorkdir }
	}
	currentWorkdir = newWorkdir
	console.log(`[Electron] Workdir changed to: ${currentWorkdir}`)
	// 重启 Gateway 以应用新工作区
	stopGateway()
	await startGateway()
	return { canceled: false, workdir: currentWorkdir }
})

/** 窗口控制 */
ipcMain.handle('window:minimize', () => mainWindow?.minimize())
ipcMain.handle('window:maximize', () => {
	if (mainWindow?.isMaximized()) {
		mainWindow.unmaximize()
	} else {
		mainWindow?.maximize()
	}
})
ipcMain.handle('window:close', () => mainWindow?.close())

// ---- App 生命周期 ----

app.whenReady().then(async () => {
	electronApp.setAppUserModelId('com.neocode.app')

	app.on('browser-window-created', (_, window) => {
		optimizer.watchWindowShortcuts(window)
	})

	await startGateway()
	createWindow()

	app.on('activate', () => {
		if (BrowserWindow.getAllWindows().length === 0) createWindow()
	})
})

app.on('before-quit', (event) => {
	if (gatewayProcess) {
		event.preventDefault()
		gatewayProcess.on('exit', () => app.quit())
		gatewayProcess.kill()
	} else {
		stopGateway()
	}
})

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		app.quit()
	}
})
