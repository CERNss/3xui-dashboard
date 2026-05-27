import {
  BellOutlined,
  DownOutlined,
  GlobalOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  SunOutlined,
} from '@ant-design/icons'
import { Button, Drawer, Layout, Tooltip, Typography, theme } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { AccountMenu, LocaleSwitcher, PageHeaderChromeProvider } from '@/components/common'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { useBranding } from '@/hooks/queries/branding'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useThemeStore } from '@/stores/theme'
import { MD_BREAKPOINT } from '@/theme'
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
    <AdminSidebar
      collapsed={collapsed}
      onCollapseToggle={() => setCollapsed((value) => !value)}
      onNavigate={selectRoute}
      onThemeToggle={toggleTheme}
      selectedKey={selected}
      sections={sections}
      themeMode={themeMode}
      title={branding?.title ?? t('app.title')}
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
          <div className="admin-topbar-heading">
            {!wide ? (
              <Button
                aria-label={t('a11y.openNav')}
                className="admin-topbar-menu"
                icon={<MenuFoldOutlined />}
                onClick={() => setDrawerOpen(true)}
                type="text"
              />
            ) : null}
            <div className="admin-topbar-copy">
              <Typography.Title className="admin-topbar-title" level={1}>
                {activeLink?.label ?? t('nav.dashboard')}
              </Typography.Title>
              <Typography.Text className="admin-topbar-subtitle">
                {t('admin.topbarWelcome')}
              </Typography.Text>
            </div>
          </div>
          <div className="admin-topbar-tools">
            <Tooltip title={t('admin.notifications')}>
              <button aria-label={t('admin.notifications')} className="admin-topbar-icon-button" type="button">
                <BellOutlined />
              </button>
            </Tooltip>
            <LocaleSwitcher variant="chip" />
            <AccountMenu
              items={[{ label: t('account.profile'), to: '/admin/settings?tab=securityAuth' }]}
              logoutLabel={t('nav.logout')}
              onLogout={logout}
            >
              <button aria-label={t('account.openMenu')} className="admin-topbar-account" type="button">
                <span aria-hidden="true" className="admin-topbar-avatar">
                  {initialsForAccount(accountLabel)}
                </span>
                <span className="admin-topbar-account-copy">
                  <span className="admin-topbar-account-name">{displayAccountName(accountLabel)}</span>
                  <span className="admin-topbar-account-role">{t('account.adminRole')}</span>
                </span>
                <DownOutlined aria-hidden="true" className="admin-topbar-account-chevron" />
              </button>
            </AccountMenu>
          </div>
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

function displayAccountName(account: string) {
  const localPart = account.split('@')[0] || account
  return localPart.length > 12 ? `${localPart.slice(0, 12)}...` : localPart
}

function initialsForAccount(account: string) {
  const source = displayAccountName(account).replace(/[^a-zA-Z0-9]/g, '')
  return (source.slice(0, 2) || 'AD').toUpperCase()
}

interface AdminSidebarProps {
  collapsed: boolean
  onCollapseToggle: () => void
  onNavigate: (key: string) => void
  onThemeToggle: () => void
  selectedKey?: string
  sections: ReturnType<typeof adminSections>
  themeMode: 'light' | 'dark'
  title: string
}

function AdminSidebar({
  collapsed,
  onCollapseToggle,
  onNavigate,
  onThemeToggle,
  selectedKey,
  sections,
  themeMode,
  title,
}: AdminSidebarProps) {
  const { t } = useTranslation()

  return (
    <div className="admin-sidebar" data-collapsed={collapsed ? 'true' : 'false'}>
      <div className="admin-sidebar-brand">
        <span className="admin-sidebar-logo" aria-hidden="true">
          <GlobalOutlined />
        </span>
        {!collapsed ? (
          <div className="admin-sidebar-brand-copy">
            <Typography.Title className="admin-sidebar-title" level={4}>
              {title}
            </Typography.Title>
          </div>
        ) : null}
      </div>

      <nav aria-label={t('nav.admin')} className="admin-sidebar-nav">
        {sections.map((section) => (
          <div className="admin-sidebar-section" key={section.key}>
            {!collapsed ? <div className="admin-sidebar-section-label">{section.label}</div> : null}
            <div className="admin-sidebar-items">
              {section.items.map((item) => {
                const active = selectedKey === item.key
                const button = (
                  <button
                    aria-current={active ? 'page' : undefined}
                    className="admin-sidebar-item"
                    data-active={active ? 'true' : 'false'}
                    key={item.key}
                    onClick={() => onNavigate(item.key)}
                    type="button"
                  >
                    <span aria-hidden="true" className="admin-sidebar-item-icon">
                      {item.icon}
                    </span>
                    {!collapsed ? <span className="admin-sidebar-item-label">{item.label}</span> : null}
                  </button>
                )

                return collapsed ? (
                  <Tooltip key={item.key} placement="right" title={item.label}>
                    {button}
                  </Tooltip>
                ) : (
                  button
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="admin-sidebar-footer">
        <button className="admin-sidebar-action" onClick={onThemeToggle} type="button">
          <span aria-hidden="true" className="admin-sidebar-item-icon">
            {themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
          </span>
          {!collapsed ? <span>{themeMode === 'dark' ? t('theme.light') : t('theme.dark')}</span> : null}
        </button>
        <button className="admin-sidebar-action" onClick={onCollapseToggle} type="button">
          <span aria-hidden="true" className="admin-sidebar-item-icon">
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </span>
          {!collapsed ? <span>{collapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')}</span> : null}
        </button>
      </div>
    </div>
  )
}
