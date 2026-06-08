import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for OpenForge frontend E2E tests.
 *
 * Path D T4 — covers dashboard / chat / code-review / admin / onboarding flows.
 *
 * The dev server is expected to be started separately (e.g. `npm run dev`) and
 * available on http://localhost:5173. We do NOT start it from the test runner
 * to keep the runner deterministic and avoid flakiness on slow CI.
 */
export default defineConfig({
  // Resolve testDir and tsconfig relative to the config file's own location
  // (frontend/e2e/) so we never pick up the root tsconfig (which `include`s
  // src/ and would pull vitest into Playwright's loader).
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,
  tsconfig: './tsconfig.json',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  expect: {
    timeout: 5_000,
  },
  timeout: 30_000,
});
