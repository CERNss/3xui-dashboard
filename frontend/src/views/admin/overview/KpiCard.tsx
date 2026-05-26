import { Card, Space, Statistic, Typography } from 'antd'
import type { ReactNode } from 'react'

interface KpiCardProps {
  title: ReactNode
  value: ReactNode
  suffix?: ReactNode
  extra?: ReactNode
}

export function KpiCard({ title, value, suffix, extra }: KpiCardProps) {
  return (
    <Card size="small" style={{ height: '100%' }}>
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Statistic title={title} value={String(value)} suffix={suffix} />
        {extra ? <Typography.Text type="secondary">{extra}</Typography.Text> : null}
      </Space>
    </Card>
  )
}
