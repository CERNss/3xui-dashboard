import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'

export type ProtectedArea = 'admin' | 'portal'

export interface ProtectedRouteProps {
  area: ProtectedArea
  children?: React.ReactNode
}

const defaultEntryPaths: Record<ProtectedArea, Set<string>> = {
  admin: new Set(['/admin', '/admin/']),
  portal: new Set(['/portal', '/portal/']),
}

function loginSearch(pathname: string, fullPath: string, area: ProtectedArea): string {
  if (defaultEntryPaths[area].has(pathname)) {
    return ''
  }

  return `?next=${encodeURIComponent(fullPath)}`
}

export function ProtectedRoute({ area, children }: ProtectedRouteProps) {
  const location = useLocation()
  const adminAuthenticated = useAdminAuthStore((state) => state.isAuthenticated || Boolean(state.token))
  const portalAuthenticated = usePortalAuthStore((state) => state.isAuthenticated || Boolean(state.token))
  const authenticated = area === 'admin' ? adminAuthenticated : portalAuthenticated

  if (!authenticated) {
    const fullPath = `${location.pathname}${location.search}`
    return <Navigate replace to={{ pathname: '/login', search: loginSearch(location.pathname, fullPath, area) }} />
  }

  return children ? <>{children}</> : <Outlet />
}
