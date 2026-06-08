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
  test: {
    // Exclude Playwright e2e specs — they live in e2e/ and are run by
    // the Playwright runner, not vitest. Without this, vitest picks up
    // the e2e/*.spec.ts files and tries to invoke `page`/`expect` from
    // `@playwright/test` against the vitest test() function.
    exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
  },
});
