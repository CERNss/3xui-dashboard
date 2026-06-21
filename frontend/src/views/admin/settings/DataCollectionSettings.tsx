import {
  ClockCircleOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  FieldTimeOutlined,
  HistoryOutlined,
  NodeIndexOutlined,
  SyncOutlined,
} from '@ant-design/icons'
import { Button, Empty, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { inputMax, inputMin, itemValue, localizedDescription, localizedLabel } from './settingHelpers'
import { SettingRow } from './SettingRow'
import type { SettingsSectionProps } from './types'

const DATA_KEYS = {
  dashboardRefresh: 'dashboard_auto_refresh_interval_seconds',
  healthEnabled: 'ops_collect_enabled',
  healthInterval: 'ops_collect_interval_seconds',
  healthConcurrency: 'ops_collect_concurrency',
  healthTimeout: 'ops_collect_timeout_seconds',
  healthRetry: 'ops_collect_retry_attempts',
  healthRetention: 'ops_retention_seconds',
  trafficEnabled: 'traffic_collect_enabled',
  trafficInterval: 'traffic_collect_interval_seconds',
  trafficConcurrency: 'traffic_collect_concurrency',
  trafficTimeout: 'traffic_collect_timeout_seconds',
  trafficRetry: 'traffic_collect_retry_attempts',
  trafficRetention: 'traffic_retention_seconds',
} as const

const KNOWN_DATA_KEYS: ReadonlySet<string> = new Set(Object.values(DATA_KEYS))

interface DataFieldSpec {
  badge: string
  icon: ReactNode
  intentKey: string
  tone?: 'schedule' | 'load' | 'safety' | 'history' | 'toggle'
  unitKey?: string
}

interface DataFieldProps extends SettingsSectionProps {
  item?: SettingItem
  spec: DataFieldSpec
}

export function DataCollectionSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const byKey = new Map(props.items.map((item) => [item.key, item]))
  const healthOn = boolEnabled(byKey.get(DATA_KEYS.healthEnabled), props.drafts)
  const trafficOn = boolEnabled(byKey.get(DATA_KEYS.trafficEnabled), props.drafts)
  const remaining = props.items.filter((item) => !KNOWN_DATA_KEYS.has(item.key))

  if (props.items.length === 0) {
    return (
      <div className="settings-section-stack">
        <section className="settings-empty-panel">
          <Empty description={t('admin.settings.emptySection')} />
        </section>
      </div>
    )
  }

  return (
    <div className="settings-section-stack settings-data-shell">
      <section className="settings-data-overview" aria-labelledby="settings-data-title">
        <div className="settings-data-overview-copy">
          <Typography.Title id="settings-data-title" level={2}>
            {t('admin.settings.dataCollectionTitle')}
          </Typography.Title>
          <Typography.Text>{t('admin.settings.dataCollectionDesc')}</Typography.Text>
        </div>
        <div className="settings-data-summary-grid">
          <SummaryCard
            icon={<DashboardOutlined />}
            title={t('admin.settings.dataCollection.dashboardSummaryTitle')}
            detail={t('admin.settings.dataCollection.dashboardSummaryDesc')}
            value={formatSecondsValue(byKey.get(DATA_KEYS.dashboardRefresh), props.drafts, t)}
          />
          <SummaryCard
            icon={<NodeIndexOutlined />}
            title={t('admin.settings.dataCollection.healthTitle')}
            detail={t('admin.settings.dataCollection.healthSummaryDesc')}
            value={healthOn ? t('admin.settings.dataCollection.running') : t('admin.settings.dataCollection.paused')}
            active={healthOn}
          />
          <SummaryCard
            icon={<DatabaseOutlined />}
            title={t('admin.settings.dataCollection.trafficTitle')}
            detail={t('admin.settings.dataCollection.trafficSummaryDesc')}
            value={trafficOn ? t('admin.settings.dataCollection.running') : t('admin.settings.dataCollection.paused')}
            active={trafficOn}
          />
        </div>
      </section>

      <DataPanel
        id="dashboard-refresh"
        icon={<DashboardOutlined />}
        title={t('admin.settings.dataCollection.dashboardPanelTitle')}
        description={t('admin.settings.dataCollection.dashboardPanelDesc')}
      >
        <div className="settings-data-grid settings-data-grid--single">
          <DataField {...props} item={byKey.get(DATA_KEYS.dashboardRefresh)} spec={fieldSpecs.dashboardRefresh} />
        </div>
      </DataPanel>

      <DataPanel
        id="health-collector"
        icon={<NodeIndexOutlined />}
        title={t('admin.settings.dataCollection.healthPanelTitle')}
        description={t('admin.settings.dataCollection.healthPanelDesc')}
        status={healthOn ? t('admin.settings.dataCollection.running') : t('admin.settings.dataCollection.paused')}
        active={healthOn}
      >
        <div className="settings-data-grid settings-data-grid--three">
          <DataField {...props} item={byKey.get(DATA_KEYS.healthEnabled)} spec={fieldSpecs.healthEnabled} />
          <DataField {...props} item={byKey.get(DATA_KEYS.healthInterval)} spec={fieldSpecs.healthInterval} />
          <DataField {...props} item={byKey.get(DATA_KEYS.healthConcurrency)} spec={fieldSpecs.healthConcurrency} />
        </div>
        <div className="settings-data-grid settings-data-grid--three">
          <DataField {...props} item={byKey.get(DATA_KEYS.healthTimeout)} spec={fieldSpecs.healthTimeout} />
          <DataField {...props} item={byKey.get(DATA_KEYS.healthRetry)} spec={fieldSpecs.healthRetry} />
          <DataField {...props} item={byKey.get(DATA_KEYS.healthRetention)} spec={fieldSpecs.healthRetention} />
        </div>
      </DataPanel>

      <DataPanel
        id="traffic-collector"
        icon={<DatabaseOutlined />}
        title={t('admin.settings.dataCollection.trafficPanelTitle')}
        description={t('admin.settings.dataCollection.trafficPanelDesc')}
        status={trafficOn ? t('admin.settings.dataCollection.running') : t('admin.settings.dataCollection.paused')}
        active={trafficOn}
      >
        <div className="settings-data-grid settings-data-grid--three">
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficEnabled)} spec={fieldSpecs.trafficEnabled} />
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficInterval)} spec={fieldSpecs.trafficInterval} />
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficConcurrency)} spec={fieldSpecs.trafficConcurrency} />
        </div>
        <div className="settings-data-grid settings-data-grid--three">
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficTimeout)} spec={fieldSpecs.trafficTimeout} />
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficRetry)} spec={fieldSpecs.trafficRetry} />
          <DataField {...props} item={byKey.get(DATA_KEYS.trafficRetention)} spec={fieldSpecs.trafficRetention} />
        </div>
      </DataPanel>

      {remaining.length > 0 ? (
        <section className="settings-group-panel" aria-labelledby="settings-data-additional">
          <header className="settings-group-header">
            <Typography.Title id="settings-data-additional" level={3}>
              {t('admin.settings.dataCollection.additionalTitle')}
            </Typography.Title>
            <Typography.Text>{t('admin.settings.dataCollection.additionalDesc')}</Typography.Text>
          </header>
          <div className="settings-group-rows">
            {remaining.map((item) => (
              <SettingRow
                key={item.key}
                item={item}
                drafts={props.drafts}
                saving={props.savingKey === item.key}
                onDraftChange={props.onDraftChange}
                onSave={props.onSave}
                onReset={props.onReset}
              />
            ))}
          </div>
        </section>
      ) : null}
    </div>
  )
}

