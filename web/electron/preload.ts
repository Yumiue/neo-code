import { contextBridge, ipcRenderer } from 'electron'

/** 暴露安全 API 到渲染进程 */
contextBridge.exposeInMainWorld('electronAPI', {
	/** 获取认证 Token */
	getToken: () => ipcRenderer.invoke('gateway:getToken'),

	/** 获取 Gateway 地址 */
	getAddress: () => ipcRenderer.invoke('gateway:getAddress'),

	/** 窗口控制 */
	minimize: () => ipcRenderer.invoke('window:minimize'),
	maximize: () => ipcRenderer.invoke('window:maximize'),
	close: () => ipcRenderer.invoke('window:close'),

	/** 监听主进程事件 */
	onGatewayEvent: (callback: (data: unknown) => void) => {
		ipcRenderer.on('gateway:event', (_event, data) => callback(data))
	},
})
