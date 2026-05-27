import { Button, Card, Modal, Segmented, Space, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { AdminOrder, OrderStatus } from '@/api/admin/orders'
import type { AdminPlan } from '@/api/admin/plans'
import type { AdminUser } from '@/api/admin/users'
import { ConfigListPage, RefreshButton } from '@/components/common'
import { useOrdersList, useRefundOrder } from '@/hooks/queries/admin/orders'
import { usePlansList } from '@/hooks/queries/admin/plans'
import { useUsersList } from '@/hooks/queries/admin/users'

type StatusFilter = 'all' | OrderStatus

function formatMoney(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '-'
}

function canRefund(status: OrderStatus) {
  return status === 'completed' || status === 'paid'
}

function statusTag(status: OrderStatus, label: string = status) {
  const colorByStatus: Record<OrderStatus, string> = {
    completed: 'green',
    paid: 'green',
    failed: 'red',
    refunded: 'gold',
    created: 'default',
  }

  return <Tag color={colorByStatus[status]}>{label}</Tag>
}

function asMap<T extends { id: number }>(items: T[] = []) {
  return new Map(items.map((item) => [item.id, item]))
}

export default function Orders() {
  const { t } = useTranslation()
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')

  const ordersQuery = useOrdersList({ limit: 200 })
  const plansQuery = usePlansList()
  const usersQuery = useUsersList({ limit: 500 })
  const refundOrder = useRefundOrder()

  const orders = useMemo(() => ordersQuery.data?.orders ?? [], [ordersQuery.data])
  const plansById = useMemo(() => asMap<AdminPlan>(plansQuery.data ?? []), [plansQuery.data])
  const usersById = useMemo(() => asMap<AdminUser>(usersQuery.data?.users ?? []), [usersQuery.data])

  const filteredOrders = useMemo(() => {
    const visible = statusFilter === 'all' ? orders : orders.filter((order) => order.status === statusFilter)
    return [...visible].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  }, [orders, statusFilter])

  const stats = useMemo(() => {
    const paidOrders = orders.filter((order) => order.status === 'completed' || order.status === 'paid')
    return {
      total: orders.length,
      completed: paidOrders.length,
      revenue: paidOrders.reduce((sum, order) => sum + order.price_cents, 0),
    }
  }, [orders])

  const loading = ordersQuery.isLoading || plansQuery.isLoading || usersQuery.isLoading
  const refreshing = ordersQuery.isFetching || plansQuery.isFetching || usersQuery.isFetching

  const planName = (id: number) => plansById.get(id)?.name ?? t('admin.stats.unknownPlan', { id })
  const userEmail = (id: number) => usersById.get(id)?.email ?? t('admin.stats.unknownUser', { id })
  const statusLabel = (status: OrderStatus) => t(`admin.orders.status.${status}`)
  const statusTagLabel = (status: OrderStatus) => t('admin.orders.statusTag', { status: statusLabel(status) })
  const statusFilters: Array<{ label: string; value: StatusFilter }> = [
    { label: t('admin.orders.filterAll'), value: 'all' },
    { label: statusLabel('completed'), value: 'completed' },
    { label: t('admin.stats.orderStatus.paid'), value: 'paid' },
    { label: statusLabel('failed'), value: 'failed' },
    { label: statusLabel('refunded'), value: 'refunded' },
    { label: statusLabel('created'), value: 'created' },
  ]

  const refresh = () => {
    ordersQuery.refetch()
    plansQuery.refetch()
    usersQuery.refetch()
  }

  const confirmRefund = (order: AdminOrder) => {
    Modal.confirm({
      title: t('admin.orders.refundTitle'),
      content: t('admin.orders.refundConfirmMsg', {
        amount: formatMoney(order.price_cents),
        email: userEmail(order.user_id),
        id: order.id,
        plan: planName(order.plan_id),
      }),
      okText: t('admin.orders.refund'),
      okButtonProps: { danger: true },
      onOk: async () => {
        await refundOrder.mutateAsync({ id: order.id, reason: 'admin manual refund' })
        message.success(t('admin.orders.status.refunded'))
      },
    })
  }

  const columns: ColumnsType<AdminOrder> = [
    {
      title: t('admin.orders.column.orderId'),
      dataIndex: 'id',
      align: 'center',
      width: 96,
      render: (id: number) => <Typography.Text code>#{id}</Typography.Text>,
    },
    {
      title: t('admin.orders.column.user'),
      dataIndex: 'user_id',
      render: (id: number) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{userEmail(id)}</Typography.Text>
          <Typography.Text type="secondary">{t('admin.stats.unknownUser', { id })}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.orders.column.plan'),
      dataIndex: 'plan_id',
      render: (id: number) => planName(id),
    },
    {
      title: t('admin.orders.column.amount'),
      dataIndex: 'price_cents',
      align: 'right',
      className: 'table-cell-number',
      width: 120,
      render: formatMoney,
    },
    {
      title: t('admin.orders.column.status'),
      dataIndex: 'status',
      align: 'center',
      width: 150,
      render: (status: OrderStatus, order) => (
        <Space direction="vertical" size={2}>
          {statusTag(status, statusTagLabel(status))}
          {order.error_message ? <Typography.Text type="danger">{order.error_message}</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: t('admin.orders.column.created'),
      dataIndex: 'created_at',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 180,
      sorter: (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      defaultSortOrder: 'descend',
      render: formatDate,
    },
    {
      title: t('admin.orders.column.completed'),
      dataIndex: 'completed_at',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 180,
      render: formatDate,
    },
    {
      title: t('admin.users.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 120,
      render: (_, order) =>
        canRefund(order.status) ? (
          <Button
            danger
            size="small"
            loading={refundOrder.isPending && refundOrder.variables?.id === order.id}
            onClick={() => confirmRefund(order)}
          >
            {t('admin.orders.refund')}
          </Button>
        ) : null,
    },
  ]

  return (
    <section>
      <ConfigListPage
        title={t('admin.orders.title')}
        subtitle={t('admin.orders.subtitle')}
        actions={<RefreshButton loading={refreshing} onClick={refresh} label={t('admin.orders.reload')} />}
        stats={
          <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))' }}>
            <Card size="small">
              <Typography.Text type="secondary">{t('admin.orders.kpiTotal')}</Typography.Text>
              <Typography.Title level={3} style={{ margin: 0 }}>
                {stats.total}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">{t('admin.orders.kpiCompleted')}</Typography.Text>
              <Typography.Title level={3} style={{ margin: 0 }}>
                {stats.completed}
              </Typography.Title>
            </Card>
            <Card size="small">
              <Typography.Text type="secondary">{t('admin.orders.kpiRevenue')}</Typography.Text>
              <Typography.Title level={3} style={{ margin: 0 }}>
                {formatMoney(stats.revenue)}
              </Typography.Title>
            </Card>
          </div>
        }
        filters={
          <Segmented
            options={statusFilters}
            value={statusFilter}
            onChange={(value) => setStatusFilter(value as StatusFilter)}
          />
        }
        rowKey="id"
        columns={columns}
        dataSource={filteredOrders}
        loading={loading}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        emptyState={{
          title: t('admin.orders.empty'),
          description: orders.length === 0 ? t('admin.orders.emptyDescriptionTotal') : t('admin.orders.emptyDescriptionFiltered'),
        }}
        mobileCard={(order) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={4}>
              <Space wrap>
                <Typography.Text code>#{order.id}</Typography.Text>
                {statusTag(order.status, statusTagLabel(order.status))}
              </Space>
              <Typography.Text strong>{planName(order.plan_id)}</Typography.Text>
              <Typography.Text>{userEmail(order.user_id)}</Typography.Text>
              <Typography.Text>{formatMoney(order.price_cents)}</Typography.Text>
              <Typography.Text type="secondary">{formatDate(order.created_at)}</Typography.Text>
              {canRefund(order.status) ? (
                <Button
                  danger
                  size="small"
                  loading={refundOrder.isPending && refundOrder.variables?.id === order.id}
                  onClick={() => confirmRefund(order)}
                >
                  {t('admin.orders.refund')}
                </Button>
              ) : null}
            </Space>
          </Card>
        )}
      />
    </section>
  )
}
