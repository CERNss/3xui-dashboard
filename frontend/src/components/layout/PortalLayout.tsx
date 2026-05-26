import { Layout, Menu, Space, Typography, theme } from 'antd'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { AccountMenu, LocaleSwitcher } from '@/components/common'
import { useMinWidth } from '@/hooks/useBreakpoint'
import { useBranding } from '@/hooks/queries/branding'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { LG_BREAKPOINT } from '@/theme'
import { menuItemsFromLinks, portalItems, selectedKey } from './nav'

const { Header, Sider, Content } = Layout

export function PortalLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const wide = useMinWidth(LG_BREAKPOINT)
  const clearAuth = usePortalAuthStore((state) => state.clear)
  const user = usePortalAuthStore((state) => state.user)
  const { data: branding } = useBranding()
  const { token } = theme.useToken()
  const items = useMemo(() => portalItems(t), [t])
  const selected = selectedKey(location.pathname, items)

  function selectRoute(key: string) {
    navigate(key)
  }

  function logout() {
    clearAuth()
    navigate('/login', { replace: true })
  }

  const wideMenu = (
    <Menu
      mode="inline"
      items={menuItemsFromLinks(items)}
      selectedKeys={selected ? [selected] : []}
      onClick={({ key }) => selectRoute(String(key))}
    />
  )
  const bottomNavMenu = (
    <Menu
      mode="horizontal"
      items={items.map((item) => ({
        key: item.key,
        icon: item.icon,
        label: (
          <Link aria-current={selected === item.key ? 'page' : undefined} to={item.to}>
            {item.label}
          </Link>
        ),
      }))}
      selectedKeys={selected ? [selected] : []}
      style={{
        borderBottom: 0,
        display: 'grid',
        gridTemplateColumns: `repeat(${items.length}, minmax(0, 1fr))`,
        lineHeight: 1.2,
        paddingBottom: 'env(safe-area-inset-bottom)',
      }}
    />
  )

  return (
    <Layout data-testid="portal-layout" style={{ minHeight: '100vh' }}>
      {wide ? (
        <Sider width={240} theme="light">
          <div style={{ padding: 20 }}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {branding?.title ?? t('app.title')}
            </Typography.Title>
          </div>
          {wideMenu}
        </Sider>
      ) : null}
      <Layout>
        <Header
          style={{
            alignItems: 'center',
            background: token.colorBgContainer,
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            display: 'flex',
            justifyContent: 'space-between',
            paddingInline: 16,
          }}
        >
          <Typography.Text strong>{user?.email ?? branding?.title ?? t('app.title')}</Typography.Text>
          <Space>
            <LocaleSwitcher />
            <AccountMenu
              items={[{ label: t('account.profile'), to: '/portal/profile' }]}
              logoutLabel={t('nav.logout')}
              onLogout={logout}
            />
          </Space>
        </Header>
        <Content style={{ background: token.colorBgLayout, margin: 0, minHeight: 0, padding: wide ? 24 : '24px 16px 88px' }}>
          <Outlet />
        </Content>
      </Layout>
      {!wide ? (
        <nav
          aria-label={t('portal.shell.navigation')}
          data-testid="portal-bottom-nav"
          style={{
            background: token.colorBgContainer,
            borderTop: `1px solid ${token.colorBorderSecondary}`,
            bottom: 0,
            left: 0,
            position: 'fixed',
            right: 0,
            zIndex: 20,
          }}
        >
          {bottomNavMenu}
        </nav>
      ) : null}
    </Layout>
  )
}
