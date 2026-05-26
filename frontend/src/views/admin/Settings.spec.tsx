import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SettingItem } from '@/api/admin/settings'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Settings from './Settings'

const setMutateAsync = vi.fn()
const clearMutateAsync = vi.fn()
const uploadMutateAsync = vi.fn()
const smtpMutateAsync = vi.fn()
const refetch = vi.fn()

let settings: SettingItem[] = []

vi.mock('@/hooks/queries/admin/settings', () => ({
  useSettingsList: () => ({
    data: settings,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch,
  }),
  useSetSetting: () => ({ error: null, isPending: false, mutateAsync: setMutateAsync }),
  useClearSetting: () => ({ error: null, isPending: false, mutateAsync: clearMutateAsync }),
  useUploadBrandIcon: () => ({ error: null, isPending: false, mutateAsync: uploadMutateAsync }),
  useSmtpTest: () => ({ error: null, isPending: false, mutateAsync: smtpMutateAsync }),
}))

vi.mock('./Webhooks', () => ({
  default: ({ embedded }: { embedded?: boolean }) => (
    <div data-testid="embedded-webhooks">{embedded ? 'embedded webhooks' : 'standalone webhooks'}</div>
  ),
}))

function item(partial: Partial<SettingItem> & Pick<SettingItem, 'key' | 'label' | 'group' | 'type' | 'value'>): SettingItem {
  return {
    default: '',
    description: '',
    env_fallback: '',
    has_override: false,
    ...partial,
  }
}

function renderSettings(initialPath = '/admin/settings') {
  return renderWithProviders(
    <Routes>
      <Route path="/admin/settings" element={<Settings />} />
    </Routes>,
    { initialPath },
  )
}

beforeEach(() => {
  settings = [
    item({ key: 'site_name', label: 'Site name', group: 'other', type: 'string', value: 'Acme' }),
    item({ key: 'subscription_remark_model', label: 'Subscription remark model', group: 'subscription', type: 'string', value: '-ieo', has_override: true }),
    item({ key: 'traffic_warn_pct', label: 'Traffic warning percent', group: 'traffic', type: 'int', value: '80' }),
    item({ key: 'ops_collect_enabled', label: 'Node health collection', group: 'data_collection', type: 'bool', value: 'true', has_override: true }),
    item({ key: 'ops_collect_interval_seconds', label: 'Health collection interval', group: 'data_collection', type: 'int', value: '60' }),
    item({ key: 'ops_collect_timeout_seconds', label: 'Health request timeout', group: 'data_collection', type: 'int', value: '12' }),
    item({ key: 'traffic_collect_concurrency', label: 'Traffic collection concurrency', group: 'data_collection', type: 'int', value: '8' }),
    item({ key: 'traffic_collect_retry_attempts', label: 'Traffic retry attempts', group: 'data_collection', type: 'int', value: '0' }),
    item({ key: 'public_registration_enabled', label: 'Public registration enabled', group: 'registration', type: 'bool', value: 'true' }),
    item({ key: 'oidc_issuer', label: 'OIDC issuer', group: 'other', type: 'string', value: 'https://auth.example.test', has_override: true }),
    item({ key: 'new_user_initial_balance_cents', label: 'New-user initial balance', group: 'registration', type: 'int', value: '100' }),
    item({ key: 'smtp_host', label: 'SMTP host', group: 'other', type: 'string', value: 'smtp.example.test' }),
    item({ key: 'brand_title', label: 'Brand title', group: 'other', type: 'string', value: 'Hidden brand row' }),
    item({ key: 'brand_subtitle', label: 'Brand subtitle', group: 'other', type: 'string', value: 'Configuration platform' }),
    item({ key: 'brand_docs_url', label: 'Documentation link', group: 'other', type: 'string', value: 'https://docs.example.test' }),
    item({ key: 'brand_homepage_content', label: 'Homepage content', group: 'other', type: 'string', value: 'Welcome copy' }),
    item({ key: 'brand_description', label: 'Brand description', group: 'other', type: 'string', value: 'Fleet control' }),
    item({ key: 'brand_footer', label: 'Brand footer', group: 'other', type: 'string', value: 'Footer text' }),
    item({ key: 'brand_icon_url', label: 'Brand icon URL', group: 'other', type: 'string', value: '' }),
  ]
  setMutateAsync.mockResolvedValue({})
  clearMutateAsync.mockResolvedValue({})
  uploadMutateAsync.mockResolvedValue({ url: '/brand/icon.png' })
  smtpMutateAsync.mockResolvedValue({})
  setMutateAsync.mockClear()
  clearMutateAsync.mockClear()
  uploadMutateAsync.mockClear()
  smtpMutateAsync.mockClear()
  refetch.mockReset()
})

