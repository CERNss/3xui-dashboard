import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia } from 'pinia'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  set: vi.fn(),
  clear: vi.fn(),
  uploadBrandIcon: vi.fn(),
  smtpTest: vi.fn(),
  listPlans: vi.fn(),
}))
vi.mock('@/api/admin/settings', () => ({
  settingsApi: {
    list: apiStubs.list,
    set: apiStubs.set,
    clear: apiStubs.clear,
    uploadBrandIcon: apiStubs.uploadBrandIcon,
    smtpTest: apiStubs.smtpTest,
  },
}))
vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: {
    list: apiStubs.listPlans,
  },
}))

import Settings from './Settings.vue'

beforeEach(() => {
  apiStubs.list.mockResolvedValue([
    {
      key: 'public_registration_enabled',
      label: 'Public registration enabled',
      label_zh: '公开注册',
      group: 'registration',
      type: 'bool',
      value: '',
      has_override: false,
      env_fallback: 'true',
      default: 'true',
      description: 'Allow signups',
      description_zh: '允许新用户注册',
    },
    {
      key: 'email_domain_allowlist',
      label: 'Email domain allowlist',
      label_zh: '邮箱域名白名单',
      group: 'registration',
      type: 'string',
      value: 'jihuanshe.com,zitadel.z.apps.tongdiaotech.com',
      has_override: false,
      description: 'Allowed domains',
      description_zh: '允许注册/绑定的邮箱域名',
    },
    {
      key: 'email_verification_required',
      label: 'Email verification required',
      label_zh: '邮箱验证',
      group: 'registration',
      type: 'bool',
      value: 'false',
      has_override: true,
      env_fallback: 'false',
      description: 'Require email verification',
      description_zh: '新用户注册时需要验证邮箱',
    },
    { key: 'expiry_warn_days', label: '到期提醒天数', group: 'other', value: '3', has_override: false },
    {
      key: 'new_user_initial_balance_cents',
      label: 'New-user initial balance',
      label_zh: '新用户初始余额',
      group: 'registration',
      type: 'int',
      value: '100',
      has_override: true,
      description: 'Initial credit',
      description_zh: '初始赠送余额',
    },
    {
      key: 'new_user_plan_ids',
      label: 'New-user starter plans',
      label_zh: '新用户可选套餐',
      group: 'registration',
      type: 'string',
      value: '1',
      has_override: true,
      description: 'Starter plans',
      description_zh: '初始可选套餐',
    },
    {
      key: 'oidc_issuer',
      label: 'OIDC issuer',
      group: 'other',
      type: 'string',
      value: 'https://auth.example.com',
      has_override: true,
      env_fallback: '',
      description: 'OIDC issuer',
    },
    {
      key: 'oidc_client_id',
      label: 'OIDC client ID',
      group: 'other',
      type: 'string',
      value: 'client-a',
      has_override: true,
      env_fallback: '',
      description: 'OIDC client ID',
    },
    {
      key: 'oidc_client_secret',
      label: 'OIDC client secret',
      group: 'other',
      type: 'string',
      value: 'secret-a',
      has_override: true,
      env_fallback: '',
      description: 'OIDC client secret',
    },
    {
      key: 'oidc_redirect_url',
      label: 'OIDC redirect URL',
      group: 'other',
      type: 'string',
      value: 'http://localhost:8080/oidc/callback',
      has_override: true,
      env_fallback: '',
      description: 'OIDC redirect URL',
    },
  ])
  apiStubs.listPlans.mockResolvedValue([
    {
      id: 1,
      name: 'Starter 100G',
      duration_days: 30,
      traffic_limit_bytes: 107374182400,
      price_cents: 999,
      enabled: true,
    },
  ])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

async function mountSettings() {
  // RouterLink in the 通知 tab needs a router instance to resolve.
  // A memory-history stub is enough — tests don't exercise navigation.
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/settings', component: { template: '<div/>' } },
      { path: '/admin/webhooks', component: { template: '<div/>' } },
    ],
  })
  await router.push('/admin/settings')
  await router.isReady()
  const w = mount(Settings, {
    global: { plugins: [router, createPinia()] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

describe('admin/Settings.vue smoke', () => {
  it('mounts and fetches settings once', async () => {
    const w = await mountSettings()
    expect(w.exists()).toBe(true)
    expect(apiStubs.list).toHaveBeenCalledTimes(1)
  })

  it('renders each setting key from the API', async () => {
    const w = await mountSettings()
    expect(w.text()).toContain('注册设置')
    expect(w.text()).toContain('到期提醒天数')
  })

  it('keeps new-user policy under the user defaults tab', async () => {
    const w = await mountSettings()

    expect(w.find('#new-user-initial-balance').isVisible()).toBe(false)

    const tabButtons = w.findAll('button[role="tab"]')
    expect(tabButtons).toHaveLength(5)
    await tabButtons[2].trigger('click')

    expect(w.find('#new-user-initial-balance').isVisible()).toBe(true)
    expect(w.text()).toContain('新用户策略')
    expect(w.text()).toContain('Starter 100G')
  })

  it('renders OIDC settings as a dedicated admin settings card', async () => {
    const w = await mountSettings()

    const tabButtons = w.findAll('button[role="tab"]')
    expect(tabButtons[1].text()).toContain('安全与认证')
    expect(w.get('section[style*="display: none"] input[placeholder="https://auth.example.com"]').isVisible()).toBe(false)
    await tabButtons[1].trigger('click')

    expect(w.text()).toContain('OIDC 登录')
    expect(w.text()).toContain('注册设置')
    expect(w.text()).toContain('开放注册')
    expect(w.text()).toContain('邮箱验证')
    expect(w.text()).toContain('邮箱域名白名单')
    expect(w.text()).toContain('@jihuanshe.com')
    expect(w.find('#setting-public_registration_enabled').exists()).toBe(false)
    expect(w.findAll('[role="switch"]')).toHaveLength(2)
    expect(w.text()).toContain('已配置')
    expect(w.text()).not.toContain('key=oidc_issuer')

    const issuer = w.get('input[placeholder="https://auth.example.com"]')
    expect((issuer.element as HTMLInputElement).value).toBe('https://auth.example.com')

    await issuer.setValue('https://runtime.example.com')
    const saveButton = w.findAll('button').find((button) => button.text().includes('保存 OIDC'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(apiStubs.set).toHaveBeenCalledWith('oidc_issuer', 'https://runtime.example.com')
    expect(apiStubs.set).toHaveBeenCalledWith('oidc_client_id', 'client-a')
  })

  it('does not expose internal setting metadata in the admin UI', async () => {
    const w = await mountSettings()

    const tabButtons = w.findAll('button[role="tab"]')
    for (const button of tabButtons) {
      await button.trigger('click')
    }

    const text = w.text()
    expect(text).not.toContain('运行时可改')
    expect(text).not.toContain('空值 =')
    expect(text).not.toContain('key=')
    expect(text).not.toContain('type=')
    expect(text).not.toContain('env fallback')
    expect(text).not.toContain('未覆盖')
    expect(text).not.toContain('已覆盖')
    expect(text).not.toContain('Allow signups')
    expect(text).not.toContain('OIDC issuer')
    expect(text).not.toContain('Returns 503')
    expect(text).not.toContain('SMTP_*')
    expect(text).not.toContain('NOTIFY_ROUTES')
  })
})
