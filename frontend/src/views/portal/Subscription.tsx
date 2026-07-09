import {
  CheckOutlined,
  CopyOutlined,
  DownloadOutlined,
  LinkOutlined,
  QrcodeOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { Alert, App, Button, Card, Col, Input, message, Row, Skeleton, Space, Typography } from 'antd'
import QRCode from 'qrcode'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { EmptyState, PageHeader } from '@/components/common'
import { useBranding } from '@/hooks/queries/branding'
import { usePortalOrdersList } from '@/hooks/queries/portal/billing'
import { useProfile, useRotateSubId } from '@/hooks/queries/portal/profile'
import { useOwnTraffic } from '@/hooks/queries/portal/traffic'
import { copyText } from '@/utils/clipboard'
import { formatError } from '@/utils/format'
import { ImportButtons } from './_shared/ImportButtons'
import { hasAnyCompletedOrder, useProvisioningStatus } from './_shared/provisioning'
import {
  subscriptionFormats,
  subscriptionUrl,
  type SubscriptionFormatInfo,
  type SubscriptionFormatKey,
} from './_shared/subscriptionFormats'

function useSubscriptionQr(url: string, activeFormat: SubscriptionFormatInfo | undefined) {
  const token = useRef(0)
  const [qrDataUrl, setQrDataUrl] = useState('')

  useEffect(() => {
    const currentToken = token.current + 1
    token.current = currentToken
    setQrDataUrl('')

    if (!url || activeFormat?.downloadOnly) return

    void QRCode.toDataURL(url, {
      width: 260,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#0c0e12', light: '#ffffff' },
    })
      .then((nextUrl) => {
        if (token.current === currentToken) setQrDataUrl(nextUrl)
      })
      .catch(() => {
        if (token.current === currentToken) setQrDataUrl('')
      })
  }, [activeFormat?.downloadOnly, url])

  return qrDataUrl
}

export function Subscription() {
  const { t } = useTranslation()
  // App.useApp() so confirm dialogs inherit the active (dark/light)
  // theme — static Modal.confirm renders outside the ConfigProvider.
  const { modal } = App.useApp()
  const navigate = useNavigate()
  const [messageApi, contextHolder] = message.useMessage()
  const [activeKey, setActiveKey] = useState<SubscriptionFormatKey>('base64')
  const profile = useProfile()
  const branding = useBranding()
  const traffic = useOwnTraffic()
  const orders = usePortalOrdersList()
  const rotateSubId = useRotateSubId()
  const formats = useMemo(() => subscriptionFormats(t), [t])
  const activeFormat = formats.find((format) => format.key === activeKey)
  const subscriptionBaseURL = branding.data?.subscription_public_base_url || window.location.origin
  const url = profile.data ? subscriptionUrl(subscriptionBaseURL, profile.data.sub_id, activeKey) : ''
  const qrDataUrl = useSubscriptionQr(url, activeFormat)
  const loading = profile.isLoading || traffic.isLoading || orders.isLoading
  const error = profile.error ?? traffic.error ?? orders.error
  const clients = traffic.data ?? []
  const provisioning = useProvisioningStatus({
    clients,
    orders: orders.data,
    ordersFetching: orders.isFetching,
    trafficFetching: traffic.isFetching,
    refetchOrders: orders.refetch,
    refetchTraffic: traffic.refetch,
  })

  async function copyUrl() {
    if (!url || activeFormat?.downloadOnly) return
    // copyText never throws — clipboard API is unavailable on plain
    // http origins, where the old call silently failed with no toast.
    if (await copyText(url)) {
      void messageApi.success(t('portal.subscription.copyOk'))
    } else {
      void messageApi.error(t('portal.subscription.copyFailed'))
    }
  }

  function rotate() {
    modal.confirm({
      title: t('portal.subscription.regenerateTitle'),
      content: t('portal.subscription.regenerateConfirm'),
      okText: t('portal.subscription.regenerate'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await rotateSubId.mutateAsync()
          void messageApi.success(t('portal.subscription.regenerateOk'))
        } catch (e) {
          void messageApi.error(formatError(e, t('portal.subscription.regenerateFailed')))
          throw e
        }
      },
    })
  }

  return (
    <>
      {contextHolder}
      <PageHeader
        title={t('portal.subscription.title')}
        subtitle={t('portal.subscription.subtitle')}
        actions={
          <Button icon={<ReloadOutlined />} loading={rotateSubId.isPending} onClick={rotate}>
            {rotateSubId.isPending ? t('portal.subscription.regenerating') : t('portal.subscription.regenerate')}
          </Button>
        }
      />

      {error ? (
        <Alert
          showIcon
          type="error"
          style={{ marginBottom: 16 }}
          message={formatError(error, t('portal.subscription.loadFailed'))}
        />
      ) : null}

      {loading ? (
        <Skeleton active />
      ) : !profile.data ? (
        <Card>
          <EmptyState
            title={t('portal.subscription.loadFailed')}
            description={t('portal.subscription.profileMissingDescription')}
            actionLabel={t('common.refresh')}
            onAction={() => void profile.refetch()}
          />
        </Card>
      ) : provisioning.isProvisioning ? (
        <Card>
          <EmptyState
            title={t('portal.subscription.provisioningTitle')}
            description={t('portal.subscription.provisioningDescription')}
            actionLabel={provisioning.isRefreshingProvisioning ? t('portal.subscription.provisioningRefreshing') : t('portal.subscription.provisioningRefresh')}
            onAction={() => {
              void Promise.all([traffic.refetch(), orders.refetch()])
            }}
          />
        </Card>
      ) : clients.length === 0 && hasAnyCompletedOrder(orders.data ?? []) ? (
        // Paid but nothing provisioned and the fast-poll window is
        // over — do NOT tell a paying user to "buy a plan first".
        <Card>
          <EmptyState
            title={t('portal.subscription.stalledTitle')}
            description={t('portal.subscription.stalledDescription')}
            actionLabel={t('portal.subscription.provisioningRefresh')}
            onAction={() => {
              void Promise.all([traffic.refetch(), orders.refetch()])
            }}
          />
        </Card>
      ) : clients.length === 0 ? (
        <Card>
          <EmptyState
            title={t('portal.subscription.empty')}
            description={t('portal.subscription.emptyDescription')}
            actionLabel={t('portal.subscription.seePlans')}
            onAction={() => navigate('/portal/plans')}
          />
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={16}>
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Card>
                <Typography.Title level={4} style={{ marginTop: 0 }}>
                  {t('portal.subscription.formats.title')}
                </Typography.Title>
                <Typography.Paragraph type="secondary">
                  {t('portal.subscription.formats.hint')}
                </Typography.Paragraph>
                <div className="portal-fmt-grid">
                  {formats.map((format) => (
                    <button
                      className="portal-fmt"
                      data-active={activeKey === format.key ? 'true' : 'false'}
                      key={format.key}
                      onClick={() => setActiveKey(format.key)}
                      type="button"
                    >
                      <span className="portal-fmt-name">
                        {format.label}
                        {activeKey === format.key ? (
                          <CheckOutlined aria-label={t('portal.subscription.selected')} className="portal-fmt-check" />
                        ) : null}
                      </span>
                      <span className="portal-fmt-desc">{format.hint}</span>
                      <span className="portal-fmt-apps">{format.apps}</span>
                    </button>
                  ))}
                </div>
              </Card>

              <Card>
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Typography.Title level={4} style={{ margin: 0 }}>
                      {activeFormat?.downloadOnly ? t('portal.subscription.downloadLink') : t('portal.subscription.urlTitle')}
                    </Typography.Title>
                    {activeFormat?.downloadOnly ? (
                      // `download` is ignored cross-origin (browser
                      // navigates instead) — only claim it when the
                      // sub endpoint shares our origin.
                      <Button
                        type="primary"
                        icon={<DownloadOutlined />}
                        href={url}
                        download={subscriptionBaseURL === window.location.origin ? 'subscription-wireguard.zip' : undefined}
                      >
                        {t('portal.subscription.downloadFile')}
                      </Button>
                    ) : (
                      <Button type="primary" icon={<CopyOutlined />} onClick={() => void copyUrl()}>
                        {t('common.copy')}
                      </Button>
                    )}
                  </Space>
                  <Input
                    aria-label={t('portal.subscription.urlTitle')}
                    className="portal-url-bar"
                    prefix={<LinkOutlined />}
                    readOnly
                    value={url}
                  />
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {t('portal.subscription.rotateNote')}
                  </Typography.Text>
                </Space>
              </Card>

              <Card>
                <Typography.Title level={4} style={{ marginTop: 0 }}>
                  {t('portal.subscription.importTitle')}
                </Typography.Title>
                <Typography.Paragraph type="secondary">
                  {t('portal.subscription.importHint')}
                </Typography.Paragraph>
                <ImportButtons
                  baseUrl={subscriptionBaseURL}
                  subId={profile.data.sub_id}
                  name={branding.data?.title || 'Subscription'}
                />
              </Card>

              <Card>
                <Typography.Title level={4} style={{ marginTop: 0 }}>
                  {t('portal.subscription.howToTitle')}
                </Typography.Title>
                <ol className="portal-howto">
                  <li>{t('portal.subscription.howTo1')}</li>
                  <li>{t('portal.subscription.howTo2')}</li>
                  <li>{t('portal.subscription.howTo3')}</li>
                  <li>{t('portal.subscription.howTo4')}</li>
                </ol>
              </Card>
            </Space>
          </Col>

          <Col xs={24} lg={8}>
            <Card>
              {activeFormat?.downloadOnly ? (
                <Space direction="vertical" size={12} align="center" style={{ width: '100%', textAlign: 'center' }}>
                  <DownloadOutlined style={{ fontSize: 64, color: '#8c8c8c' }} />
                  <Typography.Title level={4}>{t('portal.subscription.downloadOnlyTitle')}</Typography.Title>
                  <Typography.Text type="secondary">{t('portal.subscription.downloadOnlyHint')}</Typography.Text>
                  <Typography.Text type="secondary">
                    {t('portal.subscription.formatLabel', { label: activeFormat.label })}
                  </Typography.Text>
                </Space>
              ) : (
                <Space direction="vertical" size={12} align="center" style={{ width: '100%', textAlign: 'center' }}>
                  <Typography.Title level={4} style={{ alignSelf: 'flex-start', marginTop: 0 }}>
                    {t('portal.subscription.qrTitle')}
                  </Typography.Title>
                  <Typography.Text type="secondary" style={{ alignSelf: 'flex-start' }}>
                    {t('portal.subscription.qrHint')}
                  </Typography.Text>
                  <div aria-label={t('portal.subscription.qrImageAlt')} className="portal-qr-box">
                    {qrDataUrl ? (
                      <img alt={t('portal.subscription.qrImageAlt')} src={qrDataUrl} style={{ height: '100%', width: '100%' }} />
                    ) : (
                      <Space direction="vertical" align="center">
                        <QrcodeOutlined />
                        <Typography.Text type="secondary">{t('portal.subscription.generating')}</Typography.Text>
                      </Space>
                    )}
                  </div>
                  <Typography.Text type="secondary">
                    {t('portal.subscription.formatLabel', { label: activeFormat?.label ?? '' })}
                  </Typography.Text>
                </Space>
              )}
            </Card>
          </Col>
        </Row>
      )}
    </>
  )
}

export default Subscription
