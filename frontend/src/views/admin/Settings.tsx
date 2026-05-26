import { Alert, Skeleton, Tabs } from 'antd'
import type { TabsProps } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { SettingItem } from '@/api/admin/settings'
import { PageHeader, RefreshButton } from '@/components/common'
import { useClearSetting, useSetSetting, useSettingsList } from '@/hooks/queries/admin/settings'
import { AlertsSettings } from './settings/AlertsSettings'
import { DataCollectionSettings } from './settings/DataCollectionSettings'
import { GeneralSettings } from './settings/GeneralSettings'
import { MessagesSettings } from './settings/MessagesSettings'
import { NotificationsSettings } from './settings/NotificationsSettings'
import { SecurityAuthSettings } from './settings/SecurityAuthSettings'
import { SubscriptionSettings } from './settings/SubscriptionSettings'
import { UserDefaultsSettings } from './settings/UserDefaultsSettings'
import { SETTINGS_TABS, filterSettings, itemValue, tabI18nKeys, tabLabels } from './settings/settingHelpers'
import type { Drafts, SettingsSectionProps, SettingsTab } from './settings/types'

function isSettingsTab(value: string | null): value is SettingsTab {
  return SETTINGS_TABS.includes(value as SettingsTab)
}

function makeDrafts(items: SettingItem[]) {
  return items.reduce<Drafts>((drafts, item) => {
    drafts[item.key] = itemValue(item)
    return drafts
  }, {})
}

export default function Settings() {
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = isSettingsTab(searchParams.get('tab')) ? searchParams.get('tab') : 'general'
  const [drafts, setDrafts] = useState<Drafts>({})
  const [savingKey, setSavingKey] = useState<string | null>(null)

  const settingsQuery = useSettingsList()
  const setSetting = useSetSetting()
  const clearSetting = useClearSetting()
  const settings = useMemo(() => settingsQuery.data ?? [], [settingsQuery.data])

  useEffect(() => {
    setDrafts(makeDrafts(settings))
  }, [settings])

  const onDraftChange = (key: string, value: string) => {
    setDrafts((current) => ({ ...current, [key]: value }))
  }

  const save = async (item: SettingItem) => {
    setSavingKey(item.key)
    try {
      await setSetting.mutateAsync({ key: item.key, value: drafts[item.key] ?? '' })
    } finally {
      setSavingKey(null)
    }
  }

  const reset = async (item: SettingItem) => {
    setSavingKey(item.key)
    try {
      await clearSetting.mutateAsync(item.key)
    } finally {
      setSavingKey(null)
    }
  }

  const setActiveTab = (tab: string) => {
    if (!isSettingsTab(tab)) return
    const next = new URLSearchParams(searchParams)
    if (tab === 'general') {
      next.delete('tab')
    } else {
      next.set('tab', tab)
    }
    setSearchParams(next, { replace: true })
  }

  const sectionProps = (tab: SettingsTab): SettingsSectionProps => ({
    items: filterSettings(settings, tab),
    drafts,
    savingKey,
    onDraftChange,
    onSave: save,
    onReset: reset,
  })

  const items: TabsProps['items'] = SETTINGS_TABS.map((tab) => ({
    key: tab,
    label: <span data-i18n-key={tabI18nKeys[tab]}>{tabLabels[tab]}</span>,
    children:
      tab === 'general' ? (
        <GeneralSettings {...sectionProps(tab)} />
      ) : tab === 'subscription' ? (
        <SubscriptionSettings {...sectionProps(tab)} />
      ) : tab === 'alerts' ? (
        <AlertsSettings {...sectionProps(tab)} />
      ) : tab === 'dataCollection' ? (
        <DataCollectionSettings {...sectionProps(tab)} />
      ) : tab === 'securityAuth' ? (
        <SecurityAuthSettings {...sectionProps(tab)} />
      ) : tab === 'userDefaults' ? (
        <UserDefaultsSettings {...sectionProps(tab)} />
      ) : tab === 'messages' ? (
        <MessagesSettings {...sectionProps(tab)} />
      ) : (
        <NotificationsSettings />
      ),
  }))

  const error = settingsQuery.error ?? setSetting.error ?? clearSetting.error

  return (
    <div>
      <PageHeader
        title="Settings"
        subtitle="Server-driven admin settings grouped by operator workflow."
        actions={<RefreshButton loading={settingsQuery.isFetching} onClick={() => settingsQuery.refetch()} />}
      />
      {error ? <Alert type="error" showIcon message="Settings operation failed" style={{ marginBottom: 16 }} /> : null}
      {settingsQuery.isLoading ? (
        <Skeleton active />
      ) : (
        <Tabs
          activeKey={activeTab ?? 'general'}
          destroyOnHidden={false}
          items={items}
          onChange={setActiveTab}
        />
      )}
    </div>
  )
}
