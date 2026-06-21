import { Drawer, Layout, theme } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { PageHeaderChromeProvider } from '@/components/common'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { adminAuthApi } from '@/api/admin/auth'
import { useBranding } from '@/hooks/queries/branding'
import { useDashboardAutoRefresh } from '@/hooks/queries/admin/settings'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useThemeStore } from '@/stores/theme'
import { MD_BREAKPOINT } from '@/theme'
import { AppSidebar } from './AppSidebar'
import { AppTopbar } from './AppTopbar'
import { adminSections, flattenSections, selectedKey } from './nav'

const { Header, Sider, Content } = Layout

/* Cluster-status pill under the sidebar brand: animated signal bars,
 * "N nodes online", and a health score badge (% of nodes online).
 * Renders nothing until the node list has loaded. */
function ClusterStatus() {
  const { t } = useTranslation()
  const nodes = useNodesList()
  if (!nodes.data || nodes.data.length === 0) return null
  const online = nodes.data.filter((node) => node.status === 'online').length
  const health = Math.round((online / nodes.data.length) * 100)
  return (
    <div className="admin-cluster" data-health={health >= 100 ? 'ok' : health > 0 ? 'warn' : 'bad'}>
      <span aria-hidden="true" className="admin-cluster-signal">
        <i />
        <i />
        <i />
        <i />
      </span>
      <span className="admin-cluster-text">{t('nav.clusterOnline', { count: online })}</span>
      <span className="admin-cluster-badge">{health}</span>
    </div>
  )
}

export function AdminLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const wide = useMinWidth(MD_BREAKPOINT)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const clearAuth = useAdminAuthStore((state) => state.clear)
  const username = useAdminAuthStore((state) => state.username)
  const themeMode = useThemeStore((state) => state.resolvedTheme)
  const toggleTheme = useThemeStore((state) => state.toggle)
  const { data: branding } = useBranding()
  useDashboardAutoRefresh()
  const { token } = theme.useToken()
  const sections = useMemo(() => adminSections(t), [t])
  const links = useMemo(() => flattenSections(sections), [sections])
  const selected = selectedKey(location.pathname, links)
  const activeLink = links.find((item) => item.key === selected)
  const activeSection = sections.find((section) => section.items.some((item) => item.key === selected))
  const accountLabel = username || 'admin'

  function selectRoute(key: string) {
    navigate(key)
    setDrawerOpen(false)
  }

  async function logout() {
    // Clear the httpOnly cookie server-side; clear local identity
    // regardless of whether that call succeeds.
    try {
      await adminAuthApi.logout()
    } catch {
      /* ignore — we clear locally below either way */
    }
    clearAuth()
    navigate('/login', { replace: true })
  }

  const sidebar = (
    <AppSidebar
      collapsed={collapsed}
      onCollapseToggle={() => setCollapsed((value) => !value)}
      onNavigate={selectRoute}
      onThemeToggle={toggleTheme}
      selectedKey={selected}
      sections={sections}
      themeMode={themeMode}
      title={branding?.title ?? t('app.title')}
      subtitle="node orchestration"
      clusterSlot={<ClusterStatus />}
      navLabel={t('nav.admin')}
    />
  )

  return (
    <Layout className="admin-shell" data-testid="admin-layout">
      {wide ? (
        <Sider
          className="admin-shell-sider"
          collapsed={collapsed}
          collapsedWidth={80}
          theme="dark"
          trigger={null}
          width={228}
        >
          {sidebar}
        </Sider>
      ) : null}
      <Layout className="admin-shell-main">
        <Header className="admin-topbar">
          <AppTopbar
            title={activeLink?.label ?? t('nav.dashboard')}
            breadcrumbGroup={activeSection?.label}
            accountLabel={accountLabel}
            accountRole={t('account.adminRole')}
            accountItems={[{ label: t('account.profile'), to: '/admin/settings?tab=securityAuth' }]}
            onLogout={logout}
            onOpenMobileNav={!wide ? () => setDrawerOpen(true) : undefined}
            showNotifications
          />
        </Header>
        <Content className="admin-shell-content" style={{ background: token.colorBgLayout }}>
          <PageHeaderChromeProvider suppressContentHeading>
            <Outlet />
          </PageHeaderChromeProvider>
        </Content>
      </Layout>
      <Drawer
        className="admin-shell-drawer"
        closable={false}
        placement="left"
        open={!wide && drawerOpen}
        onClose={() => setDrawerOpen(false)}
        styles={{ body: { padding: 0 }, content: { background: 'var(--elev)' } }}
        width={228}
      >
        {sidebar}
      </Drawer>
    </Layout>
  )
}
