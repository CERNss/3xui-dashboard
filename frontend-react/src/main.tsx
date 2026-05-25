import React from 'react'
import ReactDOM from 'react-dom/client'
import { App as AntdApp, Button, ConfigProvider, Space, Typography } from 'antd'
import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { createAppQueryClient } from './lib/queryClient'
import { AppRouter } from './router'
import { useThemeStore } from './stores/theme'
import { darkTheme, lightTheme } from './theme'
import './style.css'

const queryClient = createAppQueryClient()

function HelloScaffold() {
  return (
    <main className="hello-shell">
      <section className="hello-panel" aria-labelledby="react-tree-title">
        <p className="hello-kicker">React + AntD rewrite scaffold</p>
        <Typography.Title id="react-tree-title" className="hello-title" level={1}>
          3XUI Dashboard React tree is online
        </Typography.Title>
        <Typography.Paragraph className="hello-copy">
          This placeholder confirms the parallel frontend-react app, brand theme, router, and query
          providers are wired without touching the Vue tree.
        </Typography.Paragraph>
        <Space wrap>
          <Button type="primary">Brand primary button</Button>
          <Button>Secondary action</Button>
        </Space>
      </section>
    </main>
  )
}

function Root() {
  const resolvedTheme = useThemeStore((state) => state.resolvedTheme)

  return (
    <ConfigProvider theme={resolvedTheme === 'dark' ? darkTheme : lightTheme}>
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <AppRouter />
          </BrowserRouter>
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  )
}

export { HelloScaffold }

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
)
