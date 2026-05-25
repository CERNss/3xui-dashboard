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
        opsRetention: '健康保留说明',
        trafficCollectEnabled: '流量开关说明',
        trafficCollectInterval: '流量间隔说明',
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
    ops_retention_seconds: '21600',
    traffic_collect_enabled: 'true',
    traffic_collect_interval_seconds: '60',
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

    await w.get('[data-setting-key="ops_collect_enabled"] button').trigger('click')
    expect(w.emitted('save')?.[0]?.[0]).toMatchObject({ key: 'ops_collect_enabled' })

    const clearButtons = w.findAll('button').filter((button) => button.text().includes('清除'))
    expect(clearButtons).toHaveLength(1)
    await clearButtons[0].trigger('click')
    expect(w.emitted('clear')?.[0]?.[0]).toMatchObject({ key: 'ops_collect_enabled' })
  })
})
