import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8030',
      '/ws': {
        target: 'ws://localhost:8030',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    // Use relative paths so Electron can load assets via file:// protocol
    base: './',
  },
});
