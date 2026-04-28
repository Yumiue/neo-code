import { app, BrowserWindow, ipcMain, shell } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'

let mainWindow: BrowserWindow | null = null

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

	// 开发模式加载 Vite dev server，生产模式加载打包文件
	if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
		mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
	} else {
		mainWindow.loadFile(join(__dirname, '../dist/index.html'))
	}
}

// ---- IPC 处理 ----

/** 获取认证 Token（从环境变量或 Keychain 读取） */
ipcMain.handle('gateway:getToken', () => {
	// 优先从环境变量读取，后续可扩展 Keychain 集成
	return process.env['NEOCODE_TOKEN'] ?? ''
})

/** 获取 Gateway 地址 */
ipcMain.handle('gateway:getAddress', () => {
	return process.env['NEOCODE_GATEWAY'] ?? '127.0.0.1:8080'
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

app.whenReady().then(() => {
	electronApp.setAppUserModelId('com.neocode.app')

	app.on('browser-window-created', (_, window) => {
		optimizer.watchWindowShortcuts(window)
	})

	createWindow()

	app.on('activate', () => {
		if (BrowserWindow.getAllWindows().length === 0) createWindow()
	})
})

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		app.quit()
	}
})
