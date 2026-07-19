import { defineConfig, devices } from '@playwright/test';

// When E2E_EXTERNAL_SERVER is set, the web app is already being served by an
// external origin (e.g. the single-image server at http://localhost:8080), so
// Playwright must NOT spin up its own `npx serve` dev server.
const useExternalServer = !!process.env.E2E_EXTERNAL_SERVER;

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    baseURL: process.env.E2E_WEB_BASE ?? 'http://localhost:8081',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // Uncomment to run cross-browser (install with: npx playwright install)
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },
    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],
  // Serve the exported Expo web bundle. Playwright waits for the URL to be
  // ready before starting tests. reuseExistingServer lets local dev skip the
  // serve step when a dev server is already running. Skipped entirely when
  // E2E_EXTERNAL_SERVER is set (single-image server already serves the web).
  webServer: useExternalServer
    ? undefined
    : {
        command: 'npx serve -s ../app/dist -l 8081',
        url: 'http://localhost:8081',
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
});
