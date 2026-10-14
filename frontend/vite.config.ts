import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
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
})
