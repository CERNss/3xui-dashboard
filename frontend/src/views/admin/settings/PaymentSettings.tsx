import {
  BankOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CreditCardOutlined,
  DollarCircleOutlined,
  KeyOutlined,
  LinkOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons'
import { Button, Empty, Input, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { inputMax, inputMin, itemValue, localizedDescription, localizedLabel } from './settingHelpers'
import { SettingRow } from './SettingRow'
import type { SettingsSectionProps } from './types'

const PAYMENT_KEYS = {
  alipayAppID: 'alipay_app_id',
  alipayPrivateKey: 'alipay_private_key',
  alipayPublicKey: 'alipay_public_key',
  alipayGateway: 'alipay_gateway',
  alipayNotifyURL: 'alipay_notify_url',
  stripeSecretKey: 'stripe_secret_key',
  stripeWebhookSecret: 'stripe_webhook_secret',
  stripeCurrency: 'stripe_currency',
  stripeSuccessURL: 'stripe_success_url',
  stripeCancelURL: 'stripe_cancel_url',
  stripeSessionExpiryMinutes: 'stripe_session_expiry_minutes',
} as const

const ALIPAY_REQUIRED_KEYS: string[] = [PAYMENT_KEYS.alipayAppID, PAYMENT_KEYS.alipayPrivateKey, PAYMENT_KEYS.alipayPublicKey]
const STRIPE_REQUIRED_KEYS: string[] = [PAYMENT_KEYS.stripeSecretKey, PAYMENT_KEYS.stripeWebhookSecret]
const KNOWN_PAYMENT_KEYS: ReadonlySet<string> = new Set(Object.values(PAYMENT_KEYS))
const TEXTAREA_KEYS: ReadonlySet<string> = new Set([PAYMENT_KEYS.alipayPrivateKey, PAYMENT_KEYS.alipayPublicKey])
const URL_KEYS: ReadonlySet<string> = new Set([PAYMENT_KEYS.alipayGateway, PAYMENT_KEYS.alipayNotifyURL, PAYMENT_KEYS.stripeSuccessURL, PAYMENT_KEYS.stripeCancelURL])

interface PaymentFieldProps extends SettingsSectionProps {
  badge?: string
  icon?: ReactNode
  item?: SettingItem
  rows?: number
}

export function PaymentSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const byKey = new Map(props.items.map((item) => [item.key, item]))
  const alipayReady = readiness(ALIPAY_REQUIRED_KEYS, byKey, props.drafts)
  const stripeReady = readiness(STRIPE_REQUIRED_KEYS, byKey, props.drafts)
  const remaining = props.items.filter((item) => !KNOWN_PAYMENT_KEYS.has(item.key))

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
    <div className="settings-section-stack settings-payment-shell">
      <section className="settings-payment-overview" aria-labelledby="settings-payment-title">
        <div className="settings-payment-overview-copy">
          <Typography.Title id="settings-payment-title" level={2}>
            {t('admin.settings.payment.title')}
          </Typography.Title>
          <Typography.Text>{t('admin.settings.payment.desc')}</Typography.Text>
        </div>
        <div className="settings-payment-provider-grid">
          <ProviderCard
            icon={<BankOutlined />}
            title={t('admin.settings.payment.alipayTitle')}
            description={t('admin.settings.payment.alipayDesc')}
            ready={alipayReady.ready}
            configured={alipayReady.configured}
            total={alipayReady.total}
          />
          <ProviderCard
            icon={<CreditCardOutlined />}
            title={t('admin.settings.payment.stripeTitle')}
            description={t('admin.settings.payment.stripeDesc')}
            ready={stripeReady.ready}
            configured={stripeReady.configured}
            total={stripeReady.total}
          />
        </div>
      </section>

      <GatewayPanel
        id="alipay"
        title={t('admin.settings.payment.alipayTitle')}
        description={t('admin.settings.payment.alipayPanelDesc')}
        icon={<BankOutlined />}
        ready={alipayReady.ready}
      >
        <div className="settings-payment-grid settings-payment-grid--three">
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.alipayAppID)} badge="APP" icon={<SafetyCertificateOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.alipayGateway)} badge="URL" icon={<LinkOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.alipayNotifyURL)} badge="CALLBACK" icon={<LinkOutlined />} />
        </div>
        <div className="settings-payment-grid settings-payment-grid--two">
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.alipayPrivateKey)} badge="PEM" icon={<KeyOutlined />} rows={8} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.alipayPublicKey)} badge="PEM" icon={<KeyOutlined />} rows={8} />
        </div>
      </GatewayPanel>

      <GatewayPanel
        id="stripe"
        title={t('admin.settings.payment.stripeTitle')}
        description={t('admin.settings.payment.stripePanelDesc')}
        icon={<CreditCardOutlined />}
        ready={stripeReady.ready}
      >
        <div className="settings-payment-grid settings-payment-grid--three">
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeSecretKey)} badge="SK" icon={<KeyOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeWebhookSecret)} badge="WHSEC" icon={<KeyOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeCurrency)} badge="ISO" icon={<DollarCircleOutlined />} />
        </div>
        <div className="settings-payment-grid settings-payment-grid--three">
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeSuccessURL)} badge="SUCCESS" icon={<LinkOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeCancelURL)} badge="CANCEL" icon={<LinkOutlined />} />
          <PaymentField {...props} item={byKey.get(PAYMENT_KEYS.stripeSessionExpiryMinutes)} badge="MIN" icon={<ClockCircleOutlined />} />
        </div>
      </GatewayPanel>

      {remaining.length > 0 ? (
        <section className="settings-group-panel" aria-labelledby="settings-payment-other">
          <header className="settings-group-header">
            <Typography.Title id="settings-payment-other" level={3}>
              {t('admin.settings.payment.otherTitle')}
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

function ProviderCard({
  description,
  icon,
  ready,
  configured,
  title,
  total,
}: {
  description: string
  icon: ReactNode
  ready: boolean
  configured: number
  title: string
  total: number
}) {
  const { t } = useTranslation()
  return (
    <div className="settings-payment-provider-card" data-ready={ready}>
      <span className="settings-payment-provider-icon">{icon}</span>
      <div className="settings-payment-provider-copy">
        <strong>{title}</strong>
        <span>{description}</span>
      </div>
      <span className="settings-payment-status">
        {ready ? <CheckCircleOutlined /> : <SafetyCertificateOutlined />}
        {ready ? t('admin.settings.payment.ready') : t('admin.settings.payment.needsConfig')}
      </span>
      <span className="settings-payment-provider-count">
        {t('admin.settings.payment.requiredConfigured', { configured, total })}
      </span>
    </div>
  )
}

function GatewayPanel({
  children,
  description,
  id,
  icon,
  ready,
  title,
}: {
  children: ReactNode
  description: string
  id: string
  icon: ReactNode
  ready: boolean
  title: string
}) {
  const { t } = useTranslation()
  return (
    <section className="settings-payment-gateway" aria-labelledby={`settings-payment-${id}`}>
      <header className="settings-payment-gateway-header">
        <span className="settings-payment-gateway-icon">{icon}</span>
        <div className="settings-payment-gateway-copy">
          <Typography.Title id={`settings-payment-${id}`} level={3}>
            {title}
          </Typography.Title>
          <Typography.Text>{description}</Typography.Text>
        </div>
        <span className="settings-payment-status" data-ready={ready}>
          {ready ? <CheckCircleOutlined /> : <SafetyCertificateOutlined />}
          {ready ? t('admin.settings.payment.ready') : t('admin.settings.payment.needsConfig')}
        </span>
      </header>
      <div className="settings-payment-gateway-body">{children}</div>
    </section>
  )
}

function PaymentField({ badge, drafts, icon, item, onDraftChange, onReset, onSave, rows = 5, savingKey }: PaymentFieldProps) {
  const { i18n, t } = useTranslation()
  if (!item) return null

  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const controlID = `setting-${item.key}`
  const label = localizedLabel(item, i18n.language)
  const description = localizedDescription(item, i18n.language)
  const isTextArea = TEXTAREA_KEYS.has(item.key)
  const isSecretTextArea = isTextArea && item.secret
  const isURL = URL_KEYS.has(item.key)
  const placeholder = placeholderForPaymentField(item, t)

  return (
    <div
      className={`settings-payment-field${isTextArea ? ' settings-payment-field--textarea' : ''}`}
      data-setting-key={item.key}
    >
      <div className="settings-payment-label-row">
        <label htmlFor={controlID}>
          {icon ? <span aria-hidden="true">{icon}</span> : null}
          {label}
        </label>
        {badge ? <span className="settings-payment-badge">{badge}</span> : null}
      </div>
      {item.type === 'int' ? (
        <input
          aria-label={label}
          className="settings-payment-native-control"
          id={controlID}
          max={inputMax(item.key, drafts)}
          min={inputMin(item.key)}
          placeholder={placeholder}
          type="number"
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : isTextArea ? (
        <Input.TextArea
          aria-label={label}
          className="settings-payment-input settings-payment-textarea"
          id={controlID}
          autoComplete={item.secret ? 'new-password' : 'off'}
          autoCorrect="off"
          placeholder={placeholder}
          rows={rows}
          spellCheck={false}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : item.secret ? (
        <Input.Password
          aria-label={label}
          className="settings-payment-input"
          id={controlID}
          autoComplete="new-password"
          placeholder={placeholder}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      ) : (
        <Input
          aria-label={label}
          className="settings-payment-input"
          id={controlID}
          autoComplete={isURL ? 'url' : 'off'}
          placeholder={placeholder}
          value={draft}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
      )}
      <div className="settings-payment-field-help">
        {description ? <Typography.Text>{description}</Typography.Text> : null}
        {isSecretTextArea ? <Typography.Text>{t('admin.settings.payment.secretTextareaHint')}</Typography.Text> : null}
        {item.env_fallback ? <Typography.Text>{t('admin.settings.fallback', { value: item.env_fallback })}</Typography.Text> : null}
      </div>
      <Space className="settings-payment-field-actions" wrap>
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

function readiness(keys: string[], items: Map<string, SettingItem>, drafts: Record<string, string>) {
  const requiredItems = keys.map((key) => items.get(key)).filter((item): item is SettingItem => Boolean(item))
  const configured = requiredItems.filter((item) => settingConfigured(item, drafts)).length
  return {
    configured,
    ready: requiredItems.length > 0 && configured === requiredItems.length,
    total: requiredItems.length,
  }
}

function settingConfigured(item: SettingItem, drafts: Record<string, string>) {
  return Boolean((drafts[item.key] ?? itemValue(item)).trim() || item.has_override || item.env_fallback || item.default)
}

function placeholderForPaymentField(item: SettingItem, t: ReturnType<typeof useTranslation>['t']) {
  if (item.secret && item.has_override) return t('admin.settings.payment.secretStored')
  if (item.secret) return t('admin.settings.payment.secretPlaceholder')
  if (item.key === PAYMENT_KEYS.alipayGateway) return 'https://openapi.alipay.com/gateway.do'
  if (item.key === PAYMENT_KEYS.alipayNotifyURL) return 'https://panel.example.com/api/public/payment/alipay/notify'
  if (item.key === PAYMENT_KEYS.stripeCurrency) return 'usd'
  if (item.key === PAYMENT_KEYS.stripeSuccessURL) return 'https://panel.example.com/portal/orders'
  if (item.key === PAYMENT_KEYS.stripeCancelURL) return 'https://panel.example.com/portal/orders'
  if (item.key === PAYMENT_KEYS.stripeSessionExpiryMinutes) return '30'
  return ''
}
