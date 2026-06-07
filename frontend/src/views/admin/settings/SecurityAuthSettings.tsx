import { Button, Card, Input, Select, Space, Switch, Typography, message } from 'antd'
import { CopyOutlined } from '@ant-design/icons'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import {
  PreferenceActions,
  PreferencePanel,
  PreferenceSwitchRow,
  PreferenceTagsRow,
  changedItems,
} from './PreferencePanel'
import { OIDC_KEYS, REGISTRATION_KEYS, itemValue } from './settingHelpers'
import type { SettingsSectionProps } from './types'

const oidcKeyOrder = [
  'oidc_enabled',
  'oidc_display_name',
  'oidc_client_id',
  'oidc_client_secret',
  'oidc_issuer',
  'oidc_auth_url',
  'oidc_token_url',
  'oidc_userinfo_url',
  'oidc_jwks_url',
  'oidc_scopes',
  'oidc_redirect_url',
  'oidc_icon_url',
]

const tokenAuthOptions = [{ label: 'client_secret_post', value: 'client_secret_post' }]
const signingAlgorithmOptions = [{ label: 'RS256', value: 'RS256' }]

export function SecurityAuthSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const registrationItems = props.items.filter((item) => REGISTRATION_KEYS.has(item.key))
  const oidcItems = useMemo(
    () =>
      props.items
        .filter((item) => OIDC_KEYS.has(item.key))
        .sort((a, b) => oidcKeyOrder.indexOf(a.key) - oidcKeyOrder.indexOf(b.key)),
    [props.items],
  )

  return (
    <div className="settings-section-stack">
      <header className="settings-section-intro">
        <Typography.Title level={2}>{t('admin.settings.securityAuthTitle')}</Typography.Title>
        <Typography.Text>{t('admin.settings.securityAuthDesc')}</Typography.Text>
      </header>
      <RegistrationSettingsPanel {...props} items={registrationItems} />
      <OIDCSettingsPanel {...props} items={oidcItems} />
    </div>
  )
}

function RegistrationSettingsPanel({
  drafts,
  items,
  savingKey,
  onDraftChange,
  onReset,
  onSave,
}: SettingsSectionProps) {
  const { t } = useTranslation()
  const byKey = useMemo(() => new Map(items.map((item) => [item.key, item])), [items])
  const changed = changedItems(items, drafts)
  const saving = items.some((item) => savingKey === item.key)

  const saveAll = () => {
    for (const item of changed) onSave(item)
  }

  const resetAll = () => {
    for (const item of items.filter((item) => item.has_override)) onReset(item)
  }

  return (
    <PreferencePanel
      id="registration"
      title={t('admin.settings.registration.title')}
      description={t('admin.settings.registration.subtitle')}
      footer={
        <PreferenceActions
          resetLabel={t('admin.settings.reset')}
          saveLabel={t('admin.settings.registration.save')}
          saving={saving}
          canReset={items.some((item) => item.has_override)}
          canSave={changed.length > 0}
          onReset={resetAll}
          onSave={saveAll}
        />
      }
    >
      <PreferenceSwitchRow
        item={byKey.get('public_registration_enabled')}
        drafts={drafts}
        title={t('admin.settings.registration.publicTitle')}
        description={t('admin.settings.registration.publicDesc')}
        onText={t('admin.settings.registration.enabled')}
        offText={t('admin.settings.registration.disabled')}
        onDraftChange={onDraftChange}
      />
      <PreferenceSwitchRow
        item={byKey.get('email_verification_required')}
        drafts={drafts}
        title={t('admin.settings.registration.verifyTitle')}
        description={t('admin.settings.registration.verifyDesc')}
        onText={t('admin.settings.registration.required')}
        offText={t('admin.settings.registration.optional')}
        onDraftChange={onDraftChange}
      />
      <PreferenceTagsRow
        item={byKey.get('email_domain_allowlist')}
        drafts={drafts}
        title={t('admin.settings.registration.allowlistTitle')}
        description={t('admin.settings.registration.allowlistDesc')}
        hint={t('admin.settings.registration.allowlistHint')}
        placeholder={t('admin.settings.registration.allowlistPlaceholder')}
        onDraftChange={onDraftChange}
      />
    </PreferencePanel>
  )
}

