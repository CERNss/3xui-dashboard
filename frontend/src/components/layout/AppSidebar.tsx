import { GlobalOutlined, MenuFoldOutlined, MenuUnfoldOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Tooltip, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { NavSection } from './nav'

interface AppSidebarProps {
  collapsed: boolean
  onCollapseToggle: () => void
  onNavigate: (key: string) => void
  onThemeToggle: () => void
  selectedKey?: string
  /** Navigation laid out as zero-or-more sections. Sections with an
   * empty label render their items without a group heading so a flat
   * nav (e.g. the portal) can reuse this same shell. */
  sections: NavSection[]
  themeMode: 'light' | 'dark'
  title: string
  /** Small monospace line under the brand title (e.g. "node
   * orchestration" on the admin shell, "user portal" on the portal). */
  subtitle?: string
  /** Optional slot rendered below the brand row — the admin shell puts
   * its cluster-status pill ("N nodes online") here. Hidden while the
   * sidebar is collapsed. */
  clusterSlot?: ReactNode
  /** Optional aria-label for the surrounding <nav>. */
  navLabel?: string
  /** Optional account row rendered at the top of the footer. The admin
   * shell passes its account dropdown here to match the design mockup
   * (account lives in the sidebar foot, not the topbar). */
  accountSlot?: ReactNode
  /** Whether to render the in-foot theme toggle. Defaults on (portal);
   * the admin shell turns it off because its theme toggle lives in the
   * topbar per the mockup. */
  showThemeToggle?: boolean
}

/**
 * AppSidebar is the shared sidebar chrome used by both AdminLayout and
 * PortalLayout. Keeps the brand row, sectioned nav, theme toggle, and
 * collapse toggle behaviour consistent between the two surfaces.
 */
export function AppSidebar({
  collapsed,
  onCollapseToggle,
  onNavigate,
  onThemeToggle,
  selectedKey,
  sections,
  themeMode,
  title,
  subtitle,
  clusterSlot,
  navLabel,
  accountSlot,
  showThemeToggle = true,
}: AppSidebarProps) {
  const { t } = useTranslation()
  const resolvedNavLabel = navLabel ?? t('nav.admin')

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
            {subtitle ? <span className="admin-sidebar-subtitle">{subtitle}</span> : null}
          </div>
        ) : null}
      </div>
      {!collapsed && clusterSlot ? <div className="admin-sidebar-cluster">{clusterSlot}</div> : null}

      <nav aria-label={resolvedNavLabel} className="admin-sidebar-nav">
        {sections.map((section) => (
          <div className="admin-sidebar-section" key={section.key}>
            {!collapsed && section.label ? (
              <div className="admin-sidebar-section-label">{section.label}</div>
            ) : null}
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
        {accountSlot ? <div className="admin-sidebar-account-slot">{accountSlot}</div> : null}
        {showThemeToggle ? (
          <button className="admin-sidebar-action" onClick={onThemeToggle} type="button">
            <span aria-hidden="true" className="admin-sidebar-item-icon">
              {themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
            </span>
            {!collapsed ? <span>{themeMode === 'dark' ? t('theme.light') : t('theme.dark')}</span> : null}
          </button>
        ) : null}
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
