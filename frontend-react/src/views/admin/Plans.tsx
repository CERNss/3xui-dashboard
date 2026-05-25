import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useState } from 'react'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import type { AdminPlan, CreatePlanInput } from '@/api/admin/plans'
import type { ProvisioningPool } from '@/api/admin/provisioningPools'
import { useCreatePlan, usePlansList, useRemovePlan, useUpdatePlan } from '@/hooks/queries/admin/plans'
import { useProvisioningPoolsList } from '@/hooks/queries/admin/provisioningPools'

const BYTES_PER_GB = 1024 * 1024 * 1024

interface PlanFormValues {
  name: string
  description?: string
  duration_days: number
  traffic_gb: number
  price_yuan: number
  ip_limit: number
  provisioning_pool_id?: number
  enabled: boolean
}

function blankForm(): PlanFormValues {
  return {
    name: '',
    description: '',
    duration_days: 30,
    traffic_gb: 100,
    price_yuan: 5,
    ip_limit: 0,
    provisioning_pool_id: 0,
    enabled: true,
  }
}

function planToForm(plan: AdminPlan): PlanFormValues {
  return {
    name: plan.name,
    description: plan.description ?? '',
    duration_days: plan.duration_days,
    traffic_gb: Math.round(plan.traffic_limit_bytes / BYTES_PER_GB),
    price_yuan: plan.price_cents / 100,
    ip_limit: plan.ip_limit ?? 0,
    provisioning_pool_id: plan.provisioning_pool_id ?? 0,
    enabled: plan.enabled,
  }
}

function formToPayload(values: PlanFormValues): CreatePlanInput {
  return {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
    duration_days: Math.max(1, Math.round(values.duration_days)),
    traffic_limit_bytes: Math.max(0, Math.round(values.traffic_gb)) * BYTES_PER_GB,
    price_cents: Math.max(0, Math.round(values.price_yuan * 100)),
    ip_limit: Math.max(0, Math.round(values.ip_limit ?? 0)),
    provisioning_pool_id: values.provisioning_pool_id ? Number(values.provisioning_pool_id) : null,
    enabled: values.enabled,
  }
}