function OIDCSettingsPanel({
  items,
  drafts,
  savingKey,
  onDraftChange,
  onSave,
  onReset,
}: SettingsSectionProps) {
  const { t } = useTranslation()
  const [messageApi, contextHolder] = message.useMessage()
  const byKey = useMemo(() => new Map(items.map((item) => [item.key, item])), [items])
  const issuer = draftValue(byKey.get('oidc_issuer'), drafts)
  const redirectURL = draftValue(byKey.get('oidc_redirect_url'), drafts)
  const discoveryURL = issuer ? `${issuer.replace(/\/+$/, '')}/.well-known/openid-configuration` : ''
  // A field counts as configured if it has a draft value OR a stored
  // override. The has_override check matters for secrets (oidc_client_secret),
  // whose value is masked to "" in the List response — without it, a fully
  // configured provider would read as unconfigured here.
  const configured = ['oidc_issuer', 'oidc_client_id', 'oidc_client_secret', 'oidc_redirect_url'].every((key) => {
    const item = byKey.get(key)
    return Boolean(draftValue(item, drafts)) || Boolean(item?.has_override)
  })
  const enabledItem = byKey.get('oidc_enabled')
  const enabledDraft = draftValue(enabledItem, drafts)
  const oidcEnabled = enabledDraft === '' ? configured : isTruthy(enabledDraft)
  const saving = items.some((item) => savingKey === item.key)

  const saveAll = async () => {
    for (const item of items) {
      if (draftValue(item, drafts) !== itemValue(item)) {
        await onSave(item)
      }
    }
    messageApi.success(t('admin.settings.oidc.saved'))
  }

  const copyRedirect = async () => {
    if (!redirectURL) return
    await navigator.clipboard?.writeText(redirectURL)
    messageApi.success(t('common.copied'))
  }

  return (
    <Card className="settings-oidc-panel" data-testid="oidc-settings-panel">
      {contextHolder}
      <div className="settings-oidc-header">
        <div>
          <Typography.Title level={4}>{t('admin.settings.oidc.title')}</Typography.Title>
          <Typography.Text type="secondary">{t('admin.settings.oidc.desc')}</Typography.Text>
        </div>
        <Switch
          checked={oidcEnabled}
          aria-label={t('admin.settings.oidc.enabledToggle')}
          onChange={(checked) => {
            if (enabledItem) onDraftChange(enabledItem.key, checked ? 'true' : 'false')
          }}
        />
      </div>

      <div className="settings-oidc-divider" />

      <div className="settings-oidc-grid settings-oidc-grid--three">
        <OIDCField item={byKey.get('oidc_display_name')} drafts={drafts} label={t('admin.settings.oidc.providerName')} onDraftChange={onDraftChange} />
        <OIDCField item={byKey.get('oidc_client_id')} drafts={drafts} label={t('admin.settings.oidc.clientId')} onDraftChange={onDraftChange} />
        <OIDCField
          item={byKey.get('oidc_client_secret')}
          drafts={drafts}
          label={t('admin.settings.oidc.clientSecret')}
          password
          hint={t('admin.settings.oidc.clientSecretHint')}
          onDraftChange={onDraftChange}
        />
      </div>

      <div className="settings-oidc-grid settings-oidc-grid--two">
        <OIDCField item={byKey.get('oidc_issuer')} drafts={drafts} label={t('admin.settings.oidc.issuer')} onDraftChange={onDraftChange} />
        <ReadOnlyField label={t('admin.settings.oidc.discoveryUrl')} value={discoveryURL} />
        <OIDCField item={byKey.get('oidc_auth_url')} drafts={drafts} label={t('admin.settings.oidc.authUrl')} onDraftChange={onDraftChange} />
        <OIDCField item={byKey.get('oidc_token_url')} drafts={drafts} label={t('admin.settings.oidc.tokenUrl')} onDraftChange={onDraftChange} />
        <OIDCField item={byKey.get('oidc_userinfo_url')} drafts={drafts} label={t('admin.settings.oidc.userinfoUrl')} onDraftChange={onDraftChange} />
        <OIDCField item={byKey.get('oidc_jwks_url')} drafts={drafts} label={t('admin.settings.oidc.jwksUrl')} onDraftChange={onDraftChange} />
        <OIDCField
          item={byKey.get('oidc_scopes')}
          drafts={drafts}
          label={t('admin.settings.oidc.scopes')}
          hint={t('admin.settings.oidc.scopesHint')}
          onDraftChange={onDraftChange}
        />
        <OIDCField item={byKey.get('oidc_icon_url')} drafts={drafts} label={t('admin.settings.oidc.iconUrl')} onDraftChange={onDraftChange} />
      </div>

      {/* Redirect URI stands on its own full-width row — it carries a
       * Copy button + hint that make it taller than every other field,
       * pairing it with anything else inside a 2-column grid produced
       * uneven row heights. */}
      <div className="settings-oidc-field settings-oidc-field--full">
        <OIDCField item={byKey.get('oidc_redirect_url')} drafts={drafts} label={t('admin.settings.oidc.redirectUrl')} onDraftChange={onDraftChange} />
        <div className="settings-oidc-copy-row">
          <Button icon={<CopyOutlined />} disabled={!redirectURL} onClick={copyRedirect}>
            {t('admin.settings.oidc.copyRedirect')}
          </Button>
          <Typography.Text code>{redirectURL || t('admin.settings.oidc.redirectPlaceholder')}</Typography.Text>
        </div>
        <Typography.Text type="secondary" className="settings-oidc-hint">
          {t('admin.settings.oidc.redirectHint')}
        </Typography.Text>
      </div>

      <div className="settings-oidc-grid settings-oidc-grid--three">
        <ReadOnlySelect label={t('admin.settings.oidc.tokenAuth')} value="client_secret_post" options={tokenAuthOptions} />
        <ReadOnlyField label={t('admin.settings.oidc.clockSkew')} value="120" />
        <ReadOnlySelect label={t('admin.settings.oidc.signingAlgorithms')} value="RS256" options={signingAlgorithmOptions} />
      </div>

      {/* 4 switches → 4-column grid so they all sit on one row instead
       * of the 4th wrapping below alone in a 3-column grid. */}
      <div className="settings-oidc-toggle-grid settings-oidc-toggle-grid--four">
        <ReadOnlySwitch label={t('admin.settings.oidc.pkce')} checked />
        <ReadOnlySwitch label={t('admin.settings.oidc.verifyIdToken')} checked />
        <ReadOnlySwitch label={t('admin.settings.oidc.requireVerifiedEmail')} checked />
        <ReadOnlySwitch label={t('admin.settings.oidc.localEmailVerification')} checked={false} />
      </div>

      <div className="settings-oidc-grid settings-oidc-grid--three">
        <ReadOnlyField label={t('admin.settings.oidc.userinfoEmailPath')} value="email" />
        <ReadOnlyField label={t('admin.settings.oidc.userinfoIdPath')} value="sub" />
        <ReadOnlyField label={t('admin.settings.oidc.userinfoNamePath')} value="preferred_username" />
      </div>

      <div className="settings-oidc-footer">
        <Space size={10}>
          {items.some((item) => item.has_override) ? (
            <Button loading={saving} onClick={() => items.filter((item) => item.has_override).forEach((item) => onReset(item))}>
              {t('admin.settings.reset')}
            </Button>
          ) : null}
          <Button type="primary" loading={saving} disabled={!items.some((item) => draftValue(item, drafts) !== itemValue(item))} onClick={saveAll}>
            {t('admin.settings.oidc.save')}
          </Button>
        </Space>
      </div>
    </Card>
  )
}

