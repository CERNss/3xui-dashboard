import { LockOutlined, LoginOutlined, MailOutlined, UserAddOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Divider, Form, Input, Space, Tabs, Typography } from 'antd'
import type { FormInstance } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router-dom'
import { adminAuthApi } from '@/api/admin/auth'
import { portalAuthApi, type OIDCProvider, type UserTokenResponse } from '@/api/portal/auth'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { formatError } from '@/utils/format'

type AuthMode = 'login' | 'register'

interface LoginFormValues {
  email: string
  password: string
}

interface RegisterFormValues {
  email: string
  password: string
  confirm_password: string
  code?: string
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

function rolePath(value: string | null | undefined, prefix: '/admin' | '/portal', fallback: string) {
  const path = localPath(value ?? null, fallback)
  return path === prefix || path.startsWith(`${prefix}/`) ? path : fallback
}

function providerLabel(provider: OIDCProvider) {
  return provider.name || 'OIDC'
}

function providerKey(provider: OIDCProvider) {
  return provider.key || provider.name
}

function normalizeEmail(value: unknown) {
  return typeof value === 'string' ? value.trim() : value
}

function isCredentialRejection(error: unknown) {
  const status = (error as { status?: number; response?: { status?: number } })?.status
    ?? (error as { response?: { status?: number } })?.response?.status
  return status === 400 || status === 401 || status === 403 || status === 404
}

function portalSessionUser(res: UserTokenResponse) {
  return { id: res.user_id, email: res.email }
}

export function Login() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const setAdminSession = useAdminAuthStore((state) => state.setSession)
  const setPortalSession = usePortalAuthStore((state) => state.setSession)
  const [loginForm] = Form.useForm<LoginFormValues>()
  const [registerForm] = Form.useForm<RegisterFormValues>()
  const [providers, setProviders] = useState<OIDCProvider[]>([])
  const [mode, setMode] = useState<AuthMode>('login')
  const [error, setError] = useState<string | null>(null)
  const [signingIn, setSigningIn] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [startingOidc, setStartingOidc] = useState(false)
  const [cooldown, setCooldown] = useState(0)
  const [codeCooldown, setCodeCooldown] = useState(0)
  const [verificationRequired, setVerificationRequired] = useState(false)

  const params = useMemo(() => new URLSearchParams(location.search), [location.search])
  const adminNextPath = rolePath(params.get('next'), '/admin', '/admin/status')
  const portalNextPath = rolePath(params.get('next'), '/portal', '/portal/subscription')

