import { CheckCircleTwoTone, CloseCircleTwoTone, ExclamationCircleTwoTone } from '@ant-design/icons'
import { Button, Modal, Space, Spin, Typography } from 'antd'
import QRCode from 'qrcode'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { portalBillingApi, type Order } from '@/api/portal/billing'
import { formatYuan, isPaidOrder } from '@/views/portal/_shared/billing'

type PayState = 'waiting' | 'success' | 'failed' | 'expired'

export interface AlipayPayModalProps {
  open: boolean
  order: Order | null
  onOpenChange: (open: boolean) => void
  onSuccess?: (order: Order) => void
}

function terminalStateFor(order: Order): PayState | null {
  if (isPaidOrder(order.status)) return 'success'
  if (order.status === 'payment_failed' || order.status === 'failed' || order.status === 'refunded') return 'failed'
  if (order.status === 'payment_expired') return 'expired'
  return null
}

function formatRemaining(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}

export function AlipayPayModal({ open, order, onOpenChange, onSuccess }: AlipayPayModalProps) {
  const { t } = useTranslation()
  const [state, setState] = useState<PayState>('waiting')
  const [qrDataURL, setQrDataURL] = useState('')
  const [remainingSec, setRemainingSec] = useState(0)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const countdownTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const autoCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const qrTokenRef = useRef(0)

  const clearTimers = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
    if (countdownTimerRef.current) {
      clearInterval(countdownTimerRef.current)
      countdownTimerRef.current = null
    }
    if (autoCloseTimerRef.current) {
      clearTimeout(autoCloseTimerRef.current)
      autoCloseTimerRef.current = null
    }
  }, [])

  const close = useCallback(() => {
    clearTimers()
    qrTokenRef.current += 1
    onOpenChange(false)
  }, [clearTimers, onOpenChange])

  const markTerminal = useCallback(
    (nextState: PayState, freshOrder?: Order) => {
      setState(nextState)
      clearTimers()
      if (nextState === 'success' && freshOrder) {
        onSuccess?.(freshOrder)
        autoCloseTimerRef.current = setTimeout(() => onOpenChange(false), 900)
      }
    },
    [clearTimers, onOpenChange, onSuccess],
  )

  const pollOnce = useCallback(async () => {
    if (!order) return
    try {
      const fresh = await portalBillingApi.getOrder(order.id)
      const terminal = terminalStateFor(fresh)
      if (terminal) {
        markTerminal(terminal, terminal === 'success' ? fresh : undefined)
      }
    } catch {
      // Keep polling through transient network errors.
    }
  }, [markTerminal, order])

  useEffect(() => {
    if (!open || !order?.payment_target_url) {
      clearTimers()
      setQrDataURL('')
      return undefined
    }

    clearTimers()
    setState('waiting')
    setQrDataURL('')
    const token = qrTokenRef.current + 1
    qrTokenRef.current = token
    void QRCode.toDataURL(order.payment_target_url, {
      width: 240,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#0c0e12', light: '#ffffff' },
    }).then(
      (url) => {
        if (qrTokenRef.current === token) setQrDataURL(url)
      },
      () => {
        if (qrTokenRef.current === token) setQrDataURL('')
      },
    )

    pollTimerRef.current = setInterval(() => {
      void pollOnce()
    }, 3000)
    countdownTimerRef.current = setInterval(() => {
      if (!order.payment_expires_at) return
      const diff = Math.floor((new Date(order.payment_expires_at).getTime() - Date.now()) / 1000)
      setRemainingSec(Math.max(0, diff))
      if (diff <= 0) markTerminal('expired')
    }, 1000)

    return () => {
      clearTimers()
      qrTokenRef.current += 1
    }
  }, [clearTimers, markTerminal, open, order, pollOnce])

  const title = t('alipay.title')

  return (
    <Modal
      centered
      destroyOnHidden
      footer={null}
      maskClosable={state !== 'waiting'}
      open={open}
      title={title}
      onCancel={close}
    >
      {order ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text type="secondary">
            {t('alipay.orderLine', { id: order.id, amount: formatYuan(order.price_cents) })}
          </Typography.Text>

          {state === 'waiting' ? (
            <>
              <div style={{ display: 'flex', justifyContent: 'center' }}>
                {qrDataURL ? (
                  <img alt={t('alipay.qrAlt')} height={240} src={qrDataURL} width={240} />
                ) : (
                  <div
                    aria-label={t('alipay.qrGenerating')}
                    style={{ alignItems: 'center', display: 'flex', height: 240, justifyContent: 'center', width: 240 }}
                  >
                    <Spin />
                  </div>
                )}
              </div>
              <Typography.Text style={{ display: 'block', textAlign: 'center' }} type="secondary">
                {t('alipay.scanHint')}
              </Typography.Text>
              {remainingSec > 0 ? (
                <Typography.Text style={{ display: 'block', textAlign: 'center' }} type="secondary">
                  {t('alipay.qrTimeout', { time: formatRemaining(remainingSec) })}
                </Typography.Text>
              ) : null}
              {order.payment_target_url ? (
                <Button block href={order.payment_target_url} type="primary">
                  {t('alipay.apkOpen')}
                </Button>
              ) : null}
            </>
          ) : null}

          {state === 'success' ? (
            <Space align="center" direction="vertical" style={{ width: '100%' }}>
              <CheckCircleTwoTone twoToneColor="#52c41a" style={{ fontSize: 48 }} />
              <Typography.Text strong>{t('alipay.success')}</Typography.Text>
              <Typography.Text type="secondary">
                {t('alipay.redirecting')}
              </Typography.Text>
            </Space>
          ) : null}

          {state === 'failed' ? (
            <Space align="center" direction="vertical" style={{ width: '100%' }}>
              <CloseCircleTwoTone twoToneColor="#ff4d4f" style={{ fontSize: 48 }} />
              <Typography.Text strong>{t('alipay.failed')}</Typography.Text>
              <Typography.Text type="secondary">
                {t('alipay.failedHint')}
              </Typography.Text>
              <Button onClick={close}>{t('alipay.close')}</Button>
            </Space>
          ) : null}

          {state === 'expired' ? (
            <Space align="center" direction="vertical" style={{ width: '100%' }}>
              <ExclamationCircleTwoTone twoToneColor="#faad14" style={{ fontSize: 48 }} />
              <Typography.Text strong>{t('alipay.expired')}</Typography.Text>
              <Typography.Text type="secondary">
                {t('alipay.expiredHint')}
              </Typography.Text>
              <Button onClick={close}>{t('alipay.close')}</Button>
            </Space>
          ) : null}
        </Space>
      ) : (
        <Typography.Text type="secondary">
          {t('alipay.noPendingOrder')}
        </Typography.Text>
      )}
    </Modal>
  )
}

export default AlipayPayModal