function OIDCField({
  item,
  drafts,
  label,
  hint,
  password,
  onDraftChange,
}: {
  item?: SettingItem
  drafts: Record<string, string>
  label: string
  hint?: string
  password?: boolean
  onDraftChange: (key: string, value: string) => void
}) {
  if (!item) return null
  const value = draftValue(item, drafts)
  const controlID = `oidc-${item.key}`
  const input = password ? (
    <Input.Password
      id={controlID}
      autoComplete="new-password"
      value={value}
      placeholder={item.has_override || item.env_fallback ? '********' : ''}
      onChange={(event) => onDraftChange(item.key, event.target.value)}
    />
  ) : (
    <Input id={controlID} value={value} onChange={(event) => onDraftChange(item.key, event.target.value)} />
  )
  return (
    <label className="settings-oidc-field" htmlFor={controlID}>
      <span>{label}</span>
      {input}
      {hint ? <Typography.Text type="secondary">{hint}</Typography.Text> : null}
    </label>
  )
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  return (
    <label className="settings-oidc-field">
      <span>{label}</span>
      <Input value={value} readOnly />
    </label>
  )
}

function ReadOnlySelect({ label, value, options }: { label: string; value: string; options: { label: string; value: string }[] }) {
  return (
    <label className="settings-oidc-field">
      <span>{label}</span>
      <Select value={value} options={options} disabled />
    </label>
  )
}

function ReadOnlySwitch({ label, checked }: { label: string; checked: boolean }) {
  return (
    <div className="settings-oidc-switch">
      <Typography.Text strong>{label}</Typography.Text>
      <Switch checked={checked} disabled />
    </div>
  )
}

function draftValue(item: SettingItem | undefined, drafts: Record<string, string>) {
  if (!item) return ''
  return drafts[item.key] ?? itemValue(item)
}

function isTruthy(value: string) {
  return ['true', '1', 'yes', 'on'].includes(value.trim().toLowerCase())
}
