// Frontend smoke test — drives the real Vue SPA against a running
// dashboard binary. Prep + run commands live in playwright.config.ts.

import { test, expect } from '@playwright/test'

const ADMIN_USER = process.env.E2E_ADMIN_USER ?? 'admin'
const ADMIN_PASS = process.env.E2E_ADMIN_PASS ?? 'letmein-pw'

const USER_EMAIL = `pw-${Date.now()}@example.com`
const USER_PASS = 'playwright-test-password'

test.describe('Admin', () => {
  test('login routes to the dashboard and renders the nodes table', async ({ page }) => {
    await page.goto('/admin/login')
    await expect(page.getByRole('heading', { name: /admin/i })).toBeVisible()

    await page.getByLabel(/username/i).fill(ADMIN_USER)
    await page.getByLabel(/password/i).fill(ADMIN_PASS)
    await page.getByRole('button', { name: /continue|submit|login/i }).click()

    await page.waitForURL(/\/admin\b/, { timeout: 5_000 })
    await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible()

    // Table headers (Name / Host / Status …) should be present once
    // the page renders, even with zero nodes.
    await expect(page.getByText(/^name$/i)).toBeVisible()
    await expect(page.getByText(/^host$/i)).toBeVisible()
  })

  test('bad password surfaces error and stays on /admin/login', async ({ page }) => {
    await page.goto('/admin/login')
    await page.getByLabel(/username/i).fill(ADMIN_USER)
    await page.getByLabel(/password/i).fill('definitely-wrong')
    await page.getByRole('button', { name: /continue|submit|login/i }).click()
    // Stay on the login page; an error message must show up.
    await expect(page).toHaveURL(/\/admin\/login/)
  })
})

test.describe('Portal', () => {
  test('register → auto-login → dashboard', async ({ page }) => {
    await page.goto('/portal/register')
    await page.getByLabel(/email/i).fill(USER_EMAIL)
    await page.getByLabel(/password/i).fill(USER_PASS)
    await page.getByRole('button', { name: /continue|submit|register/i }).click()

    await page.waitForURL(/\/portal\b/, { timeout: 5_000 })
    // Subscription URL card includes /sub/ + the user's sub_id.
    await expect(page.getByText(/\/sub\//i)).toBeVisible()
  })

  test('subscription URL is publicly fetchable (no auth)', async ({ page, request }) => {
    // Re-login the user created above to grab their sub_id.
    await page.goto('/portal/login')
    await page.getByLabel(/email/i).fill(USER_EMAIL)
    await page.getByLabel(/password/i).fill(USER_PASS)
    await page.getByRole('button', { name: /continue|submit|login/i }).click()
    await page.waitForURL(/\/portal\b/)

    const text = await page.locator('p:has-text("/sub/")').first().textContent()
    const match = text?.match(/\/sub\/([a-f0-9]{32})/)
    expect(match, `extract sub_id from "${text}"`).not.toBeNull()

    const subURL = match![0]
    const resp = await request.get(subURL)
    expect(resp.status()).toBe(200)
    expect(resp.headers()['subscription-userinfo']).toMatch(/upload=/)
  })
})
