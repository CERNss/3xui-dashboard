import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider } from 'antd'
import { type PropsWithChildren, type ReactElement } from 'react'
import { render, type RenderOptions } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { createAppQueryClient } from '@/lib/queryClient'
import { lightTheme } from '@/theme'

interface RenderWithProvidersOptions extends Omit<RenderOptions, 'wrapper'> {
  initialEntries?: string[]
  initialPath?: string
  queryClient?: QueryClient
}

export function renderWithProviders(ui: ReactElement, options: RenderWithProvidersOptions = {}) {
  const {
    initialEntries,
    initialPath = '/',
    queryClient = createAppQueryClient({
      defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
    }),
    ...renderOptions
  } = options

  function Providers({ children }: PropsWithChildren) {
    return (
      <ConfigProvider theme={lightTheme}>
        <AntdApp>
          <QueryClientProvider client={queryClient}>
            <MemoryRouter
              initialEntries={initialEntries ?? [initialPath]}
              future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
            >
              {children}
            </MemoryRouter>
          </QueryClientProvider>
        </AntdApp>
      </ConfigProvider>
    )
  }

  return {
    queryClient,
    ...render(ui, { wrapper: Providers, ...renderOptions }),
  }
}
