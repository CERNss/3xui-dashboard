import { CheckOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Modal, Radio, Space, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import type { Plan, PaymentMethod, Order } from '@/api/portal/billing'
import { AlipayPayModal } from '@/components/portal'
import { ConfigListPage, EmptyState } from '@/components/common'
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

function confirmPurchase(title: string, content: string, okText: string, cancelText: string): Promise<boolean> {
  return new Promise((resolve) => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText,
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
    alipay: t('portal.plans.method.alipay'),
    balance: t('portal.plans.method.balance'),
    stripe: t('portal.plans.method.stripe'),
  }

  const canAfford = (plan: Plan) => (profileQuery.data?.balance_cents ?? 0) >= plan.price_cents
  const canBuy = (plan: Plan) => effectiveMethod !== 'balance' || canAfford(plan)

  const buy = async (plan: Plan) => {
    const methodLabel = paymentMethodLabel(effectiveMethod, methodLabels)
    const amount = formatYuan(plan.price_cents)
    const ok = await confirmPurchase(
      t('portal.plans.confirmTitle', { name: plan.name }),
      effectiveMethod === 'balance'
        ? t('portal.plans.confirmBalanceMsg', { amount })
        : t('portal.plans.confirmPayMsg', { amount, method: methodLabel }),
      t('portal.plans.confirmPayBtn', { amount, method: methodLabel }),
      t('common.cancel'),
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
        setFlash({ type: 'error', text: t('portal.plans.stripeNoUrl') })
        return
      }

      const order = await purchase.mutateAsync(input)
      setFlash({
        type: 'success',
        text: t('portal.plans.orderCreated', { id: order.id }),
      })
      await profileQuery.refetch()
      window.setTimeout(() => navigate('/portal/orders'), 800)
    } catch (err) {
      setFlash({
        type: 'error',
        text: formatError(err, t('portal.plans.purchaseFailed')),
      })
    } finally {
      setBuyingPlanId(null)
    }
  }

  const handleAlipaySuccess = (order: Order) => {
    setFlash({
      type: 'success',
      text: t('portal.plans.orderPaid', { id: order.id }),
    })
    void profileQuery.refetch()
    window.setTimeout(() => {
      setAlipayOrder(null)
      navigate('/portal/orders')
    }, 1000)
  }

  return (
    <div>
      <ConfigListPage<Plan>
        title={t('portal.plans.title')}
        subtitle={t('portal.plans.subtitle')}
        actions={
          profileQuery.data ? (
            <Space direction="vertical" size={0} style={{ textAlign: 'right' }}>
              <Typography.Text type="secondary">
                {t('portal.plans.currentBalance')}
              </Typography.Text>
              <Typography.Text strong>{formatYuan(profileQuery.data.balance_cents)}</Typography.Text>
            </Space>
          ) : null
        }
        alerts={error || flash ? (
          <>
            {error ? <Alert message={formatError(error, t('portal.plans.loadFailed'))} showIcon type="error" /> : null}
            {flash ? <Alert message={flash.text} showIcon style={{ marginTop: error ? 16 : 0 }} type={flash.type} /> : null}
          </>
        ) : null}
        dataSource={enabledPlans}
        loading={loading}
        filters={
          methods.length > 1 ? (
            <Space wrap>
              <Typography.Text strong>{t('portal.plans.methodPickerTitle')}</Typography.Text>
              <Radio.Group
                optionType="button"
                options={methods.map((method) => ({ label: paymentMethodLabel(method, methodLabels), value: method }))}
                value={effectiveMethod}
                onChange={(event) => setSelectedMethod(event.target.value as PaymentMethod)}
              />
            </Space>
          ) : undefined
        }
        listClassName="config-list-page-card-grid"
        listContent={
          enabledPlans.length > 0 || loading ? (
            <>
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
                        ? t('portal.plans.balanceInsufficient')
                        : t('portal.plans.buyNow')}
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
                          ? t('portal.plans.unlimitedTraffic')
                          : `${formatTraffic(plan.traffic_limit_bytes)} ${t('portal.plans.trafficLabel')}`}
                      </Typography.Text>
                      <Typography.Text>
                        <CheckOutlined /> {t('portal.plans.planDays', { days: plan.duration_days })}
                      </Typography.Text>
                      {plan.ip_limit ? (
                        <Typography.Text>
                          <CheckOutlined /> {t('portal.plans.ipLimitText', { n: plan.ip_limit })}
                        </Typography.Text>
                      ) : null}
                    </Space>
                  </Space>
                </Card>
              ))}
            </>
          ) : (
            <EmptyState title={t('portal.plans.empty')} description={t('portal.plans.emptyDescription')} />
          )
        }
      />

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
