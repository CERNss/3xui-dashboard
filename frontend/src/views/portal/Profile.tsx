import {
  CalendarOutlined,
  CheckCircleOutlined,
  CreditCardOutlined,
  GlobalOutlined,
  LinkOutlined,
  MailOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Avatar,
  Button,
  Form,
  Input,
  Skeleton,
  Space,
  Tag,
  Typography,
  message,
} from 'antd'
import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { portalProfileApi } from '@/api/portal/profile'
import type { LoginMethodsResponse, OIDCProviderLink, UserProfile } from '@/api/portal/profile'
import {
  useChangeEmail,
  useChangePassword,
  useLoginMethods,
  useProfile,
  useStartEmailVerification,
  useStartOidcLink,
  useUpdateProfile,
} from '@/hooks/queries/portal/profile'
import { useOwnTraffic } from '@/hooks/queries/portal/traffic'
import { formatError } from '@/utils/format'
import { formatYuan } from './_shared/format'

interface DisplayNameFormValues {
  display_name?: string
}

interface EmailFormValues {
  email: string
  code: string
}

interface PasswordFormValues {
  old_password: string
  new_password: string
  confirm_password: string
}

function safeIconUrl(raw?: string | null) {
  if (!raw) return undefined
  return /^(https?:|data:image\/)/i.test(raw) ? raw : undefined
}

function oidcProviderKey(provider: OIDCProviderLink) {
  return provider.key ?? provider.provider_key ?? ''
}

function oidcProviderName(provider: OIDCProviderLink) {
  return provider.name ?? provider.display_name ?? 'OIDC'
}

function oidcProviderIcon(provider: OIDCProviderLink) {
  return provider.icon ?? provider.icon_url
}

function visibleOidcProviders(loginMethods?: LoginMethodsResponse): OIDCProviderLink[] {
  return loginMethods?.oidc_providers ?? []
}

