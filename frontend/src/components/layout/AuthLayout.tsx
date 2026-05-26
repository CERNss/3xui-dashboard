import { GlobalOutlined } from '@ant-design/icons'
import { Card, Space, Typography, theme } from 'antd'
import type { CSSProperties, ReactNode } from 'react'
import { Outlet } from 'react-router-dom'
import { LocaleSwitcher } from '@/components/common'
import { useBranding } from '@/hooks/queries/branding'

export interface AuthLayoutProps {
  children?: ReactNode
  cardTitle?: ReactNode
  cardSubtitle?: ReactNode
}

export function AuthLayout({ cardSubtitle, cardTitle, children }: AuthLayoutProps) {
  const { data: branding } = useBranding()
  const { token } = theme.useToken()
  const title = branding?.title ?? '3XUI Dashboard'
  const subtitle = branding?.description ?? branding?.subtitle
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
