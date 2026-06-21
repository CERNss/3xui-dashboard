import {
  CheckCircleOutlined,
  KeyOutlined,
  MailOutlined,
  SendOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { App, Button, Empty, Input, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { useSmtpTest } from '@/hooks/queries/admin/settings'
import { inputMax, inputMin, itemValue, localizedDescription, localizedLabel } from './settingHelpers'
import { SettingRow } from './SettingRow'
import type { SettingsSectionProps } from './types'

const SMTP_KEYS = {
  host: 'smtp_host',
  port: 'smtp_port',
  from: 'smtp_from',
  username: 'smtp_username',
  password: 'smtp_password',
} as const

const KNOWN_SMTP_KEYS: ReadonlySet<string> = new Set(Object.values(SMTP_KEYS))

interface MessageFieldProps extends SettingsSectionProps {
  badge: string
  icon: ReactNode
  item?: SettingItem
}

export function MessagesSettings(props: SettingsSectionProps) {
  const { message } = App.useApp()
  const { t } = useTranslation()
  const [to, setTo] = useState('')
  const smtpTest = useSmtpTest()
  const byKey = new Map(props.items.map((item) => [item.key, item]))
  const remaining = props.items.filter((item) => !KNOWN_SMTP_KEYS.has(item.key))
  const ready = hasValue(byKey.get(SMTP_KEYS.host), props.drafts) && hasValue(byKey.get(SMTP_KEYS.from), props.drafts)

  if (props.items.length === 0) {
    return (
      <div className="settings-section-stack">
        <section className="settings-empty-panel">
          <Empty description={t('admin.settings.emptySection')} />
        </section>
      </div>
    )
  }

  const sendTest = async () => {
    await smtpTest.mutateAsync(to)
    message.success(t('admin.settings.smtpSendOk', { to }))
  }

  return (
    <div className="settings-section-stack settings-message-shell">
      <section className="settings-message-overview" aria-labelledby="settings-message-title">
        <div className="settings-message-overview-copy">
          <Typography.Title id="settings-message-title" level={2}>
            {t('admin.settings.messages.title')}
          </Typography.Title>
          <Typography.Text>{t('admin.settings.messages.desc')}</Typography.Text>
        </div>
        <div className="settings-message-status-card" data-ready={ready}>
          <span className="settings-message-status-icon">{ready ? <CheckCircleOutlined /> : <MailOutlined />}</span>
          <div className="settings-message-status-copy">
            <strong>{ready ? t('admin.settings.messages.ready') : t('admin.settings.messages.needsConfig')}</strong>
            <span>{t('admin.settings.messages.readyDesc')}</span>
          </div>
        </div>
      </section>

      <MessagePanel
        id="smtp-connection"
        icon={<MailOutlined />}
        title={t('admin.settings.messages.connectionTitle')}
        description={t('admin.settings.messages.connectionDesc')}
      >
        <div className="settings-message-grid settings-message-grid--two">
          <MessageField {...props} item={byKey.get(SMTP_KEYS.host)} badge="HOST" icon={<MailOutlined />} />
          <MessageField {...props} item={byKey.get(SMTP_KEYS.port)} badge="PORT" icon={<MailOutlined />} />
        </div>
        <div className="settings-message-grid settings-message-grid--single">
          <MessageField {...props} item={byKey.get(SMTP_KEYS.from)} badge="FROM" icon={<MailOutlined />} />
        </div>
      </MessagePanel>

      <MessagePanel
        id="smtp-auth"
        icon={<KeyOutlined />}
        title={t('admin.settings.messages.authTitle')}
        description={t('admin.settings.messages.authDesc')}
      >
        <div className="settings-message-grid settings-message-grid--two">
          <MessageField {...props} item={byKey.get(SMTP_KEYS.username)} badge="USER" icon={<UserOutlined />} />
          <MessageField {...props} item={byKey.get(SMTP_KEYS.password)} badge="SECRET" icon={<KeyOutlined />} />
        </div>
      </MessagePanel>

      <section className="settings-message-test-panel" aria-labelledby="settings-message-test">
        <div className="settings-message-panel-header">
          <span className="settings-message-panel-icon">
            <SendOutlined />
          </span>
          <div className="settings-message-panel-copy">
            <Typography.Title id="settings-message-test" level={3}>
              {t('admin.settings.smtpTestTitle')}
            </Typography.Title>
            <Typography.Text>{t('admin.settings.smtpHint')}</Typography.Text>
          </div>
        </div>
        <div className="settings-message-test-body">
          <Input
            aria-label={t('admin.settings.smtpRecipient')}
            className="settings-message-test-input"
            type="email"
            placeholder="ops@example.com"
            value={to}
            onChange={(event) => setTo(event.target.value)}
          />
          <Button type="primary" icon={<SendOutlined />} disabled={!to} loading={smtpTest.isPending} onClick={sendTest}>
            {t('admin.settings.smtpSendBtn')}
          </Button>
        </div>
      </section>

      {remaining.length > 0 ? (
        <section className="settings-group-panel" aria-labelledby="settings-message-other">
          <header className="settings-group-header">
            <Typography.Title id="settings-message-other" level={3}>
              {t('admin.settings.messages.otherTitle')}
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

function MessagePanel({
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
    <section className="settings-message-panel" aria-labelledby={`settings-message-${id}`}>
      <header className="settings-message-panel-header">
        <span className="settings-message-panel-icon">{icon}</span>
        <div className="settings-message-panel-copy">
          <Typography.Title id={`settings-message-${id}`} level={3}>
            {title}
          </Typography.Title>
          <Typography.Text>{description}</Typography.Text>
        </div>
      </header>
      <div className="settings-message-panel-body">{children}</div>
    </section>
  )
}

function MessageField({ badge, drafts, icon, item, onDraftChange, onReset, onSave, savingKey }: MessageFieldProps) {
  const { i18n, t } = useTranslation()
  if (!item) return null

  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const label = localizedLabel(item, i18n.language)
  const description = localizedDescription(item, i18n.language)
  const controlID = `message-setting-${item.key}`
  const saving = savingKey === item.key

  return (
    <label className="settings-message-field" data-setting-key={item.key} htmlFor={controlID}>
      <span className="settings-message-label-row">
        <span>
          {icon}
          <span>{label}</span>
        </span>
        <span className="settings-message-badge">{badge}</span>
      </span>
      {item.secret ? (
        <Input.Password
          aria-label={label}
          autoComplete="new-password"
          className="settings-message-input"
          id={controlID}
          placeholder={item.has_override || item.env_fallback ? '********' : undefined}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : item.type === 'int' ? (
        <input
          aria-label={label}
          className="settings-message-native-control"
          id={controlID}
          max={inputMax(item.key, drafts)}
          min={inputMin(item.key)}
          type="number"
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : (
        <Input
          aria-label={label}
          className="settings-message-input"
          id={controlID}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      )}
      <span className="settings-message-field-help">
        {description ? <Typography.Text>{description}</Typography.Text> : null}
        {item.env_fallback ? <Typography.Text>{t('admin.settings.fallback', { value: item.env_fallback })}</Typography.Text> : null}
      </span>
      <Space className="settings-message-field-actions" wrap>
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
  return Boolean((drafts[item.key] ?? itemValue(item) ?? item.env_fallback ?? item.default).trim())
}
