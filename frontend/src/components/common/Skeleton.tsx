import { Card, Skeleton as AntSkeleton, Space } from 'antd'

export type SkeletonVariant = 'kpi' | 'table' | 'card'

export interface SkeletonProps {
  variant?: SkeletonVariant
  rows?: number
}

export function Skeleton({ variant = 'card', rows = 3 }: SkeletonProps) {
  if (variant === 'kpi') {
    return (
      <Card>
        <AntSkeleton.Input active block size="small" style={{ width: '40%' }} />
        <AntSkeleton.Button active block size="large" style={{ marginTop: 12, width: '65%' }} />
      </Card>
    )
  }

  if (variant === 'table') {
    return (
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        {Array.from({ length: rows }).map((_, index) => (
          <AntSkeleton.Input key={index} active block size="large" />
        ))}
      </Space>
    )
  }

  return <AntSkeleton active paragraph={{ rows }} />
}
