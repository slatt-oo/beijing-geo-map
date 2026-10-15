import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'
import { parseTileCrs } from './src/lib/tiles.ts'

export default defineConfig(({ mode }) => {
  // 坐标系写错时让构建直接失败；否则要到浏览器里加载页面时才抛错，只剩白屏。
  parseTileCrs(loadEnv(mode, process.cwd(), 'VITE_').VITE_TILE_CRS)
  return {
    plugins: [react(), tailwindcss()],
    build: {
      // maplibre-gl 打包压缩后本身就有约 1 MB，拆成单独的包也一样会触发默认 500 kB 的警告。
      chunkSizeWarningLimit: 1500,
    },
    server: {
      proxy: {
        '/api': 'http://localhost:8000',
      },
    },
    test: {
      environment: 'node',
      include: ['src/**/*.test.ts'],
    },
  }
})
