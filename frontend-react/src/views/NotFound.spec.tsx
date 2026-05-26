import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import NotFound from './NotFound'

const adminState = vi.hoisted(() => ({ isAuthenticated: false }))
const portalState = vi.hoisted(() => ({ isAuthenticated: false }))

vi.mock('@/stores/adminAuth', () => ({
  useAdminAuthStore: (selector: (state: typeof adminState) => unknown) => selector(adminState),
}))

vi.mock('@/stores/portalAuth', () => ({
  usePortalAuthStore: (selector: (state: typeof portalState) => unknown) => selector(portalState),
}))

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname}</span>
}

function renderNotFound(initialEntry = '/missing') {
  return renderWithProviders(
      <Routes>
        <Route path="/missing" element={<NotFound />} />
        <Route path="/admin/missing" element={<NotFound />} />
        <Route path="/portal/missing" element={<NotFound />} />
        <Route path="*" element={<LocationProbe />} />
      </Routes>,
    { initialEntries: [initialEntry] },
  )
}

beforeEach(() => {
  adminState.isAuthenticated = false
  portalState.isAuthenticated = false
})

describe('NotFound', () => {
  it('renders an AntD 404 result with a back home CTA', () => {
    renderNotFound()

    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to home' })).toBeInTheDocument()
  })

  it('routes admin misses back to admin home', async () => {
    renderNotFound('/admin/missing')

    await userEvent.click(screen.getByRole('button', { name: 'Back to home' }))

    expect(await screen.findByTestId('location')).toHaveTextContent('/admin')
  })

  it('routes portal misses back to portal home', async () => {
    renderNotFound('/portal/missing')

    await userEvent.click(screen.getByRole('button', { name: 'Back to home' }))

    expect(await screen.findByTestId('location')).toHaveTextContent('/portal')
  })
})
