import {
  ApiOutlined,
  BellOutlined,
  BranchesOutlined,
  CheckCircleOutlined,
  MailOutlined,
  MessageOutlined,
  RobotOutlined,
} from '@ant-design/icons'
import { Button, Empty, Input, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import Webhooks from '../Webhooks'
import { itemValue, localizedDescription, localizedLabel } from './settingHelpers'
import { SettingRow } from './SettingRow'
import type { SettingsSectionProps } from './types'

const NOTIFY_KEYS = {
  routes: 'notify_routes',
  opsRecipient: 'notify_ops_recipient',
  telegramBotToken: 'notify_telegram_bot_token',
  telegramChatID: 'notify_telegram_chat_id',
  discordWebhookURL: 'notify_discord_webhook_url',
  feishuWebhookURL: 'notify_feishu_webhook_url',
  feishuCardTemplate: 'notify_feishu_card_template',
} as const

const KNOWN_NOTIFY_KEYS: ReadonlySet<string> = new Set(Object.values(NOTIFY_KEYS))
const CHANNEL_KEYS = {
  email: [NOTIFY_KEYS.opsRecipient],
  telegram: [NOTIFY_KEYS.telegramBotToken, NOTIFY_KEYS.telegramChatID],
  discord: [NOTIFY_KEYS.discordWebhookURL],
  feishu: [NOTIFY_KEYS.feishuWebhookURL],
} as const

interface NotifyFieldProps extends SettingsSectionProps {
  badge: string
  icon: ReactNode
  item?: SettingItem
  multiline?: boolean
}

export function NotificationsSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const byKey = new Map(props.items.map((item) => [item.key, item]))
  const remaining = props.items.filter((item) => !KNOWN_NOTIFY_KEYS.has(item.key))
  const routesOn = hasValue(byKey.get(NOTIFY_KEYS.routes), props.drafts)
  const channelCount = Object.values(CHANNEL_KEYS).filter((keys) => keys.some((key) => hasValue(byKey.get(key), props.drafts))).length

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
    <div className="settings-section-stack settings-notification-shell">
      <section className="settings-notification-overview" aria-labelledby="settings-notification-title">
        <div className="settings-notification-overview-copy">
          <Typography.Title id="settings-notification-title" level={2}>
            {t('admin.settings.notifications.title')}
          </Typography.Title>
          <Typography.Text>{t('admin.settings.notifications.desc')}</Typography.Text>
        </div>
        <div className="settings-notification-summary-grid">
          <SummaryCard
            icon={<BranchesOutlined />}
            title={t('admin.settings.notifications.routesTitle')}
            detail={t('admin.settings.notifications.routesSummary')}
            value={routesOn ? t('admin.settings.notifications.configured') : t('admin.settings.notifications.notConfigured')}
            active={routesOn}
          />
          <SummaryCard
            icon={<BellOutlined />}
            title={t('admin.settings.notifications.channelsTitle')}
            detail={t('admin.settings.notifications.channelsSummary')}
            value={t('admin.settings.notifications.channelCount', { count: channelCount })}
            active={channelCount > 0}
          />
          <SummaryCard
            icon={<ApiOutlined />}
            title={t('admin.settings.notifications.webhooksTitle')}
            detail={t('admin.settings.notifications.webhooksSummary')}
            value={t('admin.settings.notifications.managedBelow')}
            active
          />
        </div>
      </section>

      <NotifyPanel
        id="routes"
        icon={<BranchesOutlined />}
        title={t('admin.settings.notifications.routesTitle')}
        description={t('admin.settings.notifications.routesDesc')}
      >
        <div className="settings-notification-grid settings-notification-grid--single">
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.routes)} badge="ROUTE" icon={<BranchesOutlined />} />
        </div>
      </NotifyPanel>

      <NotifyPanel
        id="channels"
        icon={<BellOutlined />}
        title={t('admin.settings.notifications.channelsTitle')}
        description={t('admin.settings.notifications.channelsDesc')}
      >
        <div className="settings-notification-grid settings-notification-grid--two">
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.opsRecipient)} badge="EMAIL" icon={<MailOutlined />} />
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.telegramChatID)} badge="CHAT" icon={<MessageOutlined />} />
        </div>
        <div className="settings-notification-grid settings-notification-grid--three">
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.telegramBotToken)} badge="TOKEN" icon={<RobotOutlined />} />
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.discordWebhookURL)} badge="URL" icon={<ApiOutlined />} />
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.feishuWebhookURL)} badge="URL" icon={<ApiOutlined />} />
        </div>
        <div className="settings-notification-grid settings-notification-grid--single">
          <NotifyField {...props} item={byKey.get(NOTIFY_KEYS.feishuCardTemplate)} badge="TEMPLATE" icon={<MessageOutlined />} multiline />
        </div>
      </NotifyPanel>

      <section className="settings-notification-webhooks" aria-labelledby="settings-notification-webhooks">
        <header className="settings-notification-panel-header">
          <span className="settings-notification-panel-icon">
            <ApiOutlined />
          </span>
          <div className="settings-notification-panel-copy">
            <Typography.Title id="settings-notification-webhooks" level={3}>
              {t('admin.settings.notifications.webhooksTitle')}
            </Typography.Title>
            <Typography.Text>{t('admin.settings.notifications.webhooksDesc')}</Typography.Text>
          </div>
        </header>
        <div className="settings-notification-webhooks-body">
          <Webhooks embedded />
        </div>
      </section>

      {remaining.length > 0 ? (
        <section className="settings-group-panel" aria-labelledby="settings-notification-other">
          <header className="settings-group-header">
            <Typography.Title id="settings-notification-other" level={3}>
              {t('admin.settings.notifications.otherTitle')}
            </Typography.Title>
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
    <div className="settings-notification-summary-card" data-active={Boolean(active)}>
      <span className="settings-notification-summary-icon">{icon}</span>
      <div className="settings-notification-summary-copy">
        <strong>{title}</strong>
        <span>{detail}</span>
      </div>
      <span className="settings-notification-summary-value">
        {active ? <CheckCircleOutlined /> : null}
        {value}
      </span>
    </div>
  )
}

