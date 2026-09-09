import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:45116',
        ws: true,
      },
    },
  },
  build: {
    outDir: '../server/cmd/server/dist',
    emptyOutDir: true,
    assetsDir: 'assets',
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        // naive-ui is deliberately NOT pinned to any chunk: with lazy routes
        // Rollup splits its components across per-view chunks, keeping every
        // output file small instead of one ~500 kB naive-ui bundle.
        manualChunks(id: string) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('/vue/') || id.includes('@vue/') || id.includes('vue-router')) return 'vue-vendor'
          if (id.includes('date-fns')) return 'date-fns'
          if (id.includes('@vicons')) return 'vicons'
          return undefined
        },
      },
    },
  },
})
