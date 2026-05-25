import { ReloadOutlined } from '@ant-design/icons'
import { Button } from 'antd'

export interface RefreshButtonProps {
  loading?: boolean
  disabled?: boolean
  onClick?: () => void
  label?: string
}

export function RefreshButton({ loading, disabled, onClick, label = 'Refresh' }: RefreshButtonProps) {
  return (
    <Button aria-label={label} icon={<ReloadOutlined />} loading={loading} disabled={disabled} onClick={onClick}>
      {label}
    </Button>
  )
}
