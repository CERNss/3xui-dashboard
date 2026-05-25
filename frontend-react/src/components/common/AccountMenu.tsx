import { ExportOutlined, LogoutOutlined } from '@ant-design/icons'
import { Button, Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export interface AccountMenuItem {
  key?: string
  label: ReactNode
  to?: string
  href?: string
  icon?: ReactNode
  external?: boolean
}

export interface AccountMenuProps {
  items?: AccountMenuItem[]
  children?: ReactNode
  logoutLabel?: ReactNode
  onLogout?: () => void
}

export function AccountMenu({ items = [], children, logoutLabel = 'Logout', onLogout }: AccountMenuProps) {
  const menuItems: MenuProps['items'] = [
    ...items.map((item, index) => {
      const key = item.key ?? item.to ?? item.href ?? String(index)
      const label = item.to ? (
        <Link to={item.to}>{item.label}</Link>
      ) : item.href ? (
        <a href={item.href} target={item.external ? '_blank' : undefined} rel={item.external ? 'noreferrer' : undefined}>
          {item.label}
          {item.external ? <ExportOutlined style={{ marginInlineStart: 6 }} /> : null}
        </a>
      ) : (
        item.label
      )

      return {
        key,
        icon: item.icon,
        label
      }
    }),
    {
      type: 'divider'
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: logoutLabel,
      onClick: onLogout
    }
  ]

  return (
    <Dropdown menu={{ items: menuItems }} trigger={['click']}>
      {children ?? <Button type="text">Account</Button>}
    </Dropdown>
  )
}
