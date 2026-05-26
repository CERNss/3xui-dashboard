import { Button, Empty, Space, Typography } from 'antd'
import type { ReactNode } from 'react'

export interface EmptyStateProps {
  icon?: ReactNode
  title?: ReactNode
  description?: ReactNode
  actionLabel?: ReactNode
  onAction?: () => void
}

export function EmptyState({ icon, title, description, actionLabel, onAction }: EmptyStateProps) {
  const hasAction = Boolean(actionLabel && onAction)

  return (
    <Empty image={icon ?? Empty.PRESENTED_IMAGE_SIMPLE} description={false}>
      <Space direction="vertical" size={8} align="center">
        {title ? <Typography.Title level={5}>{title}</Typography.Title> : null}
        {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
        {hasAction ? (
          <Button type="primary" onClick={onAction}>
            {actionLabel}
          </Button>
        ) : null}
      </Space>
    </Empty>
  )
}
