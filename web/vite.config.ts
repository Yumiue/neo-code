import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import electron from 'vite-plugin-electron'
import renderer from 'vite-plugin-electron-renderer'
import path from 'path'

const isElectron = process.env.NODE_ENV === 'development' && process.env.ELECTRON === 'true'
	|| process.argv.includes('--mode=electron')

export default defineConfig({
  plugins: [
    react(),
    isElectron && electron([
      {
        // 主进程
        entry: 'electron/main.ts',
      },
      {
        // 预加载脚本
        entry: 'electron/preload.ts',
        onstart(args) {
          args.reload()
        },
      },
    ]),
    isElectron && renderer(),
  ].filter(Boolean),
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    open: !isElectron,
    proxy: {
      '/rpc': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/sse': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
