import { BellOutlined, DownOutlined, MenuFoldOutlined } from '@ant-design/icons'
import { Button, Tooltip, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { AccountMenu, LocaleSwitcher } from '@/components/common'

interface AccountMenuItem {
  label: string
  to: string
}

interface AppTopbarProps {
  title: string
  subtitle?: string
  accountLabel: string
  accountRole: string
  accountItems: AccountMenuItem[]
  onLogout: () => void
  /** Mobile menu toggle. When omitted, the hamburger button is not
   * rendered (e.g. on wide layouts where the sidebar is always docked). */
  onOpenMobileNav?: () => void
  /** Notifications icon button. Hidden on portal until that surface
   * grows a real inbox. */
  showNotifications?: boolean
  /** Optional slot rendered between the page heading and the right-hand
   * tools — e.g. a per-page status pill. */
  centerSlot?: ReactNode
}

/**
 * Shared top bar chrome used by AdminLayout and PortalLayout. Left
 * side: optional hamburger + title/subtitle. Right side: optional
 * notifications, locale switcher, account menu.
 */
export function AppTopbar({
  title,
  subtitle,
  accountLabel,
  accountRole,
  accountItems,
  onLogout,
  onOpenMobileNav,
  showNotifications = false,
  centerSlot,
}: AppTopbarProps) {
  const { t } = useTranslation()
  return (
    <div className="admin-topbar-inner">
      <div className="admin-topbar-heading">
        {onOpenMobileNav ? (
          <Button
            aria-label={t('a11y.openNav')}
            className="admin-topbar-menu"
            icon={<MenuFoldOutlined />}
            onClick={onOpenMobileNav}
            type="text"
          />
        ) : null}
        <div className="admin-topbar-copy">
          <Typography.Title className="admin-topbar-title" level={1}>
            {title}
          </Typography.Title>
          {subtitle ? (
            <Typography.Text className="admin-topbar-subtitle">{subtitle}</Typography.Text>
          ) : null}
        </div>
      </div>
      {centerSlot}
      <div className="admin-topbar-tools">
        {showNotifications ? (
          <Tooltip title={t('admin.notifications')}>
            <button aria-label={t('admin.notifications')} className="admin-topbar-icon-button" type="button">
              <BellOutlined />
            </button>
          </Tooltip>
        ) : null}
        <LocaleSwitcher variant="chip" />
        <AccountMenu items={accountItems} logoutLabel={t('nav.logout')} onLogout={onLogout}>
          <button aria-label={t('account.openMenu')} className="admin-topbar-account" type="button">
            <span aria-hidden="true" className="admin-topbar-avatar">
              {initialsForAccount(accountLabel)}
            </span>
            <span className="admin-topbar-account-copy">
              <span className="admin-topbar-account-name">{displayAccountName(accountLabel)}</span>
              <span className="admin-topbar-account-role">{accountRole}</span>
            </span>
            <DownOutlined aria-hidden="true" className="admin-topbar-account-chevron" />
          </button>
        </AccountMenu>
      </div>
    </div>
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
