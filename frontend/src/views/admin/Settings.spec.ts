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
  listWebhooks: vi.fn(),
  createWebhook: vi.fn(),
  updateWebhook: vi.fn(),
  removeWebhook: vi.fn(),
  testWebhook: vi.fn(),
  webhookDeliveries: vi.fn(),
  replayWebhook: vi.fn(),
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
vi.mock('@/api/admin/webhooks', () => ({
  adminWebhooksApi: {
    list: apiStubs.listWebhooks,
    create: apiStubs.createWebhook,
    update: apiStubs.updateWebhook,
    remove: apiStubs.removeWebhook,
    test: apiStubs.testWebhook,
    deliveries: apiStubs.webhookDeliveries,
    replay: apiStubs.replayWebhook,
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
    { key: 'expiry_warn_days', label: '到期提醒天数', group: 'traffic', type: 'int', value: '3', has_override: false },
    { key: 'traffic_warn_pct', label: '流量预警 %', group: 'traffic', type: 'int', value: '80', has_override: false },
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
      key: 'brand_title',
      label: 'Brand title',
      label_zh: '品牌标题',
      group: 'other',
      type: 'string',
      value: 'Acme Panel',
      has_override: true,
      default: '3xui Central',
      description: 'Main display name',
      description_zh: '主名称',
    },
    {
      key: 'brand_subtitle',
      label: 'Brand subtitle',
      label_zh: '品牌副标题',
      group: 'other',
      type: 'string',
      value: 'node service',
      has_override: true,
      default: 'central panel',
      description: 'Short label',
      description_zh: '短说明',
    },
    {
      key: 'brand_description',
      label: 'Brand description',
      label_zh: '品牌描述',
      group: 'other',
      type: 'string',
      value: 'Private network dashboard',
      has_override: true,
      default: 'Multi-node 3x-ui',
      description: 'Login page copy',
      description_zh: '登录页文案',
    },
    {
      key: 'brand_footer',
      label: 'Brand footer',
      label_zh: '品牌页脚',
      group: 'other',
      type: 'string',
      value: '© 2026 Acme',
      has_override: true,
      default: '© 2026 3xui Central',
      description: 'Footer line',
      description_zh: '页脚',
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
    {
      key: 'subscription_remark_model',
      label: 'Subscription remark model',
      label_zh: '订阅链接备注格式',
      group: 'subscription',
      type: 'string',
      value: '-ieo',
      has_override: true,
      description: 'Remark model',
      description_zh: '订阅节点显示名称格式',
    },
    {
      key: 'clash_template_yaml',
      label: 'Clash template (YAML)',
      label_zh: 'Clash 模板（YAML）',
      group: 'subscription',
      type: 'string',
      value: 'proxies:\n  ${proxies}\nrules:\n  - MATCH,节点选择\n',
      has_override: false,
      description: 'Clash template',
      description_zh: 'Clash/Mihomo 模板',
    },
    {
      key: 'singbox_template_json',
      label: 'Sing-box template (JSON)',
      label_zh: 'Sing-box 模板（JSON）',
      group: 'subscription',
      type: 'string',
      value: '{"outbounds":[${proxies},{"type":"direct","tag":"direct"}],"route":{"final":"select"}}',
      has_override: false,
      description: 'Sing-box template',
      description_zh: 'Sing-box 模板',
    },
    {
      key: 'ops_collect_enabled',
      label: 'Node health collection',
      label_zh: '节点健康采集',
      group: 'data_collection',
      type: 'bool',
      value: 'true',
      has_override: true,
      default: 'true',
      description: 'Collect health',
      description_zh: '采集健康数据',
    },
    {
      key: 'ops_collect_interval_seconds',
      label: 'Health collection interval',
      label_zh: '健康采集间隔',
      group: 'data_collection',
      type: 'int',
      value: '60',
      has_override: false,
      default: '60',
      description: 'Interval seconds',
      description_zh: '采集间隔秒数',
    },
    {
      key: 'ops_retention_seconds',
      label: 'Health history retention',
      label_zh: '健康历史保留',
      group: 'data_collection',
      type: 'int',
      value: '21600',
      has_override: false,
      default: '21600',
      description: 'Retention seconds',
      description_zh: '健康历史保留秒数',
    },
    {
      key: 'traffic_collect_enabled',
      label: 'Node traffic collection',
      label_zh: '节点流量采集',
      group: 'data_collection',
      type: 'bool',
      value: 'true',
      has_override: true,
      default: 'true',
      description: 'Collect traffic',
      description_zh: '采集节点流量计数',
    },
    {
      key: 'traffic_collect_interval_seconds',
      label: 'Traffic collection interval',
      label_zh: '流量采集间隔',
      group: 'data_collection',
      type: 'int',
      value: '60',
      has_override: false,
      default: '60',
      description: 'Traffic interval seconds',
      description_zh: '流量采集间隔秒数',
    },
    {
      key: 'traffic_retention_seconds',
      label: 'Traffic sample retention',
      label_zh: '流量样本保留',
      group: 'data_collection',
      type: 'int',
      value: '2592000',
      has_override: false,
      default: '2592000',
      description: 'Traffic retention seconds',
      description_zh: '流量样本保留秒数',
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
  apiStubs.listWebhooks.mockResolvedValue([
    {
      id: 1,
      name: 'ops-slack',
      url: 'https://hooks.slack.com/services/T/B/X',
      events: ['order.*'],
      enabled: true,
      allow_private: false,
      method: 'POST',
      headers: {},
      body_template: '',
      template_format: 'json',
      created_at: '2026-05-21T10:00:00Z',
      updated_at: '2026-05-21T10:00:00Z',
    },
  ])
  apiStubs.webhookDeliveries.mockResolvedValue([])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

async function mountSettings() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/settings', component: { template: '<div/>' } },
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

function tabByText(wrapper: any, text: string) {
  const tab = wrapper.findAll('button[role="tab"]').find((button: any) => button.text().includes(text))
  expect(tab).toBeTruthy()
  return tab!
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
    expect(tabButtons).toHaveLength(8)
    await tabByText(w, '用户默认值').trigger('click')

    expect(w.find('#new-user-initial-balance').isVisible()).toBe(true)
    expect(w.text()).toContain('新用户策略')
    expect(w.text()).toContain('Starter 100G')
  })

  it('edits brand info on the general tab', async () => {
    const w = await mountSettings()

    expect(w.text()).toContain('品牌信息')
    expect(w.text()).toContain('Acme Panel')
    expect(w.find('#setting-brand_title').exists()).toBe(false)

    const titleInput = w.get('input[placeholder="3xui Central"]')
    await titleInput.setValue('Nova Panel')
    const saveButton = w.findAll('button').find((button) => button.text().includes('保存品牌信息'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(apiStubs.set).toHaveBeenCalledWith('brand_title', 'Nova Panel')
    expect(apiStubs.set).toHaveBeenCalledWith('brand_subtitle', 'node service')
    expect(apiStubs.set).toHaveBeenCalledWith('brand_description', 'Private network dashboard')
    expect(apiStubs.set).toHaveBeenCalledWith('brand_footer', '© 2026 Acme')
  })

  it('renders OIDC settings as a dedicated admin settings card', async () => {
    const w = await mountSettings()

    const tabButtons = w.findAll('button[role="tab"]')
    expect(tabButtons.map((button) => button.text())).toContain('安全与认证')
    expect(w.get('section[style*="display: none"] input[placeholder="https://auth.example.com"]').isVisible()).toBe(false)
    await tabByText(w, '安全与认证').trigger('click')

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

  it('keeps subscription generation settings in their own tab', async () => {
    const w = await mountSettings()

    const tabButtons = w.findAll('button[role="tab"]')
    expect(tabButtons[0].text()).toContain('通用')
    expect(tabButtons[1].text()).toContain('订阅配置')
    expect(tabButtons[2].text()).toContain('告警配置')
    expect(tabButtons[3].text()).toContain('数据收集')
    expect(w.text()).toContain('品牌信息')
    expect(w.find('#setting-clash_template_yaml').isVisible()).toBe(false)

    await tabByText(w, '订阅配置').trigger('click')
    expect(w.find('#setting-clash_template_yaml').isVisible()).toBe(true)
    await tabByText(w, '告警配置').trigger('click')
    expect(w.find('#setting-clash_template_yaml').isVisible()).toBe(false)

    await tabByText(w, '订阅配置').trigger('click')

    expect(w.text()).toContain('订阅配置')
    expect(w.text()).toContain('订阅链接备注格式')
    expect(w.text()).toContain('Clash 模板')
    expect(w.text()).toContain('Clash/Mihomo 模板')
    expect(w.find('#setting-clash_template_yaml').isVisible()).toBe(true)
  })

  it('renders template settings as full-width format-aware editors', async () => {
    const w = await mountSettings()
    await tabByText(w, '订阅配置').trigger('click')

    const clashRow = w.get('[data-setting-key="clash_template_yaml"]')
    expect(clashRow.text()).toContain('Clash 模板')
    expect(clashRow.text()).not.toContain('Clash 模板（YAML）')
    expect(clashRow.text()).toContain('YAML')
    expect(clashRow.text()).toContain('格式化')
    expect(clashRow.find('.settings-code-editor').exists()).toBe(true)

    const singboxRow = w.get('[data-setting-key="singbox_template_json"]')
    expect(singboxRow.text()).toContain('Sing-box 模板')
    expect(singboxRow.text()).toContain('JSON')

    await singboxRow.find('button').trigger('click')
    await flushPromises()

    const singboxEditor = w.get('#setting-singbox_template_json').element as HTMLTextAreaElement
    expect(singboxEditor.value).toContain('\n  "outbounds": [\n')
    expect(singboxEditor.value).toContain('${proxies}')
    expect(singboxRow.text()).toContain('JSON 已格式化')
  })

  it('keeps traffic and expiry thresholds in the alert config tab', async () => {
    const w = await mountSettings()

    const tabButtons = w.findAll('button[role="tab"]')
    expect(tabButtons[0].text()).toContain('通用')
    expect(tabButtons[2].text()).toContain('告警配置')
    expect(w.find('#setting-traffic_warn_pct').isVisible()).toBe(false)

    await tabByText(w, '告警配置').trigger('click')

    expect(w.text()).toContain('告警配置')
    expect(w.text()).toContain('流量预警 %')
    expect(w.text()).toContain('到期提醒天数')
    expect(w.find('#setting-traffic_warn_pct').isVisible()).toBe(true)
    expect(w.find('#setting-expiry_warn_days').isVisible()).toBe(true)
  })

  it('keeps node data collection settings in their own settings tab', async () => {
    const w = await mountSettings()

    expect(w.find('#setting-ops_collect_interval_seconds').isVisible()).toBe(false)
    await tabByText(w, '数据收集').trigger('click')

    expect(w.text()).toContain('数据收集')
    expect(w.text()).toContain('节点健康采集')
    expect(w.text()).toContain('健康采集间隔')
    expect(w.text()).toContain('健康历史保留')
    expect(w.text()).toContain('节点流量采集')
    expect(w.text()).toContain('流量采集间隔')
    expect(w.text()).toContain('流量样本保留')
    expect(w.find('#setting-ops_collect_enabled').isVisible()).toBe(true)
    expect(w.find('#setting-ops_collect_interval_seconds').isVisible()).toBe(true)
    expect(w.find('#setting-ops_retention_seconds').isVisible()).toBe(true)
    expect(w.find('#setting-traffic_collect_enabled').isVisible()).toBe(true)
    expect(w.find('#setting-traffic_collect_interval_seconds').isVisible()).toBe(true)
    expect(w.find('#setting-traffic_retention_seconds').isVisible()).toBe(true)
  })

  it('configures webhooks inside the notifications tab without a page jump', async () => {
    const w = await mountSettings()

    await tabByText(w, '通知').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('运营通知')
    expect(w.text()).toContain('Webhook 推送')
    expect(w.text()).toContain('ops-slack')
    expect(w.text()).toContain('新建 webhook')
    expect(apiStubs.listWebhooks).toHaveBeenCalledTimes(1)
    expect(w.find('a[href="/admin/webhooks"]').exists()).toBe(false)
  })
})