describe('Settings', () => {
  it('renders exactly eight tabs and preserves tab state through query params', async () => {
    const user = userEvent.setup()
    renderSettings('/admin/settings?tab=messages')

    const tabs = screen.getAllByRole('tab')
    expect(tabs).toHaveLength(8)
    expect(tabs.map((tab) => tab.textContent)).toEqual([
      'General',
      'Subscription',
      'Alerts',
      'Data collection',
      'Security & auth',
      'User defaults',
      'Messages',
      'Notifications',
    ])
    expect(screen.getByRole('tab', { name: 'Messages' })).toHaveAttribute('aria-selected', 'true')

    await user.click(screen.getByRole('tab', { name: 'Notifications' }))
    expect(screen.getByRole('tab', { name: 'Notifications' })).toHaveAttribute('aria-selected', 'true')
  })

  it('buffers drafts per setting key, saves changed rows, and resets overrides', async () => {
    const user = userEvent.setup()
    renderSettings()

    const input = screen.getByLabelText('SMTP host')
    await user.clear(input)
    await user.type(input, 'smtp2.example.test')
    const siteCard = screen.getByText('SMTP host').closest('.ant-card')!
    await user.click(within(siteCard as HTMLElement).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(setMutateAsync).toHaveBeenCalledWith({ key: 'smtp_host', value: 'smtp2.example.test' }))

    await user.click(screen.getByRole('tab', { name: 'Subscription' }))
    const subscriptionCard = screen.getByText('Subscription remark model').closest('.ant-card')!
    await user.click(within(subscriptionCard as HTMLElement).getByRole('button', { name: 'Reset' }))
    await waitFor(() => expect(clearMutateAsync).toHaveBeenCalledWith('subscription_remark_model'))
  })

  it('keeps plain site settings on the general tab without brand or OIDC rows', async () => {
    const { container } = renderSettings()

    expect(screen.getByRole('heading', { name: 'Site settings' })).toBeInTheDocument()
    expect(container.querySelector('[data-setting-key="site_name"]')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Site name')).toHaveValue('Hidden brand row')
    expect(screen.getByLabelText('Documentation link')).toHaveValue('https://docs.example.test')
    expect(screen.getByLabelText('Homepage content')).toHaveValue('Welcome copy')
    expect(container.querySelector('[data-setting-key="brand_title"]')).toBeInTheDocument()
    expect(container.querySelector('[data-setting-key="oidc_issuer"]')).not.toBeInTheDocument()
  })

  it('keeps registration and OIDC settings on the security auth tab without brand rows', async () => {
    const { container } = renderSettings('/admin/settings?tab=securityAuth')

    expect(screen.getByRole('heading', { name: 'Security & auth' })).toBeInTheDocument()
    expect(container.querySelector('[data-setting-key="public_registration_enabled"]')).toBeInTheDocument()
    expect(container.querySelector('[data-setting-key="oidc_issuer"]')).toBeInTheDocument()
    expect(screen.getByLabelText('Public registration enabled')).toHaveValue('true')
    expect(screen.getByLabelText('OIDC issuer')).toHaveValue('https://auth.example.test')
    expect(container.querySelector('[data-setting-key="brand_title"]')).not.toBeInTheDocument()
  })

  it('keeps new-user defaults on the user defaults tab', async () => {
    const { container } = renderSettings('/admin/settings?tab=userDefaults')

    expect(screen.getByRole('heading', { name: 'User defaults' })).toBeInTheDocument()
    expect(container.querySelector('[data-setting-key="new_user_initial_balance_cents"]')).toBeInTheDocument()
    expect(screen.getByLabelText('New-user initial balance')).toHaveValue(100)
    expect(container.querySelector('[data-setting-key="public_registration_enabled"]')).not.toBeInTheDocument()
  })

  it('ports data collection min/max behavior and renders backend-added dataCollection keys', async () => {
    settings.push(item({ key: 'ops_collect_max_jitter_seconds', label: 'Max jitter seconds', group: 'data_collection', type: 'int', value: '3' }))
    const user = userEvent.setup()
    renderSettings('/admin/settings?tab=dataCollection')

    expect(screen.getByRole('heading', { name: 'Data collection' })).toBeInTheDocument()
    expect(screen.getByLabelText('Node health collection')).toBeInTheDocument()
    expect(screen.getByLabelText('Max jitter seconds')).toBeInTheDocument()
    expect(screen.getByLabelText('Health collection interval')).toHaveAttribute('min', '5')
    expect(screen.getByLabelText('Health request timeout')).toHaveAttribute('min', '1')
    expect(screen.getByLabelText('Health request timeout')).toHaveAttribute('max', '60')
    expect(screen.getByLabelText('Traffic collection concurrency')).toHaveAttribute('max', '64')
    expect(screen.getByLabelText('Traffic retry attempts')).toHaveAttribute('max', '5')

    await user.selectOptions(screen.getByLabelText('Node health collection'), 'false')
    const row = screen.getByText('Node health collection').closest('.ant-card')!
    const saveButton = within(row as HTMLElement).getByRole('button', { name: 'Save' })
    await waitFor(() => expect(saveButton).toBeEnabled())
    await user.click(saveButton)
    await waitFor(() => expect(setMutateAsync).toHaveBeenCalledWith({ key: 'ops_collect_enabled', value: 'false' }))
  })

  it('sends SMTP tests from the messages tab', async () => {
    const user = userEvent.setup()
    renderSettings('/admin/settings?tab=messages')

    await user.type(screen.getByLabelText('SMTP test recipient'), 'ops@example.test')
    await user.click(screen.getByRole('button', { name: /Send test/ }))

    await waitFor(() => expect(smtpMutateAsync).toHaveBeenCalledWith('ops@example.test'))
  })

  it('renders notifications as a thin embedded Webhooks wrapper', async () => {
    renderSettings('/admin/settings?tab=notifications')

    expect(screen.getByTestId('embedded-webhooks')).toHaveTextContent('embedded webhooks')
  })

  it('refreshes settings from the page header action', async () => {
    const user = userEvent.setup()
    renderSettings()

    await user.click(screen.getByRole('button', { name: 'Refresh' }))

    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('shows an empty state when the selected tab has no settings', async () => {
    settings = [item({ key: 'site_name', label: 'Site name', group: 'other', type: 'string', value: 'Acme' })]
    renderSettings('/admin/settings?tab=userDefaults')

    expect(screen.getByRole('heading', { name: 'User defaults' })).toBeInTheDocument()
    expect(screen.getByText('No settings in this section')).toBeInTheDocument()
  })

  it('uploads favicon through the brand icon composite using an image-only file input', async () => {
    const user = userEvent.setup()
    renderSettings()

    const input = screen.getByLabelText('Brand icon file') as HTMLInputElement
    expect(input.accept).toBe('image/png,image/jpeg,image/webp,image/svg+xml')
    const file = new File(['fake'], 'favicon.png', { type: 'image/png' })
    await user.upload(input, file)
    await user.click(screen.getByRole('button', { name: 'Upload favicon' }))

    await waitFor(() => expect(uploadMutateAsync).toHaveBeenCalledWith(file))
  })
})
