import { describe, expect, it, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { Route, Routes, useLocation } from 'react-router-dom'
import { ProtectedRoute } from './ProtectedRoute'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{`${location.pathname}${location.search}`}</div>
}

function renderGuard(initialEntry: string, area: 'admin' | 'portal' = 'admin') {
  return renderWithProviders(
      <Routes>
        <Route element={<ProtectedRoute area={area} />}>
          <Route path="/admin/users" element={<div>Admin users</div>} />
          <Route path="/admin/status" element={<div>Admin status</div>} />
          <Route path="/admin" element={<div>Admin index</div>} />
          <Route path="/portal/subscription" element={<div>Portal subscription</div>} />
        </Route>
        <Route path="/login" element={<LocationProbe />} />
      </Routes>,
    { initialEntries: [initialEntry] },
  )
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAdminAuthStore.getState().clear()
    usePortalAuthStore.getState().clear()
  })

  it('redirects anonymous admin users with next= fullpath', () => {
    renderGuard('/admin/users?filter=active')

    expect(screen.getByTestId('location')).toHaveTextContent('/login?next=%2Fadmin%2Fusers%3Ffilter%3Dactive')
  })

  it('omits next= for default admin entry', () => {
    renderGuard('/admin')

    expect(screen.getByTestId('location')).toHaveTextContent('/login')
  })

  it('does not let a portal session satisfy the admin guard', () => {
    usePortalAuthStore.getState().setSession('portal-token', { id: 1, email: 'user@example.com' })

    renderGuard('/admin/status')

    expect(screen.getByTestId('location')).toHaveTextContent('/login?next=%2Fadmin%2Fstatus')
    expect(usePortalAuthStore.getState().token).toBe('portal-token')
  })
})
