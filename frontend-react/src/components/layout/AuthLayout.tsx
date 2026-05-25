import { Card, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
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

  return (
    <main
      data-testid="auth-layout"
      style={{
        alignItems: 'center',
        background: '#f7f8f9',
        display: 'flex',
        justifyContent: 'center',
        minHeight: '100vh',
        padding: 24,
      }}
    >
      <div style={{ position: 'absolute', right: 24, top: 24 }}>
        <LocaleSwitcher />
      </div>
      <Space direction="vertical" size={24} style={{ maxWidth: 480, width: '100%' }}>
        <Space direction="vertical" size={8} style={{ textAlign: 'center', width: '100%' }}>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {branding?.title ?? '3XUI Dashboard'}
          </Typography.Title>
          <Typography.Text type="secondary">{branding?.description ?? branding?.subtitle}</Typography.Text>
        </Space>
        <Card>
          {cardTitle || cardSubtitle ? (
            <Space direction="vertical" size={4} style={{ marginBottom: 24, textAlign: 'center', width: '100%' }}>
              {cardTitle ? <Typography.Title level={3}>{cardTitle}</Typography.Title> : null}
              {cardSubtitle ? <Typography.Text type="secondary">{cardSubtitle}</Typography.Text> : null}
            </Space>
          ) : null}
          {children ?? <Outlet />}
        </Card>
        {branding?.footer ? (
          <Typography.Text style={{ display: 'block', textAlign: 'center' }} type="secondary">
            {branding.footer}
          </Typography.Text>
        ) : null}
      </Space>
    </main>
  )
}
