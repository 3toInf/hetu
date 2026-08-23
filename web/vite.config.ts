import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite config for the Hetu web frontend.
// - dev server proxies /api to the hetud HTTP backend (default 127.0.0.1:19191).
// - build outputs into dist/, which is embedded by web/embed.go via go:embed.
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:19191'
    }
  },
  build: {
    outDir: 'dist'
  }
})