function formatYuan(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

function formatTraffic(bytes: number) {
  if (bytes === 0) return 'Unlimited'
  const gb = bytes / BYTES_PER_GB
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`
  return `${Math.round(gb)} GB`
}

function poolName(pools: ProvisioningPool[], id?: number | null) {
  if (!id) return 'No provisioning pool'
  return pools.find((pool) => pool.id === id)?.name ?? `#${id}`
}

export default function Plans() {
  const [form] = Form.useForm<PlanFormValues>()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<AdminPlan | null>(null)

  const plansQuery = usePlansList()
  const poolsQuery = useProvisioningPoolsList()
  const createPlan = useCreatePlan()
  const updatePlan = useUpdatePlan()
  const removePlan = useRemovePlan()

  const plans = plansQuery.data ?? []
  const pools = poolsQuery.data ?? []
  const loading = plansQuery.isLoading || poolsQuery.isLoading
  const saving = createPlan.isPending || updatePlan.isPending
  const error = plansQuery.error ?? poolsQuery.error ?? createPlan.error ?? updatePlan.error ?? removePlan.error

  const openCreate = () => {
    setEditing(null)
    form.setFieldsValue(blankForm())
    setModalOpen(true)
  }

  const openEdit = (plan: AdminPlan) => {
    setEditing(plan)
    form.setFieldsValue(planToForm(plan))
    setModalOpen(true)
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditing(null)
    form.resetFields()
  }

  const refresh = () => {
    plansQuery.refetch()
    poolsQuery.refetch()
  }

  const submit = async () => {
    const values = await form.validateFields().catch(() => null)
    if (!values) return
    const payload = formToPayload(values)
    if (editing) {
      await updatePlan.mutateAsync({ id: editing.id, input: payload })
    } else {
      await createPlan.mutateAsync(payload)
    }
    closeModal()
  }

  const togglePlan = async (plan: AdminPlan) => {
    await updatePlan.mutateAsync({ id: plan.id, input: { enabled: !plan.enabled } })
  }

  const confirmDelete = (plan: AdminPlan) => {
    Modal.confirm({
      title: 'Delete plan',
      content: `Delete ${plan.name}? Existing orders are not changed, but the plan will no longer be available.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => removePlan.mutateAsync(plan.id),
    })
  }

  const columns: ColumnsType<AdminPlan> = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
      render: (id: number) => <Typography.Text type="secondary">#{id}</Typography.Text>,
    },
    {
      title: 'Name',
      dataIndex: 'name',
      render: (_value, plan) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{plan.name}</Typography.Text>
          {plan.description ? <Typography.Text type="secondary">{plan.description}</Typography.Text> : null}
          <Typography.Text type="secondary">Pool: {poolName(pools, plan.provisioning_pool_id)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Price',
      dataIndex: 'price_cents',
      align: 'right',
      render: (value: number) => <Typography.Text strong>{formatYuan(value)}</Typography.Text>,
    },
    {
      title: 'Traffic',
      dataIndex: 'traffic_limit_bytes',
      align: 'right',
      render: (value: number) => formatTraffic(value),
    },
    {
      title: 'Duration',
      dataIndex: 'duration_days',
      align: 'right',
      render: (value: number) => `${value} days`,
    },
    {
      title: 'Status',
      dataIndex: 'enabled',
      render: (_value, plan) => (
        <Switch
          checked={plan.enabled}
          aria-label={`${plan.enabled ? 'Disable' : 'Enable'} ${plan.name}`}
          loading={updatePlan.isPending}
          onChange={() => togglePlan(plan)}
        />
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, plan) => (
        <Space>
          <Button aria-label={`Edit ${plan.name}`} icon={<EditOutlined />} onClick={() => openEdit(plan)} />
          <Button
            danger
            aria-label={`Delete ${plan.name}`}
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(plan)}
          />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title="Plans"
        subtitle="Create and maintain subscription plans."
        actions={
          <>
            <Button type="primary" aria-label="New Plan" icon={<PlusOutlined />} onClick={openCreate}>
              New Plan
            </Button>
            <RefreshButton loading={plansQuery.isFetching || poolsQuery.isFetching} onClick={refresh} />
          </>
        }
      />

      {error ? <Alert type="error" showIcon message="Plan operation failed" style={{ marginBottom: 16 }} /> : null}

      {plans.length > 0 || loading ? (
        <ResponsiveListTable
          rowKey="id"
          columns={columns}
          dataSource={plans}
          loading={loading}
          pagination={false}
          mobileCard={(plan) => (
            <Card size="small" style={{ width: '100%' }}>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                  <Typography.Text strong>{plan.name}</Typography.Text>
                  <Tag color={plan.enabled ? 'green' : 'default'}>{plan.enabled ? 'Enabled' : 'Disabled'}</Tag>
                </Space>
                {plan.description ? <Typography.Text type="secondary">{plan.description}</Typography.Text> : null}
                <Typography.Text>Price: {formatYuan(plan.price_cents)}</Typography.Text>
                <Typography.Text>Traffic: {formatTraffic(plan.traffic_limit_bytes)}</Typography.Text>
                <Typography.Text>Duration: {plan.duration_days} days</Typography.Text>
                <Typography.Text>Pool: {poolName(pools, plan.provisioning_pool_id)}</Typography.Text>
                <Space wrap>
                  <Switch
                    checked={plan.enabled}
                    aria-label={`${plan.enabled ? 'Disable' : 'Enable'} ${plan.name}`}
                    onChange={() => togglePlan(plan)}
                  />
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(plan)}>
                    Edit
                  </Button>
                  <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(plan)}>
                    Delete
                  </Button>
                </Space>
              </Space>
            </Card>
          )}
        />
      ) : (
        <EmptyState
          title="No plans"
          description="Create a plan to make it available to portal users."
          actionLabel="New Plan"
          onAction={openCreate}
        />
      )}

      <Modal
        title={editing ? `Edit plan #${editing.id}` : 'New Plan'}
        open={modalOpen}
        onCancel={closeModal}
        onOk={submit}
        okText={saving ? 'Saving...' : 'Save'}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={blankForm()} preserve={false}>
          <Form.Item name="name" label="Name" rules={[{ required: true, whitespace: true, message: 'Name is required' }]}>
            <Input placeholder="Premium Monthly" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input placeholder="Optional plan description" />
          </Form.Item>
          <Space align="start" style={{ width: '100%' }} wrap>
            <Form.Item
              name="price_yuan"
              label="Price"
              rules={[{ required: true, type: 'number', min: 0, message: 'Price must be zero or greater' }]}
            >
              <InputNumber min={0} step={0.01} precision={2} prefix="¥" />
            </Form.Item>
            <Form.Item
              name="duration_days"
              label="Duration days"
              rules={[{ required: true, type: 'number', min: 1, message: 'Duration must be at least 1 day' }]}
            >
              <InputNumber min={1} precision={0} />
            </Form.Item>
            <Form.Item
              name="traffic_gb"
              label="Traffic GB"
              tooltip="0 means unlimited"
              rules={[{ required: true, type: 'number', min: 0, message: 'Traffic must be zero or greater' }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item
              name="ip_limit"
              label="IP limit"
              tooltip="0 means unlimited"
              rules={[{ type: 'number', min: 0, message: 'IP limit must be zero or greater' }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="provisioning_pool_id" label="Provisioning pool">
            <Select
              options={[
                { label: 'No provisioning pool', value: 0 },
                ...pools.map((pool) => ({
                  label: `${pool.name}${pool.enabled ? '' : ' (disabled)'}`,
                  value: pool.id,
                })),
              ]}
            />
          </Form.Item>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
