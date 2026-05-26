import { expect, test } from '@playwright/test'

const ADMIN_USER = process.env.E2E_ADMIN_USER ?? 'admin'
const ADMIN_PASS = process.env.E2E_ADMIN_PASS ?? 'letmein-pw'

test.describe('React admin smoke', () => {
  test('login routes to Overview status', async ({ page }) => {
    await page.goto('/login?next=/admin/status')

    await expect(page.getByRole('heading', { name: /welcome back|欢迎回来/i })).toBeVisible()
    await page.getByLabel(/username|用户名/i).fill(ADMIN_USER)
    await page.getByLabel(/password|密码/i).fill(ADMIN_PASS)
    await page.getByRole('button', { name: /sign in|登录/i }).click()

    await page.waitForURL(/\/admin\/status\b/, { timeout: 10_000 })
    await expect(page.getByTestId('admin-layout')).toBeVisible()
    await expect(page.getByRole('heading', { name: /^status|状态$/i })).toBeVisible()
    await expect(page.getByRole('tabpanel', { name: /status panel/i })).toBeVisible()
  })

  test('bad password surfaces an error and stays on login', async ({ page }) => {
    await page.goto('/login?next=/admin/status')

    await page.getByLabel(/username|用户名/i).fill(ADMIN_USER)
    await page.getByLabel(/password|密码/i).fill('definitely-wrong')
    await page.getByRole('button', { name: /sign in|登录/i }).click()

    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByTestId('auth-layout').getByRole('alert')).toBeVisible()
  })
})
