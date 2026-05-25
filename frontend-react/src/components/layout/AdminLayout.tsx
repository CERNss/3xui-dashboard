import { MenuFoldOutlined } from '@ant-design/icons'
import { Button, Drawer, Layout, Menu, Space, Switch, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { AccountMenu, LocaleSwitcher } from '@/components/common'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { useBranding } from '@/hooks/queries/branding'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useThemeStore } from '@/stores/theme'
import { MD_BREAKPOINT } from '@/theme'
import { adminSections, flattenSections, menuItemsFromSections, selectedKey } from './nav'

const { Header, Sider, Content } = Layout

export function AdminLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const wide = useMinWidth(MD_BREAKPOINT)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const clearAuth = useAdminAuthStore((state) => state.clear)
  const username = useAdminAuthStore((state) => state.username)
  const themeMode = useThemeStore((state) => state.resolvedTheme)
  const toggleTheme = useThemeStore((state) => state.toggle)
  const { data: branding } = useBranding()
  const sections = useMemo(() => adminSections(t), [t])
  const links = useMemo(() => flattenSections(sections), [sections])
  const selected = selectedKey(location.pathname, links)

  function selectRoute(key: string) {
    navigate(key)
    setDrawerOpen(false)
  }

  function logout() {
    clearAuth()
    navigate('/login', { replace: true })
  }

  const menu = (
    <Menu
      mode="inline"
      items={menuItemsFromSections(sections)}
      selectedKeys={selected ? [selected] : []}
      onClick={({ key }) => selectRoute(String(key))}
    />
  )

  return (
    <Layout data-testid="admin-layout" style={{ minHeight: '100vh' }}>
      {wide ? (
        <Sider width={264} theme="light">
          <BrandBlock title={branding?.title} fallback={t('app.title')} />
          {menu}
        </Sider>
      ) : null}
      <Layout>
        <Header
          style={{
            alignItems: 'center',
            background: '#fff',
            borderBottom: '1px solid #eaecef',
            display: 'flex',
            gap: 12,
            justifyContent: 'space-between',
            paddingInline: 16,
          }}
        >
          <Space>
            {!wide ? (
              <Button aria-label="Open navigation" icon={<MenuFoldOutlined />} onClick={() => setDrawerOpen(true)} />
            ) : null}
            {!wide ? <BrandInline title={branding?.title ?? t('app.title')} /> : null}
          </Space>
          <Space>
            <LocaleSwitcher />
            <Switch
              checked={themeMode === 'dark'}
              checkedChildren={t('theme.dark')}
              unCheckedChildren={t('theme.light')}
              onChange={toggleTheme}
            />
            <AccountMenu
              items={[{ label: t('account.profile'), to: '/admin/settings?tab=securityAuth' }]}
              logoutLabel={t('nav.logout')}
              onLogout={logout}
            >
              <Button type="text">{username || 'admin'}</Button>
            </AccountMenu>
          </Space>
        </Header>
        <Content style={{ margin: 0, minHeight: 0, padding: 24 }}>
          <Outlet />
        </Content>
      </Layout>
      <Drawer
        title={branding?.title ?? t('app.title')}
        placement="left"
        open={!wide && drawerOpen}
        onClose={() => setDrawerOpen(false)}
        styles={{ body: { padding: 0 } }}
      >
        {menu}
      </Drawer>
    </Layout>
  )
}

function BrandBlock({ title, fallback }: { title?: string; fallback: string }) {
  return (
    <div style={{ padding: 20 }}>
      <Typography.Title level={4} style={{ margin: 0 }}>
        {title || fallback}
      </Typography.Title>
    </div>
  )
}

function BrandInline({ title }: { title: string }) {
  return <Typography.Text strong>{title}</Typography.Text>
}
