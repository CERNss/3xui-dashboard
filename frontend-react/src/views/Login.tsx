import { LockOutlined, LoginOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Divider, Form, Input, Space, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router-dom'
import { portalAuthApi, type OIDCProvider } from '@/api/portal/auth'
import { useAdminLogin } from '@/hooks/queries/admin/auth'
import { useOidcStart } from '@/hooks/queries/portal/auth'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { formatError } from '@/utils/format'

interface LoginFormValues {
  username: string
  password: string
}

function localPath(value: string | null, fallback: string) {
  if (!value || !value.startsWith('/') || value.startsWith('//')) return fallback
  try {
    const url = new URL(value, window.location.origin)
    if (url.origin !== window.location.origin) return fallback
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return fallback
  }
}

function providerLabel(provider: OIDCProvider) {
  return provider.name || 'OIDC'
}

export function Login() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const setAdminSession = useAdminAuthStore((state) => state.setSession)
  const adminLogin = useAdminLogin()
  const oidcStart = useOidcStart()
  const [providers, setProviders] = useState<OIDCProvider[]>([])
  const [error, setError] = useState<string | null>(null)
  const [cooldown, setCooldown] = useState(0)

  const params = useMemo(() => new URLSearchParams(location.search), [location.search])
  const nextPath = localPath(params.get('next'), '/admin/status')

  useEffect(() => {
    let cancelled = false
    portalAuthApi
      .oidcProviders()
      .then((list) => {
        if (!cancelled) setProviders(list)
      })
      .catch(() => {
        if (!cancelled) setProviders([])
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (cooldown <= 0) return undefined
    const timer = window.setTimeout(() => setCooldown((value) => Math.max(0, value - 1)), 1000)
    return () => window.clearTimeout(timer)
  }, [cooldown])

  async function submit(values: LoginFormValues) {
    if (cooldown > 0) return
    setError(null)
    try {
      const res = await adminLogin.mutateAsync(values)
      setAdminSession(res.token, res.username)
      navigate(nextPath, { replace: true })
    } catch (e) {
      setError(formatError(e, t('auth.loginFailed', { defaultValue: 'Login failed' })))
      setCooldown(3)
    }
  }

  async function startOidc(provider: OIDCProvider) {
    setError(null)
    try {
      if (provider.login_url) {
        window.location.assign(provider.login_url)
        return
      }
      const res = await oidcStart.mutateAsync(nextPath)
      window.location.assign(res.authorize_url)
    } catch (e) {
      setError(formatError(e, t('auth.oidcStarting', { defaultValue: 'Could not start OIDC login' })))
    }
  }

  return (
    <main className="auth-surface">
      <Card style={{ width: 'min(100%, 420px)' }}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <div>
            <Typography.Title level={2} style={{ marginBottom: 4 }}>
              {t('auth.welcomeBack', { defaultValue: 'Welcome back' })}
            </Typography.Title>
            <Typography.Text type="secondary">
              {t('auth.signInSubtitle', { defaultValue: 'Sign in with your administrator account.' })}
            </Typography.Text>
          </div>

          {error ? <Alert type="error" showIcon message={error} /> : null}

          <Form<LoginFormValues> layout="vertical" requiredMark={false} onFinish={submit}>
            <Form.Item
              name="username"
              label={t('auth.username', { defaultValue: 'Username' })}
              rules={[{ required: true, message: t('auth.usernameRequired', { defaultValue: 'Username is required' }) }]}
            >
              <Input autoComplete="username" prefix={<UserOutlined />} />
            </Form.Item>
            <Form.Item
              name="password"
              label={t('auth.password', { defaultValue: 'Password' })}
              rules={[{ required: true, message: t('auth.passwordRequired', { defaultValue: 'Password is required' }) }]}
            >
              <Input.Password autoComplete="current-password" prefix={<LockOutlined />} />
            </Form.Item>
            <Button
              block
              htmlType="submit"
              type="primary"
              icon={<LoginOutlined />}
              loading={adminLogin.isPending}
              disabled={cooldown > 0}
            >
              {cooldown > 0
                ? t('auth.retryInSeconds', { defaultValue: 'Try again in {{seconds}}s', seconds: cooldown })
                : t('auth.signIn', { defaultValue: 'Sign in' })}
            </Button>
          </Form>

          {providers.length > 0 ? (
            <>
              <Divider plain>{t('auth.orUseOidc', { defaultValue: 'Or use SSO' })}</Divider>
              <Space direction="vertical" style={{ width: '100%' }}>
                {providers.map((provider) => (
                  <Button
                    block
                    key={provider.name}
                    loading={oidcStart.isPending}
                    onClick={() => void startOidc(provider)}
                  >
                    {providerLabel(provider)}
                  </Button>
                ))}
              </Space>
            </>
          ) : null}
        </Space>
      </Card>
    </main>
  )
}

export default Login
