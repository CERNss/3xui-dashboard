import { Drawer, Layout, theme } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { PageHeaderChromeProvider } from '@/components/common'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { useBranding } from '@/hooks/queries/branding'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useThemeStore } from '@/stores/theme'
import { MD_BREAKPOINT } from '@/theme'
import { AppSidebar } from './AppSidebar'
import { AppTopbar } from './AppTopbar'
import { adminSections, flattenSections, selectedKey } from './nav'

const { Header, Sider, Content } = Layout

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
  const { token } = theme.useToken()
  const sections = useMemo(() => adminSections(t), [t])
  const links = useMemo(() => flattenSections(sections), [sections])
  const selected = selectedKey(location.pathname, links)
  const activeLink = links.find((item) => item.key === selected)
  const accountLabel = username || 'admin'

  function selectRoute(key: string) {
    navigate(key)
    setDrawerOpen(false)
  }

  function logout() {
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
          width={252}
        >
          {sidebar}
        </Sider>
      ) : null}
      <Layout className="admin-shell-main">
        <Header className="admin-topbar">
          <AppTopbar
            title={activeLink?.label ?? t('nav.dashboard')}
            subtitle={t('admin.topbarWelcome')}
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
        styles={{ body: { padding: 0 }, content: { background: themeMode === 'dark' ? '#081321' : '#f8fafc' } }}
        width={252}
      >
        {sidebar}
      </Drawer>
    </Layout>
  )
}

