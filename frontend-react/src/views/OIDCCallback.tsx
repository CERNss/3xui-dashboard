import { Alert, Button, Card, Collapse, Result, Space, Spin, Typography } from 'antd'
import type { AxiosError } from 'axios'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  portalAuthApi,
  type OIDCCallbackResponse,
  type OIDCPendingResponse,
  type OIDCResolveAction,
  type UserTokenResponse,
} from '@/api/portal/auth'
import { usePortalAuthStore } from '@/stores/portalAuth'

type ErrorKind =
  | 'emailConflict'
  | 'emailMismatch'
  | 'stateInvalid'
  | 'domainNotAllowed'
  | 'accountSuspended'
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

export function OIDCCallback() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const setSession = usePortalAuthStore((state) => state.setSession)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<TypedOidcError | null>(null)
  const [pending, setPending] = useState<OIDCPendingResponse | null>(null)
  const [resolving, setResolving] = useState<OIDCResolveAction | null>(null)

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

  async function resolvePending(action: OIDCResolveAction) {
    if (!pending) return
    setResolving(action)
    setError(null)
    try {
      acceptToken(await portalAuthApi.oidcResolve(pending.pending_token, action))
    } catch (e) {
      setError(classifyOidcError(e))
    } finally {
      setResolving(null)
    }
  }

  async function restartOidc() {
    const res = await portalAuthApi.oidcStart(safeLocalRedirect(params.get('next')) ?? '/portal/subscription')
    window.location.assign(res.authorize_url)
  }

  if (loading) {
    return (
      <main className="auth-surface">
        <Card>
          <Spin /> <Typography.Text>{t('auth.oidcReturning', { defaultValue: 'Completing OIDC login...' })}</Typography.Text>
        </Card>
      </main>
    )
  }

  if (pending) {
    return (
      <main className="auth-surface">
        <Card style={{ width: 'min(100%, 460px)' }}>
          <Space direction="vertical">
            <Typography.Title level={3}>{t('auth.oidcDecisionTitle', { defaultValue: 'Finish account link' })}</Typography.Title>
            <Typography.Paragraph>
              {t('auth.oidcDecisionBody', {
                defaultValue: 'Choose how to continue for {{email}}.',
                email: pending.email,
              })}
            </Typography.Paragraph>
            {error ? <Alert type="error" showIcon message={t(`auth.oidc.errors.${error.kind}`, { defaultValue: error.body })} /> : null}
            <Button type="primary" loading={resolving === 'bind'} onClick={() => void resolvePending('bind')}>
              {t('auth.oidcBindExisting', { defaultValue: 'Bind existing account' })}
            </Button>
            <Button loading={resolving === 'recreate'} onClick={() => void resolvePending('recreate')}>
              {t('auth.oidcRecreateAccount', { defaultValue: 'Create a new account' })}
            </Button>
          </Space>
        </Card>
      </main>
    )
  }

  return <OidcErrorResult error={error} onRestart={() => void restartOidc()} />
}

function OidcErrorResult({ error, onRestart }: { error: TypedOidcError | null; onRestart: () => void }) {
  const { t } = useTranslation()
  const kind = error?.kind ?? 'unknown'
  const message = t(`auth.oidc.errors.${kind}`, {
    defaultValue: {
      emailConflict: 'This OIDC email is already linked to a different account.',
      emailMismatch: 'The OIDC email does not match the currently bound account.',
      stateInvalid: 'This login link expired or failed the state check. Start again.',
      domainNotAllowed: 'This email domain is not allowed.',
      accountSuspended: 'This account is suspended.',
      notConfigured: 'OIDC login is not configured.',
      invalidEntry: 'This callback URL is missing code or state.',
      unknown: 'OIDC login failed.',
    }[kind],
  })

  const extra = (
    <Space wrap>
      {kind !== 'notConfigured' && kind !== 'emailMismatch' && kind !== 'accountSuspended' ? (
        <Button type="primary" onClick={kind === 'stateInvalid' ? onRestart : undefined}>
          {kind === 'emailConflict'
            ? t('auth.oidc.actions.signInFirst', { defaultValue: 'Sign in first, then link from Profile' })
            : t('auth.tryAgain', { defaultValue: 'Try again' })}
        </Button>
      ) : null}
      {kind === 'emailConflict' ? (
        <Button onClick={onRestart}>{t('auth.oidc.actions.useDifferentAccount', { defaultValue: 'Use a different OIDC account' })}</Button>
      ) : null}
      {kind === 'emailMismatch' ? (
        <Button type="primary">
          <Link to="/portal/profile">{t('auth.oidc.actions.backToProfile', { defaultValue: 'Back to Profile' })}</Link>
        </Button>
      ) : null}
      <Button>
        <Link to="/login">{t('auth.returnToLogin', { defaultValue: 'Return to login' })}</Link>
      </Button>
    </Space>
  )

  return (
    <main className="auth-surface">
      <Result
        status="error"
        title={t('auth.oidcReturningTitleFailed', { defaultValue: 'OIDC login failed' })}
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
          items={[{ key: 'details', label: t('errors.details', { defaultValue: 'Details' }), children: <pre>{error.body}</pre> }]}
        />
      ) : null}
    </main>
  )
}

export default OIDCCallback
