import { Button, Card, Empty, Modal, Segmented, Space, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import type { AdminOrder, OrderStatus } from '@/api/admin/orders'
import type { AdminPlan } from '@/api/admin/plans'
import type { AdminUser } from '@/api/admin/users'
import { PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import { useOrdersList, useRefundOrder } from '@/hooks/queries/admin/orders'
import { usePlansList } from '@/hooks/queries/admin/plans'
import { useUsersList } from '@/hooks/queries/admin/users'

type StatusFilter = 'all' | OrderStatus

const STATUS_FILTERS: Array<{ label: string; value: StatusFilter }> = [
  { label: 'All', value: 'all' },
  { label: 'Completed', value: 'completed' },
  { label: 'Paid', value: 'paid' },
  { label: 'Failed', value: 'failed' },
  { label: 'Refunded', value: 'refunded' },
  { label: 'Created', value: 'created' },
]

function formatMoney(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '-'
}

function canRefund(status: OrderStatus) {
  return status === 'completed' || status === 'paid'
}

function statusTag(status: OrderStatus) {
  const colorByStatus: Record<OrderStatus, string> = {
    completed: 'green',
    paid: 'green',
    failed: 'red',
    refunded: 'gold',
    created: 'default',
  }

  return <Tag color={colorByStatus[status]}>{status}</Tag>
}

function asMap<T extends { id: number }>(items: T[] = []) {
  return new Map(items.map((item) => [item.id, item]))
}

export default function Orders() {
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

  const planName = (id: number) => plansById.get(id)?.name ?? `Plan #${id}`
  const userEmail = (id: number) => usersById.get(id)?.email ?? `User #${id}`

  const refresh = () => {
    ordersQuery.refetch()
    plansQuery.refetch()
    usersQuery.refetch()
  }

  const confirmRefund = (order: AdminOrder) => {
    Modal.confirm({
      title: `Refund order #${order.id}`,
      content: `Refund ${formatMoney(order.price_cents)} for ${userEmail(order.user_id)} / ${planName(order.plan_id)}?`,
      okText: 'Refund',
      okButtonProps: { danger: true },
      onOk: async () => {
        await refundOrder.mutateAsync({ id: order.id, reason: 'admin manual refund' })
        message.success(`Order #${order.id} refunded`)
      },
    })
  }

  const columns: ColumnsType<AdminOrder> = [
    {
      title: 'Order',
      dataIndex: 'id',
      render: (id: number) => <Typography.Text code>#{id}</Typography.Text>,
    },
    {
      title: 'User',
      dataIndex: 'user_id',
      render: (id: number) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{userEmail(id)}</Typography.Text>
          <Typography.Text type="secondary">user #{id}</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Plan',
      dataIndex: 'plan_id',
      render: (id: number) => planName(id),
    },
    {
      title: 'Amount',
      dataIndex: 'price_cents',
      align: 'right',
      render: formatMoney,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      render: (status: OrderStatus, order) => (
        <Space direction="vertical" size={2}>
          {statusTag(status)}
          {order.error_message ? <Typography.Text type="danger">{order.error_message}</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      sorter: (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      defaultSortOrder: 'descend',
      render: formatDate,
    },
    {
      title: 'Completed',
      dataIndex: 'completed_at',
      render: formatDate,
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_, order) =>
        canRefund(order.status) ? (
          <Button
            danger
            size="small"
            loading={refundOrder.isPending && refundOrder.variables?.id === order.id}
            onClick={() => confirmRefund(order)}
          >
            Refund
          </Button>
        ) : null,
    },
  ]

  return (
    <section>
      <PageHeader
        title="Orders"
        subtitle="Review purchases, reconcile plan and user details, and process eligible refunds."
        actions={<RefreshButton loading={refreshing} onClick={refresh} />}
      />

      <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', marginBottom: 16 }}>
        <Card size="small">
          <Typography.Text type="secondary">Total orders</Typography.Text>
          <Typography.Title level={3} style={{ margin: 0 }}>
            {stats.total}
          </Typography.Title>
        </Card>
        <Card size="small">
          <Typography.Text type="secondary">Completed or paid</Typography.Text>
          <Typography.Title level={3} style={{ margin: 0 }}>
            {stats.completed}
          </Typography.Title>
        </Card>
        <Card size="small">
          <Typography.Text type="secondary">Revenue</Typography.Text>
          <Typography.Title level={3} style={{ margin: 0 }}>
            {formatMoney(stats.revenue)}
          </Typography.Title>
        </Card>
      </div>

      <Segmented
        options={STATUS_FILTERS}
        value={statusFilter}
        onChange={(value) => setStatusFilter(value as StatusFilter)}
        style={{ marginBottom: 16 }}
      />

      <ResponsiveListTable<AdminOrder>
        rowKey="id"
        columns={columns}
        dataSource={filteredOrders}
        loading={loading}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        locale={{
          emptyText: (
            <Empty
              description={orders.length === 0 ? 'No orders yet.' : 'No orders match the selected status.'}
            />
          ),
        }}
        mobileCard={(order) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={4}>
              <Space wrap>
                <Typography.Text code>#{order.id}</Typography.Text>
                {statusTag(order.status)}
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
                  Refund
                </Button>
              ) : null}
            </Space>
          </Card>
        )}
      />
    </section>
  )
}
