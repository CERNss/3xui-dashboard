import { Alert, Button, Card, Space, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import type { Order, PaymentMethod, Plan } from '@/api/portal/billing'
import { portalBillingApi } from '@/api/portal/billing'
import { AlipayPayModal } from '@/components/portal'
import { ConfigListPage } from '@/components/common'
import { usePortalOrdersList, usePortalPlansList } from '@/hooks/queries/portal/billing'
import { useProfile } from '@/hooks/queries/portal/profile'
import { formatError } from '@/utils/format'
import { canContinuePayment, formatYuan, OrderStatusTag, paymentMethodLabel } from './_shared/billing'

function planName(plans: Plan[], planId: number, fallback: string): string {
  return plans.find((plan) => plan.id === planId)?.name ?? fallback
}

export default function Orders() {
  const { t } = useTranslation()
  const ordersQuery = usePortalOrdersList()
  const plansQuery = usePortalPlansList()
  const profileQuery = useProfile()
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [refreshingOrderId, setRefreshingOrderId] = useState<number | null>(null)
  const [alipayOrder, setAlipayOrder] = useState<Order | null>(null)

  const orders = useMemo(
    () => [...(ordersQuery.data ?? [])].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()),
    [ordersQuery.data],
  )
  const plans = plansQuery.data ?? []
  const loading = ordersQuery.isLoading || plansQuery.isLoading || profileQuery.isLoading
  const error = ordersQuery.error ?? plansQuery.error ?? profileQuery.error

  const methodLabels: Record<PaymentMethod, string> = {
    alipay: t('portal.orders.method.alipay'),
    balance: t('portal.orders.method.balance'),
    stripe: t('portal.orders.method.stripe'),
  }

  const statusLabel = (status: Order['status']) => {
    const key = status === 'payment_pending' ? 'paymentPending' : status === 'payment_failed' ? 'paymentFailed' : status === 'payment_expired' ? 'paymentExpired' : status
    const label = t(`portal.orders.status.${key}` as const, { defaultValue: '' })
    return label || t('portal.orders.status.unknown', { status })
  }

  const refreshOrder = async (order: Order) => {
    setRefreshingOrderId(order.id)
    setFlash(null)
    try {
      const fresh = await portalBillingApi.getOrder(order.id)
      await ordersQuery.refetch()
      return fresh
    } catch (err) {
      setFlash({
        type: 'error',
        text: formatError(err, t('portal.orders.refreshFailed')),
      })
      return null
    } finally {
      setRefreshingOrderId(null)
    }
  }

  const continuePayment = async (order: Order) => {
    const fresh = await refreshOrder(order)
    if (!fresh || fresh.status !== 'payment_pending') return
    if (!fresh.payment_target_url) {
      setFlash({
        type: 'error',
        text: t('portal.orders.paymentLinkMissing'),
      })
      return
    }
    if (fresh.payment_method === 'stripe') {
      window.location.assign(fresh.payment_target_url)
      return
    }
    if (fresh.payment_method === 'alipay') {
      setAlipayOrder(fresh)
    }
  }

  const handleAlipaySuccess = (order: Order) => {
    setFlash({
      type: 'success',
      text: t('portal.orders.orderPaid', { id: order.id }),
    })
    void ordersQuery.refetch()
    window.setTimeout(() => setAlipayOrder(null), 1000)
  }

  const columns: ColumnsType<Order> = [
    {
      title: t('portal.orders.column.orderId'),
      dataIndex: 'id',
      render: (id: number) => <Typography.Text type="secondary">#{id}</Typography.Text>,
    },
    {
      title: t('portal.orders.column.plan'),
      dataIndex: 'plan_id',
      render: (planId: number) =>
        planName(plans, planId, t('portal.orders.unknownPlan', { id: planId })),
    },
    {
      title: t('portal.orders.column.amount'),
      dataIndex: 'price_cents',
      align: 'right',
      render: (value: number) => <Typography.Text strong>{formatYuan(value)}</Typography.Text>,
    },
    {
      title: t('portal.orders.column.method'),
      dataIndex: 'payment_method',
      render: (method: PaymentMethod) => paymentMethodLabel(method, methodLabels),
    },
    {
      title: t('portal.orders.column.status'),
      dataIndex: 'status',
      render: (status: Order['status']) => <OrderStatusTag label={statusLabel(status)} status={status} />,
    },
    {
      title: t('portal.orders.column.createdAt'),
      dataIndex: 'created_at',
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: t('portal.orders.column.actions'),
      key: 'actions',
      align: 'right',
      render: (_value, order) =>
        canContinuePayment(order) ? (
          <Button loading={refreshingOrderId === order.id} onClick={() => void continuePayment(order)}>
            {t('portal.orders.continuePayment')}
          </Button>
        ) : null,
    },
  ]

  return (
    <div>
      <ConfigListPage
        title={t('portal.orders.title')}
        subtitle={t('portal.orders.subtitle')}
        actions={
          profileQuery.data ? (
            <Space direction="vertical" size={0} style={{ textAlign: 'right' }}>
              <Typography.Text type="secondary">{t('portal.orders.balance')}</Typography.Text>
              <Typography.Text strong>{formatYuan(profileQuery.data.balance_cents)}</Typography.Text>
            </Space>
          ) : null
        }
        alerts={error || flash ? (
          <>
            {error ? <Alert message={formatError(error, t('portal.orders.loadFailed'))} showIcon type="error" /> : null}
            {flash ? <Alert message={flash.text} showIcon style={{ marginTop: error ? 16 : 0 }} type={flash.type} /> : null}
          </>
        ) : null}
        columns={columns}
        dataSource={orders}
        loading={loading}
        emptyContent={
          <Card>
            <Space direction="vertical">
              <Typography.Text strong>{t('portal.orders.empty')}</Typography.Text>
              <Typography.Text type="secondary">{t('portal.orders.emptyDescription')}</Typography.Text>
              <Link to="/portal/plans">{t('portal.orders.seePlans')}</Link>
            </Space>
          </Card>
        }
        mobileCard={(order) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical">
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>#{order.id}</Typography.Text>
                <OrderStatusTag label={statusLabel(order.status)} status={order.status} />
              </Space>
              <Typography.Text>
                {planName(plans, order.plan_id, t('portal.orders.unknownPlan', { id: order.plan_id }))}
              </Typography.Text>
              <Typography.Text>{formatYuan(order.price_cents)}</Typography.Text>
              <Typography.Text>{paymentMethodLabel(order.payment_method, methodLabels)}</Typography.Text>
              {canContinuePayment(order) ? (
                <Button loading={refreshingOrderId === order.id} onClick={() => void continuePayment(order)}>
                  {t('portal.orders.continuePayment')}
                </Button>
              ) : null}
            </Space>
          </Card>
        )}
        pagination={false}
        rowKey="id"
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