  useEffect(() => {
    setMode(params.get('mode') === 'register' ? 'register' : 'login')
  }, [params])

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
    let cancelled = false
    portalAuthApi
      .registrationPolicy()
      .then((policy) => {
        if (!cancelled) setVerificationRequired(Boolean(policy.email_verification_required))
      })
      .catch(() => {
        if (!cancelled) setVerificationRequired(false)
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

  useEffect(() => {
    if (codeCooldown <= 0) return undefined
    const timer = window.setTimeout(() => setCodeCooldown((value) => Math.max(0, value - 1)), 1000)
    return () => window.clearTimeout(timer)
  }, [codeCooldown])

  async function submitLogin(values: LoginFormValues) {
    if (cooldown > 0) return
    const email = values.email.trim()
    setError(null)
    setSigningIn(true)
    try {
      const adminRes = await adminAuthApi.login(email, values.password)
      setAdminSession(adminRes.token, adminRes.username)
      navigate(adminNextPath, { replace: true })
      return
    } catch (adminError) {
      if (!isCredentialRejection(adminError)) {
        setError(formatError(adminError, t('auth.loginFailed')))
        setCooldown(3)
        setSigningIn(false)
        return
      }
    }

    try {
      const portalRes = await portalAuthApi.login({
        email,
        password: values.password,
      })
      setPortalSession(portalRes.token, portalSessionUser(portalRes))
      navigate(
        rolePath(portalRes.redirect_after ?? portalRes.next, '/portal', portalNextPath),
        { replace: true },
      )
    } catch (portalError) {
      setError(formatError(portalError, t('auth.wrongCredentials')))
      setCooldown(3)
    } finally {
      setSigningIn(false)
    }
  }

  async function submitRegister(values: RegisterFormValues) {
    setError(null)
    setRegistering(true)
    try {
      const res = await portalAuthApi.register({
        email: values.email,
        password: values.password,
        code: verificationRequired ? values.code : undefined,
      })
      setPortalSession(res.token, portalSessionUser(res))
      navigate(rolePath(res.redirect_after ?? res.next, '/portal', portalNextPath), { replace: true })
    } catch (registerError) {
      setError(formatError(registerError, t('auth.registerFailed')))
    } finally {
      setRegistering(false)
    }
  }

  async function sendRegisterCode() {
    if (codeCooldown > 0) return
    setError(null)
    try {
      await registerForm.validateFields(['email'])
      const email = registerForm.getFieldValue('email')
      setSendingCode(true)
      const res = await portalAuthApi.startEmailVerification({ email, purpose: 'register' })
      setCodeCooldown(res.resend_after_seconds ?? res.cooldown_seconds ?? 60)
    } catch (codeError) {
      if (!(codeError && typeof codeError === 'object' && 'errorFields' in codeError)) {
        setError(formatError(codeError, t('auth.codeFailedToSend')))
      }
    } finally {
      setSendingCode(false)
    }
  }

  async function startOidc(provider: OIDCProvider) {
    setError(null)
    setStartingOidc(true)
    try {
      if (provider.login_url && /^https?:\/\//i.test(provider.login_url)) {
        window.location.assign(provider.login_url)
        return
      }
      const res = await portalAuthApi.oidcStart(portalNextPath, provider.key)
      window.location.assign(res.authorize_url)
    } catch (e) {
      setError(formatError(e, t('auth.oidcStarting')))
      setStartingOidc(false)
    }
  }

  function switchMode(nextMode: string) {
    const next = nextMode as AuthMode
    setMode(next)
    setError(null)
    setCooldown(0)
    if (next === 'register') {
      registerForm.setFieldValue('code', undefined)
      setCodeCooldown(0)
    }
  }

  return (
    <section className="auth-surface auth-login-surface">
      <Card className="auth-login-card" styles={{ body: { padding: 30 } }}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          {error ? <Alert type="error" showIcon message={error} /> : null}

          <Tabs
            activeKey={mode}
            centered
            className="auth-tabs"
            items={[
              {
                key: 'login',
                label: t('auth.loginTab'),
                children: (
                  <LoginPane
                    cooldown={cooldown}
                    form={loginForm}
                    loading={signingIn}
                    onOidcStart={startOidc}
                    onSubmit={submitLogin}
                    oidcLoading={startingOidc}
                    providers={providers}
                    t={t}
                  />
                ),
              },
              {
                key: 'register',
                label: t('auth.registerTab'),
                children: (
                  <RegisterPane
                    codeCooldown={codeCooldown}
                    form={registerForm}
                    loading={registering}
                    onSendCode={sendRegisterCode}
                    onSubmit={submitRegister}
                    sendingCode={sendingCode}
                    t={t}
                    verificationRequired={verificationRequired}
                  />
                ),
              },
            ]}
            onChange={switchMode}
          />
        </Space>
      </Card>
    </section>
  )
}

interface LoginPaneProps {
  cooldown: number
  form: FormInstance<LoginFormValues>
  loading: boolean
  oidcLoading: boolean
  providers: OIDCProvider[]
  t: ReturnType<typeof useTranslation>['t']
  onSubmit: (values: LoginFormValues) => Promise<void>
  onOidcStart: (provider: OIDCProvider) => Promise<void>
}

function LoginPane({
  cooldown,
  form,
  loading,
  oidcLoading,
  onOidcStart,
  onSubmit,
  providers,
  t,
}: LoginPaneProps) {
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Form<LoginFormValues> form={form} layout="vertical" name="login" requiredMark={false} onFinish={onSubmit}>
        <Form.Item
          name="email"
          label={t('auth.email')}
          normalize={normalizeEmail}
          rules={[
            { required: true, message: t('auth.emailRequired') },
            { type: 'email', message: t('auth.emailInvalid') },
          ]}
        >
          <Input autoComplete="email" inputMode="email" prefix={<MailOutlined />} />
        </Form.Item>
        <Form.Item
          name="password"
          label={t('auth.password')}
          rules={[{ required: true, message: t('auth.passwordRequired') }]}
        >
          <Input.Password
            aria-label={t('auth.password')}
            autoComplete="current-password"
            prefix={<LockOutlined />}
          />
        </Form.Item>
        <Button
          block
          htmlType="submit"
          type="primary"
          icon={<LoginOutlined />}
          loading={loading}
          disabled={cooldown > 0}
        >
          {cooldown > 0 ? t('auth.retryInSeconds', { seconds: cooldown }) : t('auth.signIn')}
        </Button>
      </Form>

      {providers.length > 0 ? (
        <>
          <Divider plain>{t('auth.orUseOidc')}</Divider>
          <Space direction="vertical" style={{ width: '100%' }}>
            {providers.map((provider) => (
              <Button
                block
                className="auth-oidc-button"
                key={providerKey(provider)}
                loading={oidcLoading}
                onClick={() => void onOidcStart(provider)}
              >
                {providerLabel(provider)}
              </Button>
            ))}
          </Space>
        </>
      ) : null}
    </Space>
  )
}

interface RegisterPaneProps {
  codeCooldown: number
  form: FormInstance<RegisterFormValues>
  loading: boolean
  sendingCode: boolean
  t: ReturnType<typeof useTranslation>['t']
  verificationRequired: boolean
  onSendCode: () => Promise<void>
  onSubmit: (values: RegisterFormValues) => Promise<void>
}

function RegisterPane({
  codeCooldown,
  form,
  loading,
  onSendCode,
  onSubmit,
  sendingCode,
  t,
  verificationRequired,
}: RegisterPaneProps) {
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Text className="auth-tab-note" type="secondary">
        {t('auth.registerSubtitle')}
      </Typography.Text>
      <Form<RegisterFormValues> form={form} layout="vertical" name="register" requiredMark={false} onFinish={onSubmit}>
        <Form.Item
          name="email"
          label={t('auth.email')}
          normalize={normalizeEmail}
          rules={[
            { required: true, message: t('auth.emailRequired') },
            { type: 'email', message: t('auth.emailInvalid') },
          ]}
        >
          <Input autoComplete="email" prefix={<MailOutlined />} />
        </Form.Item>
        <Form.Item
          name="password"
          label={t('auth.password')}
          rules={[
            { required: true, message: t('auth.passwordRequired') },
            { min: 8, message: t('auth.passwordTooShort') },
          ]}
        >
          <Input.Password
            aria-label={t('auth.password')}
            autoComplete="new-password"
            prefix={<LockOutlined />}
          />
        </Form.Item>
        <Form.Item
          name="confirm_password"
          label={t('auth.confirmPassword')}
          dependencies={['password']}
          rules={[
            { required: true, message: t('auth.confirmPasswordRequired') },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('password') === value) return Promise.resolve()
                return Promise.reject(new Error(t('auth.passwordsMustMatch')))
              },
            }),
          ]}
        >
          <Input.Password
            aria-label={t('auth.confirmPassword')}
            autoComplete="new-password"
            prefix={<LockOutlined />}
          />
        </Form.Item>
        {verificationRequired ? (
          <>
            <Form.Item className="auth-code-item" label={t('auth.verificationCode')} required>
              <Space.Compact block className="auth-code-control">
                <Form.Item name="code" noStyle rules={[{ required: true, message: t('auth.codeMustBe6') }]}>
                  <Input
                    aria-label={t('auth.verificationCode')}
                    autoComplete="one-time-code"
                    maxLength={6}
                    placeholder={t('auth.codePlaceholder')}
                  />
                </Form.Item>
                <Button
                  disabled={codeCooldown > 0}
                  loading={sendingCode}
                  onClick={() => void onSendCode()}
                >
                  {codeCooldown > 0 ? t('auth.codeRetry', { n: codeCooldown }) : t('auth.sendCode')}
                </Button>
              </Space.Compact>
            </Form.Item>
            <Typography.Text className="auth-code-hint" type="secondary">
              {t('auth.codeValidHint')}
            </Typography.Text>
          </>
        ) : null}
        <Button
          block
          htmlType="submit"
          type="primary"
          icon={<UserAddOutlined />}
          loading={loading}
        >
          {loading ? t('auth.registering') : t('auth.createAccount')}
        </Button>
      </Form>
    </Space>
  )
}

export default Login
