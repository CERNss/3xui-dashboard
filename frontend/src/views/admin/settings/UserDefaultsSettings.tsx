import { PlusOutlined } from '@ant-design/icons'
import { Button, Empty, Select, Space, Typography } from 'antd'
import type { RefSelectProps } from 'antd'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { itemValue, localizedLabel } from './settingHelpers'
import type { SettingsSectionProps } from './types'

const BALANCE_KEY = 'new_user_initial_balance_cents'
const STARTER_PLANS_KEY = 'new_user_plan_ids'

export function UserDefaultsSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const balanceItem = props.items.find((item) => item.key === BALANCE_KEY)
  const starterPlansItem = props.items.find((item) => item.key === STARTER_PLANS_KEY)
  const changed = props.items.filter((item) => draftValue(item, props.drafts) !== itemValue(item))
  const saving = props.items.some((item) => props.savingKey === item.key)
  const planSelectRef = useRef<RefSelectProps | null>(null)

  if (props.items.length === 0) {
    return (
      <div className="settings-section-stack">
        <header className="settings-section-intro">
          <Typography.Title level={2}>{t('admin.settings.userDefaultsTitle')}</Typography.Title>
          <Typography.Text>{t('admin.settings.userDefaultsDesc')}</Typography.Text>
        </header>
        <section className="settings-empty-panel">
          <Empty description={t('admin.settings.emptySection')} />
        </section>
      </div>
    )
  }

  const saveAll = () => {
    for (const item of changed) props.onSave(item)
  }

  const resetAll = () => {
    for (const item of props.items.filter((item) => item.has_override)) props.onReset(item)
  }

  return (
    <div className="settings-section-stack">
      <section className="settings-user-defaults-panel" aria-labelledby="settings-user-defaults-title">
        <header className="settings-user-defaults-header">
          <Typography.Title id="settings-user-defaults-title" level={3}>
            {t('admin.settings.userDefaultsTitle')}
          </Typography.Title>
          <Typography.Text>{t('admin.settings.userDefaultsDesc')}</Typography.Text>
        </header>

        <div className="settings-user-defaults-body">
          <div className="settings-user-defaults-grid">
            <NumberField
              item={balanceItem}
              drafts={props.drafts}
              title={t('admin.settings.newUserPolicy.defaultBalance')}
              description={t('admin.settings.newUserPolicy.defaultBalanceDesc')}
              unit={t('admin.settings.newUserPolicy.cents')}
              onDraftChange={props.onDraftChange}
            />
          </div>

          <section className="settings-user-defaults-list" aria-labelledby="settings-user-defaults-plans">
            <header className="settings-user-defaults-subheader">
              <div>
                <Typography.Title id="settings-user-defaults-plans" level={4}>
                  {t('admin.settings.newUserPolicy.defaultSubscriptionList')}
                </Typography.Title>
                <Typography.Text>{t('admin.settings.newUserPolicy.defaultSubscriptionListDesc')}</Typography.Text>
              </div>
              <Button icon={<PlusOutlined />} onClick={() => planSelectRef.current?.focus()}>
                {t('admin.settings.newUserPolicy.addDefaultSubscription')}
              </Button>
            </header>
            <PlanIdsField
              item={starterPlansItem}
              drafts={props.drafts}
              bindSelectRef={(node) => {
                planSelectRef.current = node
              }}
              onDraftChange={props.onDraftChange}
            />
          </section>
        </div>

        <footer className="settings-user-defaults-footer">
          <Space size={10} wrap>
            {props.items.some((item) => item.has_override) ? (
              <Button loading={saving} onClick={resetAll}>
                {t('admin.settings.reset')}
              </Button>
            ) : null}
            <Button type="primary" loading={saving} disabled={changed.length === 0} onClick={saveAll}>
              {t('admin.settings.newUserPolicy.save')}
            </Button>
          </Space>
        </footer>
      </section>
    </div>
  )
}

function NumberField({
  description,
  drafts,
  item,
  onDraftChange,
  title,
  unit,
}: {
  description: string
  drafts: Record<string, string>
  item?: SettingItem
  onDraftChange: (key: string, value: string) => void
  title: string
  unit: string
}) {
  if (!item) return null
  const { i18n } = useTranslation()
  const label = localizedLabel(item, i18n.language)
  return (
    <label className="settings-user-defaults-field" data-setting-key={item.key}>
      <span>{title}</span>
      <span className="settings-user-defaults-number">
        <input
          aria-label={label}
          min={0}
          type="number"
          value={draftValue(item, drafts)}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
        <span>{unit}</span>
      </span>
      <Typography.Text>{description}</Typography.Text>
    </label>
  )
}

function PlanIdsField({
  bindSelectRef,
  drafts,
  item,
  onDraftChange,
}: {
  bindSelectRef: (node: RefSelectProps | null) => void
  drafts: Record<string, string>
  item?: SettingItem
  onDraftChange: (key: string, value: string) => void
}) {
  const { i18n, t } = useTranslation()
  if (!item) return null
  const label = localizedLabel(item, i18n.language)
  const values = splitList(draftValue(item, drafts))

  return (
    <div className="settings-user-defaults-plan-field" data-setting-key={item.key}>
      <Select
        aria-label={label}
        ref={bindSelectRef}
        mode="tags"
        placeholder={t('admin.settings.newUserPolicy.starterPlansPlaceholder')}
        tokenSeparators={[',', ' ']}
        value={values}
        onChange={(next) => onDraftChange(item.key, joinList(next))}
      />
      <Typography.Text>{t('admin.settings.newUserPolicy.starterPlansHint')}</Typography.Text>
      {values.length === 0 ? (
        <div className="settings-user-defaults-empty-list">
          {t('admin.settings.newUserPolicy.defaultSubscriptionEmpty')}
        </div>
      ) : null}
    </div>
  )
}

function draftValue(item: SettingItem | undefined, drafts: Record<string, string>) {
  if (!item) return ''
  return drafts[item.key] ?? itemValue(item)
}

function splitList(value: string) {
  return value
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
}

function joinList(values: string[]) {
  return values
    .map((value) => value.trim())
    .filter(Boolean)
    .join(',')
}
