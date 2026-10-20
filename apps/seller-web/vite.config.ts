import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Seller Desk dev server on port 5174, served under the /seller base path.
export default defineConfig({
  base: '/seller/',
  plugins: [vue()],
  server: { port: 5174 },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@carlitos/types': fileURLToPath(new URL('../../packages/types/src/index.ts', import.meta.url)),
      '@carlitos/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@carlitos/shared': fileURLToPath(new URL('../../packages/shared/src/index.ts', import.meta.url)),
    },
  },
})