export function Profile() {
  const { t } = useTranslation()
  const [messageApi, contextHolder] = message.useMessage()
  const [displayForm] = Form.useForm<DisplayNameFormValues>()
  const [emailForm] = Form.useForm<EmailFormValues>()
  const [passwordForm] = Form.useForm<PasswordFormValues>()
  const [emailPanelOpen, setEmailPanelOpen] = useState(false)

  const profile = useProfile()
  const methods = useLoginMethods()
  const traffic = useOwnTraffic()
  const updateProfile = useUpdateProfile()
  const startEmailVerification = useStartEmailVerification()
  const changeEmail = useChangeEmail()
  const changePassword = useChangePassword()
  const startOidcLink = useStartOidcLink()

  const loading = profile.isLoading || methods.isLoading
  const error = profile.error ?? methods.error ?? traffic.error
  const providers = useMemo(() => visibleOidcProviders(methods.data), [methods.data])
  const activeClientCount = traffic.data?.length ?? 0

  useEffect(() => {
    if (!profile.data) return
    displayForm.setFieldsValue({ display_name: profile.data.display_name ?? '' })
  }, [displayForm, profile.data])

  useEffect(() => {
    if (!profile.data || !emailPanelOpen) return
    emailForm.setFieldsValue({ email: profile.data.email ?? '' })
  }, [emailForm, emailPanelOpen, profile.data])

  async function saveDisplayName(values: DisplayNameFormValues) {
    await updateProfile.mutateAsync({ display_name: values.display_name?.trim() || null })
    messageApi.success(t('portal.profile.displayNameSaved'))
  }

  async function sendEmailCode() {
    const email = emailForm.getFieldValue('email')
    await emailForm.validateFields(['email'])
    await startEmailVerification.mutateAsync({ email, purpose: 'change_email' })
    messageApi.success(t('portal.profile.codeSent'))
  }

  async function submitEmail(values: EmailFormValues) {
    const verified = await portalProfileApi.confirmEmailVerification({
      email: values.email,
      code: values.code,
      purpose: 'change_email',
    })
    await changeEmail.mutateAsync({ email: values.email, verificationToken: verified.verification_token })
    emailForm.resetFields(['code'])
    messageApi.success(t('portal.profile.emailSaved'))
  }

  async function submitPassword(values: PasswordFormValues) {
    await changePassword.mutateAsync({
      oldPassword: values.old_password,
      newPassword: values.new_password,
    })
    passwordForm.resetFields()
    messageApi.success(t('portal.profile.changePwOk'))
  }

  async function connectProvider(provider: OIDCProviderLink) {
    const result = await startOidcLink.mutateAsync({
      providerKey: oidcProviderKey(provider),
      redirectAfter: '/portal/profile?linked=oidc',
    })
    window.location.assign(result.authorize_url)
  }

  return (
    <>
      {contextHolder}

      {error ? (
        <Alert showIcon type="error" style={{ marginBottom: 16 }} message={formatError(error, t('portal.profile.loadFailed'))} />
      ) : null}

      {loading ? (
        <Skeleton active />
      ) : profile.data ? (
        <div className="portal-profile-page">
          <AccountSummary
            activeClientCount={activeClientCount}
            activeClientLoading={traffic.isLoading}
            profile={profile.data}
          />

          <section className="portal-profile-section">
            <div className="portal-profile-section-heading">
              <Typography.Title level={3}>{t('portal.profile.profileSectionTitle')}</Typography.Title>
              <Typography.Text type="secondary">{t('portal.profile.profileSectionSubtitle')}</Typography.Text>
            </div>

            <div className="portal-profile-grid">
              <div className="portal-profile-panel portal-profile-avatar-panel">
                <ProfileAvatar profile={profile.data} size="large" />
                <div className="portal-profile-panel-copy">
                  <Typography.Title level={4}>{t('portal.profile.avatarTitle')}</Typography.Title>
                  <Typography.Text type="secondary">{t('portal.profile.avatarHint')}</Typography.Text>
                </div>
              </div>

              <div className="portal-profile-panel">
                <Typography.Title level={4}>{t('portal.profile.editProfile')}</Typography.Title>
                <Form<DisplayNameFormValues>
                  form={displayForm}
                  layout="vertical"
                  requiredMark={false}
                  onFinish={(values) => void saveDisplayName(values)}
                >
                  <Form.Item
                    name="display_name"
                    label={t('portal.profile.username')}
                  >
                    <Input autoComplete="nickname" maxLength={80} />
                  </Form.Item>
                  <div className="portal-profile-form-actions">
                    <Button type="primary" htmlType="submit" loading={updateProfile.isPending}>
                      {t('portal.profile.updateProfile')}
                    </Button>
                  </div>
                </Form>
              </div>
            </div>
          </section>

          <section className="portal-profile-section portal-profile-password-section">
            <div className="portal-profile-section-heading portal-profile-password-heading">
              <Typography.Title level={3}>{t('portal.profile.changePw')}</Typography.Title>
            </div>

            <Form<PasswordFormValues>
              className="portal-profile-password-form"
              form={passwordForm}
              layout="vertical"
              requiredMark={false}
              onFinish={(values) => void submitPassword(values)}
            >
              <Form.Item
                name="old_password"
                label={t('portal.profile.currentPw')}
                rules={[{ required: true, message: t('portal.profile.currentPw') }]}
              >
                <Input.Password autoComplete="current-password" />
              </Form.Item>
              <Form.Item
                name="new_password"
                label={t('portal.profile.newPw')}
                extra={t('portal.profile.pwMin8')}
                rules={[
                  { required: true, message: t('portal.profile.newPw') },
                  { min: 8, message: t('portal.profile.newPwMin8') },
                ]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                name="confirm_password"
                label={t('portal.profile.confirmPw')}
                dependencies={['new_password']}
                rules={[
                  { required: true, message: t('portal.profile.confirmPw') },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                      return Promise.reject(new Error(t('portal.profile.pwsMustMatch')))
                    },
                  }),
                ]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <div className="portal-profile-form-actions">
                <Button type="primary" htmlType="submit" loading={changePassword.isPending}>
                  {t('portal.profile.updatePw')}
                </Button>
              </div>
            </Form>
          </section>

          <section className="portal-profile-section">
            <div className="portal-profile-section-heading">
              <Typography.Title level={3}>{t('portal.profile.loginMethods.title')}</Typography.Title>
              <Typography.Text type="secondary">{t('portal.profile.loginMethods.subtitle')}</Typography.Text>
            </div>

            <div className="portal-profile-method-list">
              <div className="portal-profile-method-row">
                <div className="portal-profile-method-icon">
                  <MailOutlined />
                </div>
                <div className="portal-profile-method-main">
                  <div className="portal-profile-method-title">
                    <Typography.Text strong>{t('portal.profile.loginMethods.emailTitle')}</Typography.Text>
                    <Tag color={profile.data.email_verified ? 'green' : 'gold'}>
                      {profile.data.email_verified ? t('portal.profile.verified') : t('portal.profile.unverified')}
                    </Tag>
                  </div>
                  <Typography.Text>{profile.data.email || t('portal.profile.noEmail')}</Typography.Text>
                  <Typography.Text type="secondary">{t('portal.profile.loginMethods.emailHint')}</Typography.Text>
                </div>
                <Button onClick={() => setEmailPanelOpen((value) => !value)}>
                  {t('portal.profile.loginMethods.manageEmail')}
                </Button>
              </div>

              {emailPanelOpen ? (
                <div className="portal-profile-inline-form">
                  <Form<EmailFormValues>
                    form={emailForm}
                    layout="vertical"
                    requiredMark={false}
                    onFinish={(values) => void submitEmail(values)}
                  >
                    <div className="portal-profile-form-grid">
                      <Form.Item
                        name="email"
                        label={t('portal.profile.column.email')}
                        rules={[
                          { required: true, message: t('auth.enterValidEmail') },
                          { type: 'email', message: t('auth.enterValidEmail') },
                        ]}
                      >
                        <Input autoComplete="email" />
                      </Form.Item>
                      <Form.Item
                        name="code"
                        label={t('auth.verificationCode')}
                        rules={[{ required: true, message: t('auth.codeMustBe6') }]}
                      >
                        <Input maxLength={6} autoComplete="one-time-code" />
                      </Form.Item>
                    </div>
                    <Space wrap>
                      <Button loading={startEmailVerification.isPending} onClick={() => void sendEmailCode()}>
                        {t('portal.profile.sendVerificationCode')}
                      </Button>
                      <Button type="primary" htmlType="submit" loading={changeEmail.isPending}>
                        {t('portal.profile.updateEmail')}
                      </Button>
                    </Space>
                  </Form>
                </div>
              ) : null}

              {providers.length > 0 ? (
                providers.map((provider) => (
                  <div
                    className="portal-profile-method-row"
                    data-testid={`oidc-provider-${oidcProviderKey(provider)}`}
                    key={oidcProviderKey(provider)}
                  >
                    <div className="portal-profile-method-icon">
                      <ProviderAvatar provider={provider} />
                    </div>
                    <div className="portal-profile-method-main">
                      <div className="portal-profile-method-title">
                        <Typography.Text strong>{oidcProviderName(provider)}</Typography.Text>
                        <Tag color={provider.linked ? 'green' : 'default'}>
                          {provider.linked ? t('portal.profile.loginMethods.bound') : t('portal.profile.loginMethods.unbound')}
                        </Tag>
                      </div>
                      <Typography.Text type="secondary">
                        {provider.linked
                          ? provider.provider_email || t('portal.profile.loginMethods.oidcBoundText', { provider: oidcProviderName(provider) })
                          : t('portal.profile.loginMethods.oidcUnboundText', { provider: oidcProviderName(provider) })}
                      </Typography.Text>
                    </div>
                    {provider.linked ? (
                      <Tag color="success" icon={<CheckCircleOutlined />}>
                        {t('portal.profile.loginMethods.bound')}
                      </Tag>
                    ) : (
                      <Button
                        type="primary"
                        icon={<LinkOutlined />}
                        loading={startOidcLink.isPending}
                        onClick={() => void connectProvider(provider)}
                      >
                        {t('portal.profile.loginMethods.linkProvider', { provider: oidcProviderName(provider) })}
                      </Button>
                    )}
                  </div>
                ))
              ) : (
                <div className="portal-profile-method-empty">
                  {t('portal.profile.loginMethods.oidcUnavailableHint')}
                </div>
              )}
            </div>
          </section>
        </div>
      ) : null}
    </>
  )
}

