import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [
    vue(),
  ],
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "element-plus/theme-chalk/src/index" as *;`
      }
    }
  },
  server: {
    port: 3000,
    host: '0.0.0.0',
    hmr: {
        host: 'circle33-nexus.test',
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
})
