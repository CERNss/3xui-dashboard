import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import DataCollectionSettings from './DataCollectionSettings.vue'

const messages = {
  admin: {
    settings: {
      dataCollectionTitle: '数据收集',
      dataCollectionDesc: '配置全部节点相关的后台采集任务',
      groupDataCollection: '节点数据收集',
      settingHelp: {
        opsCollectEnabled: '健康开关说明',
        opsCollectInterval: '健康间隔说明',
        opsCollectConcurrency: '健康并发说明',
        opsCollectTimeout: '健康超时说明',
        opsCollectRetry: '健康重试说明',
        opsRetention: '健康保留说明',
        trafficCollectEnabled: '流量开关说明',
        trafficCollectInterval: '流量间隔说明',
        trafficCollectConcurrency: '流量并发说明',
        trafficCollectTimeout: '流量超时说明',
        trafficCollectRetry: '流量重试说明',
        trafficRetention: '流量保留说明',
      },
      useDefaultEmpty: '跟随系统',
      values: { true: 'true', false: 'false' },
      save: '保存',
      saving: '保存中…',
      reset: '清除',
    },
  },
}

function mountComponent(overrides = {}) {
  const drafts = {
    ops_collect_enabled: 'true',
    ops_collect_interval_seconds: '60',
    ops_collect_concurrency: '8',
    ops_collect_timeout_seconds: '12',
    ops_collect_retry_attempts: '0',
    ops_retention_seconds: '21600',
    traffic_collect_enabled: 'true',
    traffic_collect_interval_seconds: '60',
    traffic_collect_concurrency: '8',
    traffic_collect_timeout_seconds: '30',
    traffic_collect_retry_attempts: '0',
    traffic_retention_seconds: '2592000',
  }
  return mount(DataCollectionSettings, {
    props: {
      items: [
        {
          key: 'ops_collect_enabled',
          label: 'Node health collection',
          label_zh: '节点健康采集',
          type: 'bool',
          group: 'data_collection',
          value: 'true',
          has_override: true,
          env_fallback: '',
          default: 'true',
          description: '',
        },
        {
          key: 'ops_collect_timeout_seconds',
          label: 'Health request timeout',
          label_zh: '健康请求超时',
          type: 'int',
          group: 'data_collection',
          value: '12',
          has_override: false,
          env_fallback: '',
          default: '12',
          description: '',
        },
        {
          key: 'traffic_collect_interval_seconds',
          label: 'Traffic collection interval',
          label_zh: '流量采集间隔',
          type: 'int',
          group: 'data_collection',
          value: '60',
          has_override: false,
          env_fallback: '',
          default: '60',
          description: '',
        },
        {
          key: 'traffic_collect_concurrency',
          label: 'Traffic collection concurrency',
          label_zh: '流量采集并发',
          type: 'int',
          group: 'data_collection',
          value: '8',
          has_override: false,
          env_fallback: '',
          default: '8',
          description: '',
        },
        {
          key: 'traffic_collect_retry_attempts',
          label: 'Traffic retry attempts',
          label_zh: '流量重试次数',
          type: 'int',
          group: 'data_collection',
          value: '0',
          has_override: false,
          env_fallback: '',
          default: '0',
          description: '',
        },
      ],
      drafts,
      savingKey: null,
      ...overrides,
    },
    global: {
      mocks: {
        $t: (key: string) => key.split('.').reduce((value: any, part) => value?.[part], messages) ?? key,
      },
      plugins: [
        {
          install(app: any) {
            app.provide(Symbol.for('vue-i18n'), {})
          },
        },
      ],
      stubs: {},
    },
  })
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string) => key.split('.').reduce((value: any, part) => value?.[part], messages) ?? key,
  }),
}))

describe('DataCollectionSettings.vue', () => {
  it('renders node collection settings and emits save/clear actions', async () => {
    const w = mountComponent()

    expect(w.text()).toContain('数据收集')
    expect(w.text()).toContain('节点健康采集')
    expect(w.text()).toContain('流量采集间隔')
    expect(w.text()).toContain('健康开关说明')
    expect(w.text()).toContain('流量间隔说明')
    expect(w.text()).toContain('流量采集并发')
    expect(w.text()).toContain('流量重试说明')

    expect(w.get('#setting-traffic_collect_concurrency').attributes('min')).toBe('1')
    expect(w.get('#setting-traffic_collect_concurrency').attributes('max')).toBe('64')
    expect(w.get('#setting-ops_collect_timeout_seconds').attributes('min')).toBe('1')
    expect(w.get('#setting-ops_collect_timeout_seconds').attributes('max')).toBe('60')
    expect(w.get('#setting-traffic_collect_retry_attempts').attributes('min')).toBe('0')
    expect(w.get('#setting-traffic_collect_retry_attempts').attributes('max')).toBe('5')

    await w.get('[data-setting-key="ops_collect_enabled"] button').trigger('click')
    expect(w.emitted('save')?.[0]?.[0]).toMatchObject({ key: 'ops_collect_enabled' })

    const clearButtons = w.findAll('button').filter((button) => button.text().includes('清除'))
    expect(clearButtons).toHaveLength(1)
    await clearButtons[0].trigger('click')
    expect(w.emitted('clear')?.[0]?.[0]).toMatchObject({ key: 'ops_collect_enabled' })
  })
})
