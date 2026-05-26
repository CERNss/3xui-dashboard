import {
  AuditOutlined,
  ApiOutlined,
  BarChartOutlined,
  ClusterOutlined,
  CreditCardOutlined,
  DashboardOutlined,
  NodeIndexOutlined,
  SettingOutlined,
  ShopOutlined,
  ShoppingCartOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons'
import type { ItemType, MenuItemType } from 'antd/es/menu/interface'
import type { ReactNode } from 'react'
import type { TFunction } from 'i18next'

export interface NavLinkItem {
  key: string
  to: string
  label: string
  icon: ReactNode
}

export interface NavSection {
  key: string
  label: string
  items: NavLinkItem[]
}

export function adminSections(t: TFunction): NavSection[] {
  return [
    {
      key: 'overview',
      label: t('section.overview'),
      items: [
        { key: '/admin/ops-monitor', to: '/admin/ops-monitor', label: t('nav.opsMonitor'), icon: <BarChartOutlined /> },
        { key: '/admin/status', to: '/admin/status', label: t('nav.status'), icon: <DashboardOutlined /> },
      ],
    },
    {
      key: 'nodes',
      label: t('section.nodes'),
      items: [
        { key: '/admin/nodes', to: '/admin/nodes', label: t('nav.nodes'), icon: <NodeIndexOutlined /> },
        { key: '/admin/inbounds', to: '/admin/inbounds', label: t('nav.inbounds'), icon: <ClusterOutlined /> },
      ],
    },
    {
      key: 'users',
      label: t('section.users'),
      items: [
        { key: '/admin/users', to: '/admin/users', label: t('nav.users'), icon: <TeamOutlined /> },
        { key: '/admin/plans', to: '/admin/plans', label: t('nav.plansAdmin'), icon: <ShopOutlined /> },
        {
          key: '/admin/provisioning-pools',
          to: '/admin/provisioning-pools',
          label: t('nav.provisioningPools'),
          icon: <ClusterOutlined />,
        },
        { key: '/admin/orders', to: '/admin/orders', label: t('nav.ordersAdmin'), icon: <ShoppingCartOutlined /> },
      ],
    },
    {
      key: 'system',
      label: t('section.system'),
      items: [
        { key: '/admin/audit-log', to: '/admin/audit-log', label: t('nav.audit'), icon: <AuditOutlined /> },
        { key: '/admin/webhooks', to: '/admin/webhooks', label: t('nav.webhooks'), icon: <ApiOutlined /> },
        { key: '/admin/settings', to: '/admin/settings', label: t('nav.settings'), icon: <SettingOutlined /> },
      ],
    },
  ]
}

export function portalItems(t: TFunction): NavLinkItem[] {
  return [
    { key: '/portal/subscription', to: '/portal/subscription', label: t('nav.subscription'), icon: <CreditCardOutlined /> },
    { key: '/portal/usage', to: '/portal/usage', label: t('nav.usage'), icon: <BarChartOutlined /> },
    { key: '/portal/plans', to: '/portal/plans', label: t('nav.plans'), icon: <ShopOutlined /> },
    { key: '/portal/orders', to: '/portal/orders', label: t('nav.orders'), icon: <ShoppingCartOutlined /> },
    { key: '/portal/profile', to: '/portal/profile', label: t('nav.profile'), icon: <UserOutlined /> },
  ]
}

export function selectedKey(pathname: string, items: NavLinkItem[]) {
  const match = items.find((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))
  return match?.key
}

export function flattenSections(sections: NavSection[]) {
  return sections.flatMap((section) => section.items)
}

export function menuItemsFromSections(sections: NavSection[]): ItemType<MenuItemType>[] {
  return sections.map((section) => ({
    key: section.key,
    label: section.label,
    type: 'group',
    children: section.items.map((item) => ({
      key: item.key,
      icon: item.icon,
      label: item.label,
    })),
  }))
}

export function menuItemsFromLinks(items: NavLinkItem[]): ItemType<MenuItemType>[] {
  return items.map((item) => ({
    key: item.key,
    icon: item.icon,
    label: item.label,
  }))
}
