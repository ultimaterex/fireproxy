/// <reference types="vitest/config" />
import path from 'node:path'
import { randomBytes } from 'node:crypto'
import { writeFileSync } from 'node:fs'
import type { Plugin } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const uiBuild = randomBytes(6).toString('hex')

function uiVersionPlugin(build: string): Plugin {
  return {
    name: 'ui-version',
    writeBundle(options) {
      const outDir = options.dir ?? path.resolve(import.meta.dirname, 'dist')
      writeFileSync(path.join(outDir, 'version.json'), `${JSON.stringify({ build })}\n`)
    },
  }
}

export default defineConfig({
  define: {
    __UI_BUILD__: JSON.stringify(uiBuild),
  },
  plugins: [react(), tailwindcss(), uiVersionPlugin(uiBuild)],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
