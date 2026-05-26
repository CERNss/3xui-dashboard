import {
  CheckCircleOutlined,
  GlobalOutlined,
  LinkOutlined,
  LockOutlined,
  MailOutlined,
  UserOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Avatar,
  Button,
  Card,
  Col,
  Form,
  Input,
  List,
  Row,
  Skeleton,
  Space,
  Tag,
  Typography,
  message,
} from 'antd'
import { useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { portalProfileApi } from '@/api/portal/profile'
import type { LoginMethodsResponse, OIDCProviderLink, UserProfile } from '@/api/portal/profile'
import { PageHeader, RefreshButton } from '@/components/common'
import {
  useChangeEmail,
  useChangePassword,
  useLoginMethods,
  useProfile,
  useStartEmailVerification,
  useStartOidcLink,
  useUpdateProfile,
} from '@/hooks/queries/portal/profile'
import { formatError } from '@/utils/format'

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

const labels = {
  codeSent: 'Verification code sent',
  connect: 'Connect',
  displayName: 'Display name',
  displayNameSaved: 'Display name saved',
  emailSaved: 'Email updated and verified',
  sendVerificationCode: 'Send verification code',
  updateEmail: 'Update email',
  verifiedEmail: 'Verified email',
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

  const profile = useProfile()
  const methods = useLoginMethods()
  const updateProfile = useUpdateProfile()
  const startEmailVerification = useStartEmailVerification()
  const changeEmail = useChangeEmail()
  const changePassword = useChangePassword()
  const startOidcLink = useStartOidcLink()

  const loading = profile.isLoading || methods.isLoading
  const refreshing = profile.isFetching || methods.isFetching
  const error = profile.error ?? methods.error
  const providers = useMemo(() => visibleOidcProviders(methods.data), [methods.data])

  useEffect(() => {
    if (!profile.data) return
    displayForm.setFieldsValue({ display_name: profile.data.display_name ?? '' })
    emailForm.setFieldsValue({ email: profile.data.email ?? '' })
  }, [displayForm, emailForm, profile.data])

  async function reload() {
    await Promise.all([profile.refetch(), methods.refetch()])
  }

  async function saveDisplayName(values: DisplayNameFormValues) {
    await updateProfile.mutateAsync({ display_name: values.display_name?.trim() || null })
    messageApi.success(labels.displayNameSaved)
  }

  async function sendEmailCode() {
    const email = emailForm.getFieldValue('email')
    await emailForm.validateFields(['email'])
    await startEmailVerification.mutateAsync({ email, purpose: 'change_email' })
    messageApi.success(labels.codeSent)
  }

  async function submitEmail(values: EmailFormValues) {
    const verified = await portalProfileApi.confirmEmailVerification({
      email: values.email,
      code: values.code,
      purpose: 'change_email',
    })
    await changeEmail.mutateAsync({ email: values.email, verificationToken: verified.verification_token })
    emailForm.resetFields(['code'])
    messageApi.success(labels.emailSaved)
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
      <PageHeader
        title={t('portal.profile.title')}
        subtitle={t('portal.profile.subtitle')}
        actions={<RefreshButton loading={refreshing} onClick={() => void reload()} label={t('portal.dashboard.refresh')} />}
      />

      {error ? (
        <Alert showIcon type="error" style={{ marginBottom: 16 }} message={formatError(error, t('portal.profile.loadFailed'))} />
      ) : null}

      {loading ? (
        <Skeleton active />
      ) : (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {profile.data ? <AccountSummary profile={profile.data} /> : null}

          <Card
            title={
              <Space>
                <UserOutlined />
                {labels.displayName}
              </Space>
            }
          >
            <Form<DisplayNameFormValues>
              form={displayForm}
              layout="vertical"
              requiredMark={false}
              onFinish={(values) => void saveDisplayName(values)}
            >
              <Form.Item
                name="display_name"
                label={labels.displayName}
              >
                <Input autoComplete="nickname" maxLength={80} />
              </Form.Item>
              <Button type="primary" htmlType="submit" loading={updateProfile.isPending}>
                {t('common.save')}
              </Button>
            </Form>
          </Card>

          <Card
            title={
              <Space>
                <MailOutlined />
                {labels.verifiedEmail}
              </Space>
            }
          >
            <Form<EmailFormValues>
              form={emailForm}
              layout="vertical"
              requiredMark={false}
              onFinish={(values) => void submitEmail(values)}
            >
              <Row gutter={12}>
                <Col xs={24} md={14}>
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
                </Col>
                <Col xs={24} md={10}>
                  <Form.Item
                    name="code"
                    label={t('auth.verificationCode')}
                    rules={[{ required: true, message: t('auth.codeMustBe6') }]}
                  >
                    <Input maxLength={6} autoComplete="one-time-code" />
                  </Form.Item>
                </Col>
              </Row>
              <Space wrap>
                <Button loading={startEmailVerification.isPending} onClick={() => void sendEmailCode()}>
                  {labels.sendVerificationCode}
                </Button>
                <Button type="primary" htmlType="submit" loading={changeEmail.isPending}>
                  {labels.updateEmail}
                </Button>
              </Space>
            </Form>
          </Card>

          <Card
            title={
              <Space>
                <LockOutlined />
                {t('portal.profile.changePw')}
              </Space>
            }
          >
            <Form<PasswordFormValues>
              form={passwordForm}
              layout="vertical"
              requiredMark={false}
              onFinish={(values) => void submitPassword(values)}
            >
              <Row gutter={12}>
                <Col xs={24} md={8}>
                  <Form.Item
                    name="old_password"
                    label={t('portal.profile.currentPw')}
                    rules={[{ required: true, message: t('portal.profile.currentPw') }]}
                  >
                    <Input.Password autoComplete="current-password" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item
                    name="new_password"
                    label={t('portal.profile.newPw')}
                    rules={[
                      { required: true, message: t('portal.profile.newPw') },
                      { min: 8, message: t('portal.profile.newPwMin8') },
                    ]}
                  >
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
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
                </Col>
              </Row>
              <Button type="primary" htmlType="submit" loading={changePassword.isPending}>
                {t('portal.profile.updatePw')}
              </Button>
            </Form>
          </Card>

          <Card
            title={
              <Space>
                <LinkOutlined />
                {t('portal.profile.loginMethods.title')}
              </Space>
            }
          >
            <List
              dataSource={providers}
              locale={{ emptyText: t('portal.profile.loginMethods.oidcUnavailableHint') }}
              renderItem={(provider) => (
                <List.Item
                  actions={[
                    provider.linked ? (
                      <Tag key="connected" color="success" icon={<CheckCircleOutlined />}>
                        {t('portal.profile.loginMethods.bound')}
                      </Tag>
                    ) : (
                      <Button
                        key="connect"
                        type="primary"
                        icon={<LinkOutlined />}
                        loading={startOidcLink.isPending}
                        onClick={() => void connectProvider(provider)}
                      >
                        {labels.connect}
                      </Button>
                    ),
                  ]}
                >
                  <List.Item.Meta
                    avatar={<ProviderAvatar provider={provider} />}
                    title={
                      <Space wrap>
                        <span>{oidcProviderName(provider)}</span>
                        <Tag color={provider.linked ? 'green' : 'default'}>
                          {provider.linked ? t('portal.profile.loginMethods.bound') : t('portal.profile.loginMethods.unbound')}
                        </Tag>
                      </Space>
                    }
                    description={
                      provider.linked
                        ? provider.provider_email || t('portal.profile.loginMethods.oidcBoundText', { provider: oidcProviderName(provider) })
                        : t('portal.profile.loginMethods.oidcUnboundText', { provider: oidcProviderName(provider) })
                    }
                  />
                </List.Item>
              )}
            />
          </Card>
        </Space>
      )}
    </>
  )
}

function AccountSummary({ profile }: { profile: UserProfile }) {
  const { t } = useTranslation()
  return (
    <Card title={t('portal.profile.accountInfo')}>
      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <Typography.Text type="secondary">{t('portal.profile.column.userId')}</Typography.Text>
          <div>
            <Typography.Text code>#{profile.id}</Typography.Text>
          </div>
        </Col>
        <Col xs={24} md={12}>
          <Typography.Text type="secondary">{t('portal.profile.column.email')}</Typography.Text>
          <div>
            <Space wrap>
              <span>{profile.email || t('portal.profile.noEmail')}</span>
              {profile.email ? (
                <Tag color={profile.email_verified ? 'green' : 'gold'}>
                  {profile.email_verified ? t('portal.profile.verified') : t('portal.profile.unverified')}
                </Tag>
              ) : null}
            </Space>
          </div>
        </Col>
        <Col xs={24} md={12}>
          <Typography.Text type="secondary">{t('portal.profile.column.status')}</Typography.Text>
          <div>
            <Tag color={profile.status === 'active' ? 'green' : 'red'}>
              {profile.status === 'active' ? t('portal.profile.status.active') : t('portal.profile.status.suspended')}
            </Tag>
          </div>
        </Col>
        <Col xs={24} md={12}>
          <Typography.Text type="secondary">{t('portal.profile.column.createdAt')}</Typography.Text>
          <div>{new Date(profile.created_at).toLocaleDateString()}</div>
        </Col>
      </Row>
    </Card>
  )
}

function ProviderAvatar({ provider }: { provider: OIDCProviderLink }) {
  const icon = safeIconUrl(oidcProviderIcon(provider))
  if (icon) return <Avatar src={icon} />
  return <Avatar icon={<GlobalOutlined />} />
}

export default Profile
