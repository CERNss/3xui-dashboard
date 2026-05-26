import { ReloadOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import { useTranslation } from 'react-i18next'

export interface RefreshButtonProps {
  loading?: boolean
  disabled?: boolean
  onClick?: () => void
  label?: string
}

export function RefreshButton({ loading, disabled, onClick, label }: RefreshButtonProps) {
  const { t } = useTranslation()
  const buttonLabel = label ?? t('common.refresh')

  return (
    <Button aria-label={buttonLabel} icon={<ReloadOutlined />} loading={loading} disabled={disabled} onClick={onClick}>
      {buttonLabel}
    </Button>
  )
}
