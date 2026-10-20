import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Admin Desk dev server on port 5175, served under the /admin base path.
export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  server: { port: 5175 },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@carlitos/types': fileURLToPath(new URL('../../packages/types/src/index.ts', import.meta.url)),
      '@carlitos/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@carlitos/shared': fileURLToPath(new URL('../../packages/shared/src/index.ts', import.meta.url)),
    },
  },
})
