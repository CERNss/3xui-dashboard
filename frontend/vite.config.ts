import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            if (id.includes('/vue/') || id.includes('/vue-router/') || id.includes('/pinia/') || id.includes('/@vue/')) {
              return 'vendor-vue'
            }
            if (id.includes('/@vueuse/')) {
              return 'vendor-vueuse'
            }
            if (id.includes('/qrcode/')) {
              return 'vendor-qrcode'
            }
            return 'vendor-misc'
          }
        }
      }
    }
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/sub': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
