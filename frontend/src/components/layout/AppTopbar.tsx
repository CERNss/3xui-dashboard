import { BellOutlined, DownOutlined, MenuFoldOutlined, MoonOutlined, RightOutlined, SearchOutlined, SunOutlined } from '@ant-design/icons'
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
  /** When set, the left side renders a breadcrumb ("group › page") in
   * place of the title/subtitle pair — the admin shell uses this to
   * mirror the design demo's topbar. */
  breadcrumbGroup?: string
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
  notificationsLabel?: string
  /** Locale switcher chip. Defaults on; the admin shell hides it because
   * the design mockup keeps the admin topbar to search + theme + bell. */
  showLocale?: boolean
  /** Account menu. Defaults on; the admin shell moves the account row
   * into the sidebar foot per the mockup, so it hides it here. */
  showAccount?: boolean
  /** When set, renders the design-mockup search box (decorative ⌘K). */
  searchPlaceholder?: string
  /** When provided, renders the dark/light theme-toggle pill (admin
   * topbar). Portal keeps the toggle in the sidebar foot instead. */
  themeMode?: 'light' | 'dark'
  onThemeToggle?: () => void
  /** Optional slot rendered in the right-hand tools after the locale
   * switcher — e.g. portal balance. */
  toolSlot?: ReactNode
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
  breadcrumbGroup,
  accountLabel,
  accountRole,
  accountItems,
  onLogout,
  onOpenMobileNav,
  showNotifications = false,
  notificationsLabel,
  showLocale = true,
  showAccount = true,
  searchPlaceholder,
  themeMode,
  onThemeToggle,
  toolSlot,
  centerSlot,
}: AppTopbarProps) {
  const { t } = useTranslation()
  const notificationText = notificationsLabel ?? t('admin.notifications')
  const showThemePill = Boolean(themeMode && onThemeToggle)
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
        {breadcrumbGroup ? (
          <nav aria-label={title} className="admin-topbar-crumb">
            <span className="admin-topbar-crumb-group">{breadcrumbGroup}</span>
            <RightOutlined aria-hidden="true" className="admin-topbar-crumb-sep" />
            <Typography.Title className="admin-topbar-crumb-current" level={1}>
              {title}
            </Typography.Title>
          </nav>
        ) : (
          <div className="admin-topbar-copy">
            <Typography.Title className="admin-topbar-title" level={1}>
              {title}
            </Typography.Title>
            {subtitle ? (
              <Typography.Text className="admin-topbar-subtitle">{subtitle}</Typography.Text>
            ) : null}
          </div>
        )}
      </div>
      {centerSlot}
      <div className="admin-topbar-tools">
        {searchPlaceholder ? (
          <button aria-label={searchPlaceholder} className="admin-topbar-search" type="button">
            <SearchOutlined aria-hidden="true" />
            <span className="admin-topbar-search-text">{searchPlaceholder}</span>
            <kbd className="admin-topbar-search-kbd">⌘K</kbd>
          </button>
        ) : null}
        {showLocale ? <LocaleSwitcher variant="chip" /> : null}
        {showThemePill ? (
          <>
            <span aria-hidden="true" className="admin-topbar-divider" />
            <button
              aria-label={themeMode === 'dark' ? t('theme.toggleLight') : t('theme.toggleDark')}
              className="admin-topbar-theme"
              onClick={onThemeToggle}
              type="button"
            >
              <span className="admin-topbar-theme-opt" data-active={themeMode === 'dark' ? 'true' : 'false'}>
                <MoonOutlined />
              </span>
              <span className="admin-topbar-theme-opt" data-active={themeMode === 'light' ? 'true' : 'false'}>
                <SunOutlined />
              </span>
            </button>
          </>
        ) : null}
        {showNotifications ? (
          <Tooltip title={notificationText}>
            <button aria-label={notificationText} className="admin-topbar-icon-button" type="button">
              <BellOutlined />
            </button>
          </Tooltip>
        ) : null}
        {toolSlot}
        {showAccount ? (
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
        ) : null}
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
