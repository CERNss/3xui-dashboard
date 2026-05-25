import { CheckOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Modal, Radio, Space, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import type { Plan, PaymentMethod, Order } from '@/api/portal/billing'
import { AlipayPayModal } from '@/components/portal'
import { PageHeader } from '@/components/common'
import { usePaymentMethods, usePortalPlansList, usePurchasePlan, usePurchaseViaPayment } from '@/hooks/queries/portal/billing'
import { useProfile } from '@/hooks/queries/portal/profile'
import { formatError } from '@/utils/format'
import { formatTraffic, formatYuan, paymentMethodLabel } from './_shared/billing'

function uuid(): string {
  if (crypto.randomUUID) return crypto.randomUUID()
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

function confirmPurchase(title: string, content: string, okText: string): Promise<boolean> {
  return new Promise((resolve) => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText: 'Cancel',
      onCancel: () => resolve(false),
      onOk: () => resolve(true),
    })
  })
}

export default function Plans() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const plansQuery = usePortalPlansList()
  const profileQuery = useProfile()
  const methodsQuery = usePaymentMethods()
  const purchase = usePurchasePlan()
  const purchaseViaPayment = usePurchaseViaPayment()
  const [selectedMethod, setSelectedMethod] = useState<PaymentMethod>('balance')
  const [buyingPlanId, setBuyingPlanId] = useState<number | null>(null)
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [alipayOrder, setAlipayOrder] = useState<Order | null>(null)

  const methods = useMemo(() => {
    const fetched = methodsQuery.data?.length ? methodsQuery.data : ['balance' as PaymentMethod]
    return fetched.includes(selectedMethod) ? fetched : fetched
  }, [methodsQuery.data, selectedMethod])

  const effectiveMethod = methods.includes(selectedMethod) ? selectedMethod : methods[0]
  const enabledPlans = useMemo(
    () => [...(plansQuery.data ?? [])].filter((plan) => plan.enabled).sort((a, b) => a.price_cents - b.price_cents),
    [plansQuery.data],
  )
  const loading = plansQuery.isLoading || profileQuery.isLoading || methodsQuery.isLoading
  const error = plansQuery.error ?? profileQuery.error ?? methodsQuery.error ?? purchase.error ?? purchaseViaPayment.error

  const methodLabels: Record<PaymentMethod, string> = {
    alipay: t('portal.plans.method.alipay', { defaultValue: 'Alipay' }),
    balance: t('portal.plans.method.balance', { defaultValue: 'Balance' }),
    stripe: t('portal.plans.method.stripe', { defaultValue: 'Stripe' }),
  }

  const canAfford = (plan: Plan) => (profileQuery.data?.balance_cents ?? 0) >= plan.price_cents
  const canBuy = (plan: Plan) => effectiveMethod !== 'balance' || canAfford(plan)

  const buy = async (plan: Plan) => {
    const methodLabel = paymentMethodLabel(effectiveMethod, methodLabels)
    const amount = formatYuan(plan.price_cents)
    const ok = await confirmPurchase(
      t('portal.plans.confirmTitle', { defaultValue: 'Buy "{name}"', name: plan.name }),
      effectiveMethod === 'balance'
        ? t('portal.plans.confirmBalanceMsg', { defaultValue: 'Pay {amount} from balance.', amount })
        : t('portal.plans.confirmPayMsg', { defaultValue: 'Pay {amount} with {method}.', amount, method: methodLabel }),
      t('portal.plans.confirmPayBtn', { defaultValue: 'Pay {amount} with {method}', amount, method: methodLabel }),
    )
    if (!ok) return

    setBuyingPlanId(plan.id)
    setFlash(null)
    try {
      const input = { plan_id: plan.id, idempotency_key: uuid() }
      if (effectiveMethod === 'alipay') {
        const order = await purchaseViaPayment.mutateAsync({ provider: 'alipay', input })
        setAlipayOrder(order)
        return
      }
      if (effectiveMethod === 'stripe') {
        const order = await purchaseViaPayment.mutateAsync({ provider: 'stripe', input })
        if (order.payment_target_url) {
          window.location.assign(order.payment_target_url)
          return
        }
        setFlash({ type: 'error', text: t('portal.plans.stripeNoUrl', { defaultValue: 'Stripe did not return a payment URL' }) })
        return
      }

      const order = await purchase.mutateAsync(input)
      setFlash({
        type: 'success',
        text: t('portal.plans.orderCreated', { defaultValue: 'Order #{id} created', id: order.id }),
      })
      await profileQuery.refetch()
      window.setTimeout(() => navigate('/portal/orders'), 800)
    } catch (err) {
      setFlash({
        type: 'error',
        text: formatError(err, t('portal.plans.purchaseFailed', { defaultValue: 'Purchase failed' })),
      })
    } finally {
      setBuyingPlanId(null)
    }
  }

  const handleAlipaySuccess = (order: Order) => {
    setFlash({
      type: 'success',
      text: t('portal.plans.orderPaid', { defaultValue: 'Order #{id} paid', id: order.id }),
    })
    void profileQuery.refetch()
    window.setTimeout(() => {
      setAlipayOrder(null)
      navigate('/portal/orders')
    }, 1000)
  }

  return (
    <div>
      <PageHeader
        title={t('portal.plans.title', { defaultValue: 'Plans' })}
        subtitle={t('portal.plans.subtitle', { defaultValue: 'Buy a plan and provision service instantly' })}
        actions={
          profileQuery.data ? (
            <Space direction="vertical" size={0} style={{ textAlign: 'right' }}>
              <Typography.Text type="secondary">
                {t('portal.plans.currentBalance', { defaultValue: 'Current balance' })}
              </Typography.Text>
              <Typography.Text strong>{formatYuan(profileQuery.data.balance_cents)}</Typography.Text>
            </Space>
          ) : null
        }
      />

      {error ? (
        <Alert message={formatError(error, t('portal.plans.loadFailed', { defaultValue: 'Load failed' }))} showIcon style={{ marginBottom: 16 }} type="error" />
      ) : null}
      {flash ? <Alert message={flash.text} showIcon style={{ marginBottom: 16 }} type={flash.type} /> : null}

      {methods.length > 1 ? (
        <Card size="small" style={{ marginBottom: 16 }}>
          <Space direction="vertical" size={8}>
            <Typography.Text strong>{t('portal.plans.methodPickerTitle', { defaultValue: 'Payment method' })}</Typography.Text>
            <Typography.Text type="secondary">
              {t('portal.plans.methodPickerHint', { defaultValue: 'Choose how to pay for this order' })}
            </Typography.Text>
            <Radio.Group
              optionType="button"
              options={methods.map((method) => ({ label: paymentMethodLabel(method, methodLabels), value: method }))}
              value={effectiveMethod}
              onChange={(event) => setSelectedMethod(event.target.value as PaymentMethod)}
            />
          </Space>
        </Card>
      ) : null}

      {enabledPlans.length > 0 || loading ? (
        <div
          aria-busy={loading}
          style={{ display: 'grid', gap: 16, gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))' }}
        >
          {enabledPlans.map((plan) => (
            <Card
              key={plan.id}
              actions={[
                <Button
                  key="buy"
                  block
                  disabled={!canBuy(plan)}
                  loading={buyingPlanId === plan.id}
                  type="primary"
                  onClick={() => void buy(plan)}
                >
                  {effectiveMethod === 'balance' && !canAfford(plan)
                    ? t('portal.plans.balanceInsufficient', { defaultValue: 'Insufficient balance' })
                    : t('portal.plans.buyNow', { defaultValue: 'Buy now' })}
                </Button>,
              ]}
            >
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Typography.Title level={4} style={{ margin: 0 }}>
                  {plan.name}
                </Typography.Title>
                {plan.description ? <Typography.Text type="secondary">{plan.description}</Typography.Text> : null}
                <Typography.Title level={2} style={{ margin: 0 }}>
                  {formatYuan(plan.price_cents)}
                </Typography.Title>
                <Space direction="vertical" size={6}>
                  <Typography.Text>
                    <CheckOutlined />{' '}
                    {plan.traffic_limit_bytes === 0
                      ? t('portal.plans.unlimitedTraffic', { defaultValue: 'Unlimited traffic' })
                      : `${formatTraffic(plan.traffic_limit_bytes)} ${t('portal.plans.trafficLabel', { defaultValue: 'traffic' })}`}
                  </Typography.Text>
                  <Typography.Text>
                    <CheckOutlined /> {t('portal.plans.planDays', { defaultValue: '{days} days', days: plan.duration_days })}
                  </Typography.Text>
                  {plan.ip_limit ? (
                    <Typography.Text>
                      <CheckOutlined /> {t('portal.plans.ipLimitText', { defaultValue: 'Up to {n} IPs', n: plan.ip_limit })}
                    </Typography.Text>
                  ) : null}
                </Space>
              </Space>
            </Card>
          ))}
        </div>
      ) : (
        <Card>
          <Typography.Text>{t('portal.plans.empty', { defaultValue: 'No plans available' })}</Typography.Text>
        </Card>
      )}

      <AlipayPayModal
        open={Boolean(alipayOrder)}
        order={alipayOrder}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) setAlipayOrder(null)
        }}
        onSuccess={handleAlipaySuccess}
      />
    </div>
  )
}
