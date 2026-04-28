/** Electron preload 暴露的 API 类型 */
export interface ElectronAPI {
  getToken: () => Promise<string>
  getAddress: () => Promise<string>
  minimize: () => Promise<void>
  maximize: () => Promise<void>
  close: () => Promise<void>
  onGatewayEvent: (callback: (data: unknown) => void) => void
}

declare global {
  interface Window {
    electronAPI?: ElectronAPI
  }
}
