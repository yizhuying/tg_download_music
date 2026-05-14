import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:45116',
        ws: true,
      },
    },
  },
  build: {
    outDir: '../server/cmd/server/dist',
    assetsDir: 'assets',
  },
})
