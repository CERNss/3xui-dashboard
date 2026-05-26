import {
  CheckOutlined,
  CopyOutlined,
  DownloadOutlined,
  LinkOutlined,
  QrcodeOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Col, Input, message, Modal, Row, Skeleton, Space, Typography } from 'antd'
import QRCode from 'qrcode'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { EmptyState, PageHeader } from '@/components/common'
import { useProfile, useRotateSubId } from '@/hooks/queries/portal/profile'
import { useOwnTraffic } from '@/hooks/queries/portal/traffic'
import { formatError } from '@/utils/format'
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
  const [messageApi, contextHolder] = message.useMessage()
  const [activeKey, setActiveKey] = useState<SubscriptionFormatKey>('base64')
  const profile = useProfile()
  const traffic = useOwnTraffic()
  const rotateSubId = useRotateSubId()
  const formats = useMemo(() => subscriptionFormats(t), [t])
  const activeFormat = formats.find((format) => format.key === activeKey)
  const url = profile.data ? subscriptionUrl(window.location.origin, profile.data.sub_id, activeKey) : ''
  const qrDataUrl = useSubscriptionQr(url, activeFormat)
  const loading = profile.isLoading || traffic.isLoading
  const error = profile.error ?? traffic.error
  const clients = traffic.data ?? []

  async function copyUrl() {
    if (!url || activeFormat?.downloadOnly) return
    await navigator.clipboard.writeText(url)
    void messageApi.success(t('portal.subscription.copyOk'))
  }

  function rotate() {
    Modal.confirm({
      title: t('portal.subscription.regenerateTitle'),
      content: t('portal.subscription.regenerateConfirm'),
      okText: t('portal.subscription.regenerate'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await rotateSubId.mutateAsync()
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
      ) : !profile.data ? null : clients.length === 0 ? (
        <Card>
          <EmptyState
            title={t('portal.subscription.empty')}
            description={t('portal.subscription.emptyDescription')}
            actionLabel={t('portal.subscription.seePlans')}
            onAction={() => undefined}
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
                <Row gutter={[8, 8]}>
                  {formats.map((format) => (
                    <Col xs={12} md={8} key={format.key}>
                      <Button
                        block
                        type={activeKey === format.key ? 'primary' : 'default'}
                        style={{ height: 'auto', minHeight: 88, padding: 12, textAlign: 'left', whiteSpace: 'normal' }}
                        onClick={() => setActiveKey(format.key)}
                      >
                        <Space direction="vertical" size={2} style={{ alignItems: 'flex-start', width: '100%' }}>
                          <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                            <Typography.Text strong>{format.label}</Typography.Text>
                            {activeKey === format.key ? <CheckOutlined aria-label={t('portal.subscription.selected')} /> : null}
                          </Space>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {format.hint}
                          </Typography.Text>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {format.apps}
                          </Typography.Text>
                        </Space>
                      </Button>
                    </Col>
                  ))}
                </Row>
              </Card>

              <Card>
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Typography.Title level={4} style={{ margin: 0 }}>
                      {activeFormat?.downloadOnly ? t('portal.subscription.downloadLink') : t('portal.subscription.urlTitle')}
                    </Typography.Title>
                    {activeFormat?.downloadOnly ? (
                      <Button type="primary" icon={<DownloadOutlined />} href={url} download>
                        {t('portal.subscription.downloadFile')}
                      </Button>
                    ) : (
                      <Button type="primary" icon={<CopyOutlined />} onClick={() => void copyUrl()}>
                        {t('common.copy')}
                      </Button>
                    )}
                  </Space>
                  <Input readOnly value={url} prefix={<LinkOutlined />} aria-label={t('portal.subscription.urlTitle')} />
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {t('portal.subscription.rotateNote')}
                  </Typography.Text>
                </Space>
              </Card>

              <Card>
                <Typography.Title level={4} style={{ marginTop: 0 }}>
                  {t('portal.subscription.howToTitle')}
                </Typography.Title>
                <ol style={{ marginBottom: 0, paddingInlineStart: 20 }}>
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
                  <div
                    aria-label={t('portal.subscription.qrImageAlt')}
                    style={{
                      alignItems: 'center',
                      aspectRatio: '1 / 1',
                      border: '1px solid #eaecef',
                      borderRadius: 8,
                      display: 'flex',
                      justifyContent: 'center',
                      padding: 12,
                      width: 'min(100%, 300px)',
                    }}
                  >
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
