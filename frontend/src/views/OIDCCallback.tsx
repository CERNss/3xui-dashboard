import { Alert, Avatar, Button, Card, Collapse, Divider, Form, Input, Result, Space, Spin, Tabs, Typography } from 'antd'
import type { AxiosError } from 'axios'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  portalAuthApi,
  type OIDCCallbackResponse,
  type OIDCPendingResponse,
  type UserTokenResponse,
} from '@/api/portal/auth'
import { usePortalAuthStore } from '@/stores/portalAuth'

type ErrorKind =
  | 'emailConflict'
  | 'emailMismatch'
  | 'stateInvalid'
  | 'domainNotAllowed'
  | 'accountSuspended'
  | 'emailUnverified'
  | 'notConfigured'
  | 'unknown'
  | 'invalidEntry'

interface TypedOidcError {
  kind: ErrorKind
  status?: number
  body: string
  domain?: string
}

function safeLocalRedirect(value: string | null | undefined): string | null {
  if (!value || !value.startsWith('/') || value.startsWith('//')) return null
  try {
    const url = new URL(value, window.location.origin)
    if (url.origin !== window.location.origin) return null
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

function stringifyBody(value: unknown): string {
  if (!value) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function errorStatus(error: unknown) {
  const apiError = error as { status?: number; response?: { status?: number } }
  return apiError?.status ?? apiError?.response?.status
}

function errorBody(error: unknown) {
  const apiError = error as { message?: string; data?: unknown; response?: { data?: unknown } }
  const data = apiError?.data ?? apiError?.response?.data
  const body = stringifyBody(data)
  return body || apiError?.message || stringifyBody((error as AxiosError)?.response?.data)
}

function rejectedDomain(body: string) {
  const emailMatch = body.match(/[A-Z0-9._%+-]+@([A-Z0-9.-]+\.[A-Z]{2,})/i)
  if (emailMatch?.[1]) return emailMatch[1]
  const domainMatch = body.match(/domain[^A-Z0-9.-]+([A-Z0-9.-]+\.[A-Z]{2,})/i)
  return domainMatch?.[1]
}

export function classifyOidcError(error: unknown): TypedOidcError {
  const status = errorStatus(error)
  const body = errorBody(error)
  const lower = body.toLowerCase()

  if (status === 409 && lower.includes('oidc: email already linked to a different account')) {
    return { kind: 'emailConflict', status, body }
  }
  if (status === 409 && lower.includes('oidc email does not match')) {
    return { kind: 'emailMismatch', status, body }
  }
  if (status === 400 && lower.includes('state')) {
    return { kind: 'stateInvalid', status, body }
  }
  if (status === 400 && lower.includes('verified email')) {
    return { kind: 'emailUnverified', status, body }
  }
  if (status === 403 && lower.includes('domain')) {
    return { kind: 'domainNotAllowed', status, body, domain: rejectedDomain(body) }
  }
  if (status === 403 && lower.includes('suspended')) {
    return { kind: 'accountSuspended', status, body }
  }
  if (status === 501) {
    return { kind: 'notConfigured', status, body }
  }
  return { kind: 'unknown', status, body }
}

function isPendingResponse(res: OIDCCallbackResponse): res is OIDCPendingResponse {
  return 'status' in res && res.status === 'pending'
}

interface BindExistingValues {
  password: string
}

interface CreateAccountValues {
  display_name: string
  email: string
  password: string
  confirm_password: string
  code: string
}

function pendingProviderEmail(pending: OIDCPendingResponse) {
  return pending.provider_email || pending.email || ''
}

function pendingProviderName(pending: OIDCPendingResponse) {
  return pending.provider?.name || 'OIDC'
}

export function OIDCCallback() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const setSession = usePortalAuthStore((state) => state.setSession)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<TypedOidcError | null>(null)
  const [pending, setPending] = useState<OIDCPendingResponse | null>(null)
  const [bindForm] = Form.useForm<BindExistingValues>()
  const [createForm] = Form.useForm<CreateAccountValues>()
  const [binding, setBinding] = useState(false)
  const [creating, setCreating] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)

  const params = useMemo(() => new URLSearchParams(location.search), [location.search])

  const acceptToken = useCallback((res: UserTokenResponse) => {
    setSession(res.token, { id: res.user_id, email: res.email })
    navigate(
      safeLocalRedirect(res.redirect_after) ??
        safeLocalRedirect(res.next) ??
        safeLocalRedirect(params.get('next')) ??
        '/portal/subscription',
      { replace: true },
    )
  }, [navigate, params, setSession])

  useEffect(() => {
    const code = params.get('code')
    const state = params.get('state')
    if (!code || !state) {
      setError({ kind: 'invalidEntry', body: '' })
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)
    portalAuthApi
      .oidcCallback(code, state)
      .then((res) => {
        if (cancelled) return
        if (isPendingResponse(res)) {
          setPending(res)
          setLoading(false)
          return
        }
        acceptToken(res)
      })
      .catch((e) => {
        if (!cancelled) {
          setError(classifyOidcError(e))
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [acceptToken, params])

  async function bindExisting(values: BindExistingValues) {
    if (!pending) return
    setBinding(true)
    setError(null)
    try {
      acceptToken(
        await portalAuthApi.oidcBindExisting({
          pendingToken: pending.pending_token,
          password: values.password,
        }),
      )
    } catch (e) {
      setError(classifyOidcError(e))
    } finally {
      setBinding(false)
    }
  }

  async function sendCreateCode() {
    if (!pending) return
    const email = createForm.getFieldValue('email')
    await createForm.validateFields(['email'])
    setSendingCode(true)
    setError(null)
    try {
      await portalAuthApi.startEmailVerification({ email, purpose: 'oidc_create_account' })
    } catch (e) {
      setError(classifyOidcError(e))
    } finally {
      setSendingCode(false)
    }
  }

  async function createAccount(values: CreateAccountValues) {
    if (!pending) return
    setCreating(true)
    setError(null)
    try {
      acceptToken(
        await portalAuthApi.oidcCreateAccount({
          pendingToken: pending.pending_token,
          displayName: values.display_name,
          email: values.email,
          password: values.password,
          verificationToken: (
            await portalAuthApi.confirmEmailVerification({
              email: values.email,
              code: values.code,
              purpose: 'oidc_create_account',
            })
          ).verification_token,
        }),
      )
    } catch (e) {
      setError(classifyOidcError(e))
    } finally {
      setCreating(false)
    }
  }

  async function restartOidc() {
    const res = await portalAuthApi.oidcStart(safeLocalRedirect(params.get('next')) ?? '/portal/subscription')
    window.location.assign(res.authorize_url)
  }

  if (loading) {
    return (
      <section className="auth-surface">
        <Card>
          <Spin /> <Typography.Text>{t('auth.oidcReturning')}</Typography.Text>
        </Card>
      </section>
    )
  }

  if (pending) {
    return (
      <section className="auth-surface">
        <Card style={{ width: 'min(100%, 560px)' }}>
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <Space align="start">
              <Avatar src={pending.provider?.icon || undefined}>{pendingProviderName(pending).slice(0, 1)}</Avatar>
              <div>
                <Typography.Title level={3} style={{ marginBottom: 4 }}>
                  {t('auth.oidcDecisionTitle')}
                </Typography.Title>
                <Typography.Text type="secondary">
                  {t('auth.oidcCallback.providerReturned', {
                    email: pendingProviderEmail(pending),
                    provider: pendingProviderName(pending),
                  })}
                </Typography.Text>
              </div>
            </Space>
            {error ? <Alert type="error" showIcon message={t(`auth.oidcCallback.${error.kind}`)} /> : null}
            <Tabs
              defaultActiveKey={pending.existing_user ? 'bind' : 'create'}
              items={[
                {
                  key: 'bind',
                  label: t('auth.oidcBindExisting'),
                  children: (
                    <Form<BindExistingValues>
                      form={bindForm}
                      layout="vertical"
                      requiredMark={false}
                      onFinish={(values) => void bindExisting(values)}
                    >
                      <Typography.Paragraph type="secondary">
                        {t('auth.oidcCallback.bindHint')}
                      </Typography.Paragraph>
                      <Form.Item
                        name="password"
                        label={t('auth.password')}
                        rules={[{ required: true, message: t('auth.oidcCallback.passwordRequired') }]}
                      >
                        <Input.Password autoComplete="current-password" />
                      </Form.Item>
                      <Button type="primary" htmlType="submit" loading={binding}>
                        {t('auth.oidcBindExisting')}
                      </Button>
                    </Form>
                  ),
                },
                {
                  key: 'create',
                  label: t('auth.oidcCallback.createNew'),
                  children: (
                    <Form<CreateAccountValues>
                      form={createForm}
                      layout="vertical"
                      requiredMark={false}
                      initialValues={{ email: pendingProviderEmail(pending) }}
                      onFinish={(values) => void createAccount(values)}
                    >
                      <Typography.Paragraph type="secondary">
                        {t('auth.oidcCallback.createHint')}
                      </Typography.Paragraph>
                      <Form.Item
                        name="display_name"
                        label={t('auth.oidcCallback.displayName')}
                        rules={[{ required: true, whitespace: true, message: t('auth.oidcCallback.displayNameRequired') }]}
                      >
                        <Input autoComplete="nickname" />
                      </Form.Item>
                      <Form.Item
                        name="email"
                        label={t('auth.email')}
                        rules={[
                          { required: true, message: t('auth.enterValidEmail') },
                          { type: 'email', message: t('auth.enterValidEmail') },
                        ]}
                      >
                        <Input autoComplete="email" />
                      </Form.Item>
                      <Divider style={{ margin: '8px 0' }} />
                      <Form.Item
                        name="password"
                        label={t('auth.password')}
                        rules={[
                          { required: true, message: t('auth.oidcCallback.passwordRequired') },
                          { min: 8, message: t('auth.passwordTooShort') },
                        ]}
                      >
                        <Input.Password autoComplete="new-password" />
                      </Form.Item>
                      <Form.Item
                        name="confirm_password"
                        label={t('auth.confirmPassword')}
                        dependencies={['password']}
                        rules={[
                          { required: true, message: t('auth.confirmPassword') },
                          ({ getFieldValue }) => ({
                            validator(_, value) {
                              if (!value || getFieldValue('password') === value) return Promise.resolve()
                              return Promise.reject(new Error(t('auth.passwordsMustMatch')))
                            },
                          }),
                        ]}
                      >
                        <Input.Password autoComplete="new-password" />
                      </Form.Item>
                      <Form.Item
                        name="code"
                        label={t('auth.verificationCode')}
                        rules={[{ required: true, message: t('auth.codeMustBe6') }]}
                      >
                        <Input maxLength={6} autoComplete="one-time-code" />
                      </Form.Item>
                      <Space wrap>
                        <Button loading={sendingCode} onClick={() => void sendCreateCode()}>
                          {t('auth.sendCode')}
                        </Button>
                        <Button type="primary" htmlType="submit" loading={creating}>
                          {t('auth.createAccount')}
                        </Button>
                      </Space>
                    </Form>
                  ),
                },
              ]}
            />
          </Space>
        </Card>
      </section>
    )
  }

  return <OidcErrorResult error={error} onRestart={() => void restartOidc()} />
}

function OidcErrorResult({ error, onRestart }: { error: TypedOidcError | null; onRestart: () => void }) {
  const { t } = useTranslation()
  const kind = error?.kind ?? 'unknown'
  const message = t(`auth.oidcCallback.${kind}`)

  const extra = (
    <Space wrap>
      {kind !== 'notConfigured' && kind !== 'emailMismatch' && kind !== 'accountSuspended' ? (
        <Button type="primary" onClick={kind === 'stateInvalid' ? onRestart : undefined}>
          {kind === 'emailConflict'
            ? t('auth.oidcCallback.signInFirst')
            : t('auth.oidcCallback.tryAgain')}
        </Button>
      ) : null}
      {kind === 'emailConflict' ? (
        <Button onClick={onRestart}>{t('auth.oidcCallback.useDifferentAccount')}</Button>
      ) : null}
      {kind === 'emailMismatch' ? (
        <Button type="primary">
          <Link to="/portal/profile">{t('auth.oidcCallback.backToProfile')}</Link>
        </Button>
      ) : null}
      <Button>
        <Link to="/login">{t('auth.returnToLogin')}</Link>
      </Button>
    </Space>
  )

  return (
    <section className="auth-surface">
      <Result
        status="error"
        title={t('auth.oidcReturningTitleFailed')}
        subTitle={
          <Space direction="vertical">
            <span>{message}</span>
            {kind === 'domainNotAllowed' && error?.domain ? (
              <Typography.Text type="secondary">{error.domain}</Typography.Text>
            ) : null}
          </Space>
        }
        extra={extra}
      />
      {kind === 'unknown' && error?.body ? (
        <Collapse
          style={{ maxWidth: 720, margin: '0 auto' }}
          items={[{ key: 'details', label: t('auth.oidcCallback.details'), children: <pre>{error.body}</pre> }]}
        />
      ) : null}
    </section>
  )
}

export default OIDCCallback