function NotifyPanel({
  children,
  description,
  icon,
  id,
  title,
}: {
  children: ReactNode
  description: string
  icon: ReactNode
  id: string
  title: string
}) {
  return (
    <section className="settings-notification-panel" aria-labelledby={`settings-notification-${id}`}>
      <header className="settings-notification-panel-header">
        <span className="settings-notification-panel-icon">{icon}</span>
        <div className="settings-notification-panel-copy">
          <Typography.Title id={`settings-notification-${id}`} level={3}>
            {title}
          </Typography.Title>
          <Typography.Text>{description}</Typography.Text>
        </div>
      </header>
      <div className="settings-notification-panel-body">{children}</div>
    </section>
  )
}

function NotifyField({
  badge,
  drafts,
  icon,
  item,
  multiline,
  onDraftChange,
  onReset,
  onSave,
  savingKey,
}: NotifyFieldProps) {
  const { i18n, t } = useTranslation()
  if (!item) return null

  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const label = localizedLabel(item, i18n.language)
  const description = localizedDescription(item, i18n.language)
  const controlID = `notify-setting-${item.key}`
  const saving = savingKey === item.key

  return (
    <label className={`settings-notification-field${multiline ? ' settings-notification-field--wide' : ''}`} data-setting-key={item.key} htmlFor={controlID}>
      <span className="settings-notification-label-row">
        <span>
          {icon}
          <span>{label}</span>
        </span>
        <span className="settings-notification-badge">{badge}</span>
      </span>
      {multiline ? (
        <Input.TextArea
          aria-label={label}
          className="settings-notification-input settings-notification-textarea"
          id={controlID}
          rows={6}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : item.secret ? (
        <Input.Password
          aria-label={label}
          autoComplete="new-password"
          className="settings-notification-input"
          id={controlID}
          placeholder={item.has_override || item.env_fallback ? '********' : undefined}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : (
        <Input
          aria-label={label}
          className="settings-notification-input"
          id={controlID}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      )}
      <span className="settings-notification-field-help">
        {description ? <Typography.Text>{description}</Typography.Text> : null}
        {item.env_fallback ? <Typography.Text>{t('admin.settings.fallback', { value: item.env_fallback })}</Typography.Text> : null}
      </span>
      <Space className="settings-notification-field-actions" wrap>
        <Button type="primary" size="small" disabled={!changed} loading={saving} onClick={() => onSave(item)}>
          {t('admin.settings.save')}
        </Button>
        {item.has_override ? (
          <Button size="small" loading={saving} onClick={() => onReset(item)}>
            {t('admin.settings.reset')}
          </Button>
        ) : null}
      </Space>
    </label>
  )
}

function hasValue(item: SettingItem | undefined, drafts: Record<string, string>) {
  if (!item) return false
  return Boolean((drafts[item.key] ?? itemValue(item) ?? item.env_fallback ?? item.default).trim() || item.has_override)
}
