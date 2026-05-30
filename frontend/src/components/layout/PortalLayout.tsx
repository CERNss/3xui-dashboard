import { Drawer, Layout, theme } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { portalAuthApi } from '@/api/portal/auth'
import { useBranding } from '@/hooks/queries/branding'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { useThemeStore } from '@/stores/theme'
import { MD_BREAKPOINT } from '@/theme'
import { AppSidebar } from './AppSidebar'
import { AppTopbar } from './AppTopbar'
import { portalItems, selectedKey } from './nav'

const { Header, Sider, Content } = Layout

export function PortalLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const wide = useMinWidth(MD_BREAKPOINT)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const clearAuth = usePortalAuthStore((state) => state.clear)
  const user = usePortalAuthStore((state) => state.user)
  const themeMode = useThemeStore((state) => state.resolvedTheme)
  const toggleTheme = useThemeStore((state) => state.toggle)
  const { data: branding } = useBranding()
  const { token } = theme.useToken()
  const items = useMemo(() => portalItems(t), [t])
  // The portal nav is flat (no section grouping). Wrap it in a single
  // unlabeled NavSection so AppSidebar's grouped render path can take
  // it as-is without conditionals.
  const sections = useMemo(() => [{ key: 'portal', label: '', items }], [items])
  const selected = selectedKey(location.pathname, items)
  const activeLink = items.find((item) => item.key === selected)
  const accountLabel = user?.email ?? ''

  function selectRoute(key: string) {
    navigate(key)
    setDrawerOpen(false)
  }

  async function logout() {
    try {
      await portalAuthApi.logout()
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
      navLabel={t('portal.shell.navigation')}
    />
  )

  return (
    <Layout className="admin-shell" data-testid="portal-layout">
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
            accountRole={t('account.userRole')}
            accountItems={[{ label: t('account.profile'), to: '/portal/profile' }]}
            onLogout={logout}
            onOpenMobileNav={!wide ? () => setDrawerOpen(true) : undefined}
          />
        </Header>
        <Content className="admin-shell-content" style={{ background: token.colorBgLayout }}>
          <Outlet />
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