const fieldSpecs: Record<keyof typeof DATA_KEYS, DataFieldSpec> = {
  dashboardRefresh: {
    badge: 'UI',
    icon: <SyncOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.dashboardRefresh',
    tone: 'schedule',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  healthEnabled: {
    badge: 'RUN',
    icon: <NodeIndexOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthEnabled',
    tone: 'toggle',
  },
  healthInterval: {
    badge: 'SCHEDULE',
    icon: <ClockCircleOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthInterval',
    tone: 'schedule',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  healthConcurrency: {
    badge: 'LOAD',
    icon: <NodeIndexOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthConcurrency',
    tone: 'load',
    unitKey: 'admin.settings.dataCollection.unit.nodes',
  },
  healthTimeout: {
    badge: 'TIMEOUT',
    icon: <FieldTimeOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthTimeout',
    tone: 'safety',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  healthRetry: {
    badge: 'RETRY',
    icon: <SyncOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthRetry',
    tone: 'safety',
    unitKey: 'admin.settings.dataCollection.unit.attempts',
  },
  healthRetention: {
    badge: 'HISTORY',
    icon: <HistoryOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.healthRetention',
    tone: 'history',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  trafficEnabled: {
    badge: 'RUN',
    icon: <DatabaseOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficEnabled',
    tone: 'toggle',
  },
  trafficInterval: {
    badge: 'SCHEDULE',
    icon: <ClockCircleOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficInterval',
    tone: 'schedule',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  trafficConcurrency: {
    badge: 'LOAD',
    icon: <NodeIndexOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficConcurrency',
    tone: 'load',
    unitKey: 'admin.settings.dataCollection.unit.nodes',
  },
  trafficTimeout: {
    badge: 'TIMEOUT',
    icon: <FieldTimeOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficTimeout',
    tone: 'safety',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
  trafficRetry: {
    badge: 'RETRY',
    icon: <SyncOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficRetry',
    tone: 'safety',
    unitKey: 'admin.settings.dataCollection.unit.attempts',
  },
  trafficRetention: {
    badge: 'HISTORY',
    icon: <HistoryOutlined />,
    intentKey: 'admin.settings.dataCollection.intent.trafficRetention',
    tone: 'history',
    unitKey: 'admin.settings.dataCollection.unit.seconds',
  },
}

function SummaryCard({
  active,
  detail,
  icon,
  title,
  value,
}: {
  active?: boolean
  detail: string
  icon: ReactNode
  title: string
  value: string
}) {
  return (
    <div className="settings-data-summary-card" data-active={active}>
      <span className="settings-data-summary-icon">{icon}</span>
      <div className="settings-data-summary-copy">
        <strong>{title}</strong>
        <span>{detail}</span>
      </div>
      <span className="settings-data-summary-value">{value}</span>
    </div>
  )
}

function DataPanel({
  active,
  children,
  description,
  icon,
  id,
  status,
  title,
}: {
  active?: boolean
  children: ReactNode
  description: string
  icon: ReactNode
  id: string
  status?: string
  title: string
}) {
  return (
    <section className="settings-data-panel" aria-labelledby={`settings-data-${id}`}>
      <header className="settings-data-panel-header">
        <span className="settings-data-panel-icon">{icon}</span>
        <div className="settings-data-panel-copy">
          <Typography.Title id={`settings-data-${id}`} level={3}>
            {title}
          </Typography.Title>
          <Typography.Text>{description}</Typography.Text>
        </div>
        {status ? (
          <span className="settings-data-status" data-active={active}>
            {status}
          </span>
        ) : null}
      </header>
      <div className="settings-data-panel-body">{children}</div>
    </section>
  )
}

function DataField({ drafts, item, onDraftChange, onReset, onSave, savingKey, spec }: DataFieldProps) {
  const { i18n, t } = useTranslation()
  if (!item) return null

  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const controlID = `setting-${item.key}`
  const label = localizedLabel(item, i18n.language)
  const description = localizedDescription(item, i18n.language)
  const min = inputMin(item.key)
  const max = inputMax(item.key, drafts)

  return (
    <div className="settings-data-field" data-setting-key={item.key} data-tone={spec.tone}>
      <div className="settings-data-field-header">
        <label htmlFor={controlID}>
          <span aria-hidden="true">{spec.icon}</span>
          {label}
        </label>
        <span className="settings-data-badge">{spec.badge}</span>
      </div>
      {item.type === 'bool' ? (
        <select
          aria-label={label}
          className="settings-data-native-control"
          id={controlID}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        >
          <option value="">{t('admin.settings.useDefault')}</option>
          <option value="true">{t('admin.settings.dataCollection.enabled')}</option>
          <option value="false">{t('admin.settings.dataCollection.disabled')}</option>
        </select>
      ) : (
        <input
          aria-label={label}
          className="settings-data-native-control"
          id={controlID}
          max={max}
          min={min}
          type="number"
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      )}
      <div className="settings-data-field-meta">
        <span>{t(spec.intentKey)}</span>
        {item.type === 'int' && spec.unitKey ? (
          <code>{t('admin.settings.dataCollection.unitLabel', { unit: t(spec.unitKey) })}</code>
        ) : null}
        {item.type === 'int' ? (
          <code>{t('admin.settings.dataCollection.rangeLabel', { range: formatRange(min, max, t) })}</code>
        ) : null}
      </div>
      {description ? <Typography.Text className="settings-data-field-description">{description}</Typography.Text> : null}
      {item.env_fallback ? (
        <Typography.Text className="settings-data-field-description">{t('admin.settings.fallback', { value: item.env_fallback })}</Typography.Text>
      ) : null}
      <Space className="settings-data-field-actions" wrap>
        <Button type="primary" size="small" disabled={!changed} loading={savingKey === item.key} onClick={() => onSave(item)}>
          {t('admin.settings.save')}
        </Button>
        {item.has_override ? (
          <Button size="small" loading={savingKey === item.key} onClick={() => onReset(item)}>
            {t('admin.settings.reset')}
          </Button>
        ) : null}
      </Space>
    </div>
  )
}

function boolEnabled(item: SettingItem | undefined, drafts: Record<string, string>) {
  const value = item ? ((drafts[item.key] ?? itemValue(item)) || item.default) : ''
  return value !== 'false'
}

function formatSecondsValue(item: SettingItem | undefined, drafts: Record<string, string>, t: ReturnType<typeof useTranslation>['t']) {
  const raw = item ? ((drafts[item.key] ?? itemValue(item)) || item.default) : ''
  const seconds = Number(raw)
  if (!Number.isFinite(seconds) || seconds <= 0) return t('admin.settings.dataCollection.off')
  return t('admin.settings.dataCollection.everySeconds', { seconds })
}

function formatRange(min: number | undefined, max: number | undefined, t: ReturnType<typeof useTranslation>['t']) {
  if (typeof min === 'number' && typeof max === 'number') return t('admin.settings.dataCollection.rangeBoth', { min, max })
  if (typeof min === 'number') return t('admin.settings.dataCollection.rangeMin', { min })
  if (typeof max === 'number') return t('admin.settings.dataCollection.rangeMax', { max })
  return t('admin.settings.dataCollection.rangeAny')
}
