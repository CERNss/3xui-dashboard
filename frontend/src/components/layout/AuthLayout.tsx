import { GlobalOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Card, Space, Typography, theme } from 'antd'
import type { CSSProperties, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet } from 'react-router-dom'
import { LocaleSwitcher } from '@/components/common'
import { useBranding } from '@/hooks/queries/branding'
import { useThemeStore } from '@/stores/theme'

export interface AuthLayoutProps {
  children?: ReactNode
  cardTitle?: ReactNode
  cardSubtitle?: ReactNode
}

export function AuthLayout({ cardSubtitle, cardTitle, children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { data: branding } = useBranding()
  const { token } = theme.useToken()
  const themeMode = useThemeStore((state) => state.resolvedTheme)
  const toggleTheme = useThemeStore((state) => state.toggle)
  const title = branding?.title ?? t('app.title')
  const subtitle = branding?.description ?? branding?.subtitle
  const nextThemeLabel = themeMode === 'dark' ? t('theme.light') : t('theme.dark')
  const toggleThemeLabel = themeMode === 'dark' ? t('theme.toggleLight') : t('theme.toggleDark')
  const style = {
    '--auth-layout-bg': token.colorBgLayout,
    '--auth-container-bg': token.colorBgContainer,
    '--auth-border-color': token.colorBorder,
    '--auth-brand-color': token.colorPrimary,
    '--auth-text-color': token.colorText,
    '--auth-text-secondary': token.colorTextSecondary,
  } as CSSProperties

  return (
    <main className="auth-layout" data-testid="auth-layout" style={style}>
      <div className="auth-locale">
        <LocaleSwitcher />
      </div>
      <div className="auth-theme">
        <button aria-label={toggleThemeLabel} className="auth-theme-button" onClick={toggleTheme} type="button">
          <span aria-hidden="true" className="auth-theme-icon">
            {themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
          </span>
          <span>{nextThemeLabel}</span>
        </button>
      </div>
      <div className="auth-layout-inner">
        <Space className="auth-brand" direction="vertical" size={10}>
          {branding?.icon_url ? (
            <img alt="" className="auth-brand-icon" src={branding.icon_url} />
          ) : (
            <span aria-hidden="true" className="auth-brand-mark">
              <GlobalOutlined />
            </span>
          )}
          <Typography.Title className="auth-brand-title" level={2}>
            {title}
          </Typography.Title>
          {subtitle ? <Typography.Text className="auth-brand-subtitle">{subtitle}</Typography.Text> : null}
        </Space>
        {cardTitle || cardSubtitle ? (
          <Card className="auth-login-card">
            <Space className="auth-login-heading" direction="vertical" size={4}>
              {cardTitle ? <Typography.Title level={3}>{cardTitle}</Typography.Title> : null}
              {cardSubtitle ? <Typography.Text type="secondary">{cardSubtitle}</Typography.Text> : null}
            </Space>
            {children ?? <Outlet />}
          </Card>
        ) : (
          children ?? <Outlet />
        )}
        {branding?.footer ? (
          <Typography.Text className="auth-footer" type="secondary">
            {branding.footer}
          </Typography.Text>
        ) : null}
      </div>
    </main>
  )
}