function AccountSummary({
  activeClientCount,
  activeClientLoading,
  profile,
}: {
  activeClientCount: number
  activeClientLoading: boolean
  profile: UserProfile
}) {
  const { t } = useTranslation()
  const displayName = displayNameForProfile(profile)
  return (
    <section className="portal-profile-hero">
      <ProfileAvatar profile={profile} size="hero" />
      <div className="portal-profile-hero-main">
        <div className="portal-profile-identity">
          <Typography.Title level={2}>{displayName}</Typography.Title>
          <Tag>{t('account.userRole')}</Tag>
          <Tag color={profile.status === 'active' ? 'green' : 'red'}>
            {profile.status === 'active' ? t('portal.profile.status.active') : t('portal.profile.status.suspended')}
          </Tag>
        </div>
        <Typography.Text className="portal-profile-email">{profile.email || t('portal.profile.noEmail')}</Typography.Text>
        <div className="portal-profile-stat-grid">
          <ProfileStat
            icon={<CreditCardOutlined />}
            label={t('portal.profile.summary.balance')}
            value={formatYuan(profile.balance_cents)}
          />
          <ProfileStat
            icon={<GlobalOutlined />}
            label={t('portal.profile.summary.accessLimit')}
            loading={activeClientLoading}
            value={String(activeClientCount)}
          />
          <ProfileStat
            icon={<CalendarOutlined />}
            label={t('portal.profile.summary.registeredAt')}
            value={formatProfileMonth(profile.created_at)}
          />
        </div>
      </div>
    </section>
  )
}

