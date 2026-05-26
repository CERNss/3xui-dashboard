import { Tabs } from 'antd'
import { useCallback, useEffect, useRef, useState } from 'react'
import { PageHeader, RefreshButton } from '@/components/common'
import { StatsPanel } from './overview/StatsPanel'
import { type OverviewPanelHandle, StatusPanel } from './overview/StatusPanel'

type OverviewTab = 'status' | 'stats'

interface OverviewProps {
  defaultTab?: OverviewTab
}

const tabCopy: Record<OverviewTab, { title: string; subtitle: string }> = {
  status: {
    title: 'Status',
    subtitle: 'Monitor fleet health, inbounds, clients, and node probes.',
  },
  stats: {
    title: 'Stats',
    subtitle: 'Review operational KPIs, traffic rankings, audit activity, and recent orders.',
  },
}

export default function Overview({ defaultTab = 'status' }: OverviewProps) {
  const [activeTab, setActiveTab] = useState<OverviewTab>(defaultTab)
  const [mountedTabs, setMountedTabs] = useState<Set<OverviewTab>>(() => new Set([defaultTab]))
  const [fetchingByTab, setFetchingByTab] = useState<Record<OverviewTab, boolean>>({ status: false, stats: false })
  const statusRef = useRef<OverviewPanelHandle>(null)
  const statsRef = useRef<OverviewPanelHandle>(null)

  useEffect(() => {
    setActiveTab(defaultTab)
    setMountedTabs((current) => new Set(current).add(defaultTab))
  }, [defaultTab])

  const activeRef = activeTab === 'status' ? statusRef : statsRef
  const activeCopy = tabCopy[activeTab]

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
  }

  return (
    <div>
      <PageHeader
        title={activeCopy.title}
        subtitle={activeCopy.subtitle}
        actions={
          <RefreshButton
            loading={fetchingByTab[activeTab]}
            onClick={() => activeRef.current?.reload()}
          />
        }
      />

      <Tabs
        activeKey={activeTab}
        onChange={activateTab}
        items={[
          { key: 'status', label: 'Status' },
          { key: 'stats', label: 'Stats' },
        ]}
      />

      <section data-overview-content>
        {mountedTabs.has('status') ? (
          <div role="tabpanel" aria-label="Status panel" hidden={activeTab !== 'status'} style={{ display: activeTab === 'status' ? undefined : 'none' }}>
            <StatusPanel
              ref={statusRef}
              onFetchingChange={setStatusFetching}
            />
          </div>
        ) : null}
        {mountedTabs.has('stats') ? (
          <div role="tabpanel" aria-label="Stats panel" hidden={activeTab !== 'stats'} style={{ display: activeTab === 'stats' ? undefined : 'none' }}>
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
