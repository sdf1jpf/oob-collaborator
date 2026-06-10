import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'

export default defineConfig({
  plugins: [
    TanStackRouterVite({ target: 'react', autoCodeGeneration: true }),
    react(),
  ],
  server: {
    proxy: {
      '/api': 'http://localhost:443',
      '/ws': { target: 'ws://localhost:443', ws: true },
    },
  },
})
