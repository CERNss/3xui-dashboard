import { Button, Result } from 'antd'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'

export function NotFound() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const adminAuthenticated = useAdminAuthStore((state) => state.isAuthenticated)
  const portalAuthenticated = usePortalAuthStore((state) => state.isAuthenticated)

  const home = useMemo(() => {
    if (location.pathname.startsWith('/portal')) return '/portal'
    if (location.pathname.startsWith('/admin')) return '/admin'
    if (adminAuthenticated) return '/admin'
    if (portalAuthenticated) return '/portal'
    return '/admin'
  }, [adminAuthenticated, location.pathname, portalAuthenticated])

  return (
    <Result
      status="404"
      title="404"
      subTitle={t('errors.notFound', { defaultValue: 'Page not found' })}
      extra={
        <Button type="primary" onClick={() => navigate(home)}>
          {t('errors.backHome', { defaultValue: 'Back to home' })}
        </Button>
      }
    />
  )
}

export default NotFound
