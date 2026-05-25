import { Space, Typography } from 'antd'
import type { ReactNode } from 'react'

export interface PageHeaderProps {
  title: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
}

export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <div
      style={{
        alignItems: 'flex-start',
        display: 'flex',
        gap: 16,
        justifyContent: 'space-between',
        marginBottom: 24
      }}
    >
      <Space direction="vertical" size={4}>
        <Typography.Title level={2} style={{ margin: 0 }}>
          {title}
        </Typography.Title>
        {subtitle ? <Typography.Text type="secondary">{subtitle}</Typography.Text> : null}
      </Space>
      {actions ? <Space wrap>{actions}</Space> : null}
    </div>
  )
}
