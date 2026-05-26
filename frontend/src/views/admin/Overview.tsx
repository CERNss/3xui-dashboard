import { Tabs } from 'antd'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import { PageHeader, RefreshButton } from '@/components/common'
import { StatsPanel } from './overview/StatsPanel'
import { type OverviewPanelHandle, StatusPanel } from './overview/StatusPanel'

type OverviewTab = 'status' | 'stats'

interface OverviewProps {
  defaultTab?: OverviewTab
}

function tabFromSearch(value: string | null): OverviewTab | null {
  return value === 'stats' || value === 'status' ? value : null
}

export default function Overview({ defaultTab = 'status' }: OverviewProps) {
  const { t } = useTranslation()
  const [searchParams, setSearchParams] = useSearchParams()
  const initialTab = tabFromSearch(searchParams.get('tab')) ?? defaultTab
  const [activeTab, setActiveTab] = useState<OverviewTab>(initialTab)
  const [mountedTabs, setMountedTabs] = useState<Set<OverviewTab>>(() => new Set([initialTab]))
  const [fetchingByTab, setFetchingByTab] = useState<Record<OverviewTab, boolean>>({ status: false, stats: false })
  const statusRef = useRef<OverviewPanelHandle>(null)
  const statsRef = useRef<OverviewPanelHandle>(null)

  useEffect(() => {
    const next = tabFromSearch(searchParams.get('tab')) ?? defaultTab
    setActiveTab(next)
    setMountedTabs((current) => new Set(current).add(next))
  }, [defaultTab, searchParams])

  const activeRef = activeTab === 'status' ? statusRef : statsRef

  const setStatusFetching = useCallback((fetching: boolean) => {
    setFetchingByTab((current) => (current.status === fetching ? current : { ...current, status: fetching }))
  }, [])

  const setStatsFetching = useCallback((fetching: boolean) => {
    setFetchingByTab((current) => (current.stats === fetching ? current : { ...current, stats: fetching }))
  }, [])

  const activateTab = (tab: string) => {
    const next = tab as OverviewTab
    setActiveTab(next)
    setMountedTabs((current) => new Set(current).add(next))
    if (next === 'status') {
      setSearchParams({}, { replace: true })
    } else {
      setSearchParams({ tab: next }, { replace: true })
    }
  }

  return (
    <div>
      <PageHeader
        title={t('admin.status.title')}
        subtitle={t('admin.overview.subtitle')}
        actions={
          <RefreshButton
            loading={fetchingByTab[activeTab]}
            label={t('admin.status.reload')}
            onClick={() => activeRef.current?.reload()}
          />
        }
      />

      <Tabs
        activeKey={activeTab}
        onChange={activateTab}
        items={[
          { key: 'status', label: t('admin.status.tab') },
          { key: 'stats', label: t('admin.stats.tab') },
        ]}
      />

      <section data-overview-content>
        {mountedTabs.has('status') ? (
          <div role="tabpanel" aria-label={t('admin.status.panelLabel')} hidden={activeTab !== 'status'} style={{ display: activeTab === 'status' ? undefined : 'none' }}>
            <StatusPanel
              ref={statusRef}
              onFetchingChange={setStatusFetching}
            />
          </div>
        ) : null}
        {mountedTabs.has('stats') ? (
          <div role="tabpanel" aria-label={t('admin.stats.panelLabel')} hidden={activeTab !== 'stats'} style={{ display: activeTab === 'stats' ? undefined : 'none' }}>
            <StatsPanel
              ref={statsRef}
              onFetchingChange={setStatsFetching}
            />
          </div>
        ) : null}
      </section>
    </div>
  )
}
