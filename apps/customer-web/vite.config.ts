import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Customer Store dev server on port 5173. Aliases resolve the shared workspace
// packages directly to their TS sources so no build step is needed for them.
export default defineConfig({
  plugins: [vue()],
  server: { port: 5173 },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@carlitos/types': fileURLToPath(new URL('../../packages/types/src/index.ts', import.meta.url)),
      '@carlitos/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@carlitos/shared': fileURLToPath(new URL('../../packages/shared/src/index.ts', import.meta.url)),
    },
  },
})