function ProfileStat({
  icon,
  label,
  loading = false,
  value,
}: {
  icon: ReactNode
  label: string
  loading?: boolean
  value: string
}) {
  return (
    <div className="portal-profile-stat">
      <span className="portal-profile-stat-icon" aria-hidden="true">
        {icon}
      </span>
      <span className="portal-profile-stat-label">{label}</span>
      {loading ? <Skeleton.Input active size="small" /> : <span className="portal-profile-stat-value">{value}</span>}
    </div>
  )
}

function ProviderAvatar({ provider }: { provider: OIDCProviderLink }) {
  const icon = safeIconUrl(oidcProviderIcon(provider))
  if (icon) return <Avatar className="portal-provider-avatar" src={icon} />
  return <Avatar className="portal-provider-avatar" icon={<GlobalOutlined />} />
}

function ProfileAvatar({ profile, size }: { profile: UserProfile; size: 'hero' | 'large' }) {
  return (
    <span className={`portal-profile-avatar portal-profile-avatar--${size}`} aria-hidden="true">
      {initialsForProfile(profile)}
    </span>
  )
}

function displayNameForProfile(profile: UserProfile) {
  return profile.display_name?.trim() || profile.email?.split('@')[0] || `User #${profile.id}`
}

function initialsForProfile(profile: UserProfile) {
  const source = displayNameForProfile(profile).replace(/[^\p{L}\p{N}]/gu, '')
  return (Array.from(source).slice(0, 2).join('') || 'U').toUpperCase()
}

function formatProfileMonth(value: string) {
  return new Intl.DateTimeFormat(undefined, { month: 'short', year: 'numeric' }).format(new Date(value))
}

export default Profile
