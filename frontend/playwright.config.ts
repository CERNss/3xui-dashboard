import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright config for the 3xui-dashboard UI smoke tests.
 *
 * The tests drive the production build served by the Go binary, so
 * before running you need:
 *
 *   1. A running Postgres at $DATABASE_URL (the docker one-liner from
 *      backend/internal/e2e/harness_test.go works).
 *   2. The backend binary running against that DB with admin creds
 *      matching the test fixtures below.
 *
 * One-shot prep:
 *
 *   docker run -d --rm --name pg-pw -e POSTGRES_PASSWORD=test \
 *     -e POSTGRES_DB=dashboard_pw -p 5498:5432 postgres:16-alpine
 *   cat > /tmp/dashboard.pw.env <<EOF
 *   ENV=dev
 *   LISTEN_ADDR=127.0.0.1:18080
 *   DATABASE_URL=postgres://postgres:test@127.0.0.1:5498/dashboard_pw?sslmode=disable
 *   JWT_SECRET=playwright-test-secret-32-bytes-long
 *   ADMIN_USERNAME=admin
 *   ADMIN_PASSWORD=letmein-pw
 *   PUBLIC_REGISTRATION=true
 *   EOF
 *   cd backend && go run ./cmd/dashboard -env /tmp/dashboard.pw.env &
 *
 *   cd frontend && npm install
 *   npm run e2e:install   # downloads Chromium (~120MB), one-time
 *   BASE_URL=http://127.0.0.1:18080 npm run e2e
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: process.env.BASE_URL ?? 'http://127.0.0.1:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
