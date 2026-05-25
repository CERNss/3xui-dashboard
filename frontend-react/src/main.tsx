import React from 'react'
import ReactDOM from 'react-dom/client'
import { Button, ConfigProvider, Space, Typography } from 'antd'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { lightTheme } from './theme'
import './style.css'

const queryClient = new QueryClient()

function App() {
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

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <ConfigProvider theme={lightTheme}>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ConfigProvider>
  </React.StrictMode>
)
