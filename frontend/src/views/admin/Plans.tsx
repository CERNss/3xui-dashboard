import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfigListPage, RefreshButton } from '@/components/common'
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

function formatTraffic(bytes: number, unlimitedLabel: string) {
  if (bytes === 0) return unlimitedLabel
  const gb = bytes / BYTES_PER_GB
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`
  return `${Math.round(gb)} GB`
}

function poolName(pools: ProvisioningPool[], fallback: string, id?: number | null) {
  if (!id) return fallback
  return pools.find((pool) => pool.id === id)?.name ?? `#${id}`
}

export default function Plans() {
  const { t } = useTranslation()
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
      title: t('admin.plans.confirmDelete'),
      content: t('admin.plans.confirmDeleteMsg', { name: plan.name }),
      okText: t('admin.plans.delete'),
      okButtonProps: { danger: true },
      onOk: () => removePlan.mutateAsync(plan.id),
    })
  }

  const columns: ColumnsType<AdminPlan> = [
    {
      title: 'ID',
      dataIndex: 'id',
      align: 'center',
      width: 80,
      render: (id: number) => <Typography.Text type="secondary">#{id}</Typography.Text>,
    },
    {
      title: t('admin.plans.column.name'),
      dataIndex: 'name',
      render: (_value, plan) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{plan.name}</Typography.Text>
          {plan.description ? <Typography.Text type="secondary">{plan.description}</Typography.Text> : null}
          <Typography.Text type="secondary">{t('admin.plans.provisioningPool')}: {poolName(pools, t('admin.plans.provisioningPoolNone'), plan.provisioning_pool_id)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.plans.column.price'),
      dataIndex: 'price_cents',
      align: 'right',
      className: 'table-cell-number',
      width: 120,
      render: (value: number) => <Typography.Text strong>{formatYuan(value)}</Typography.Text>,
    },
    {
      title: t('admin.plans.column.traffic'),
      dataIndex: 'traffic_limit_bytes',
      align: 'right',
      className: 'table-cell-number',
      width: 140,
      render: (value: number) => formatTraffic(value, t('admin.stats.unlimited')),
    },
    {
      title: t('admin.plans.column.duration'),
      dataIndex: 'duration_days',
      align: 'right',
      className: 'table-cell-number',
      width: 120,
      render: (value: number) => `${value} ${t('admin.plans.unitDays')}`,
    },
    {
      title: t('admin.plans.column.status'),
      dataIndex: 'enabled',
      align: 'center',
      width: 110,
      render: (_value, plan) => (
        <Switch
          checked={plan.enabled}
          aria-label={`${plan.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${plan.name}`}
          loading={updatePlan.isPending}
          onChange={() => togglePlan(plan)}
        />
      ),
    },
    {
      title: t('admin.plans.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 112,
      render: (_value, plan) => (
        <Space>
          <Button aria-label={`${t('admin.plans.edit')} ${plan.name}`} icon={<EditOutlined />} onClick={() => openEdit(plan)} />
          <Button
            danger
            aria-label={`${t('admin.plans.delete')} ${plan.name}`}
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(plan)}
          />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <ConfigListPage
        title={t('admin.plans.title')}
        subtitle={t('admin.plans.subtitle')}
        actions={
          <>
            <Button type="primary" aria-label={t('admin.plans.create')} icon={<PlusOutlined />} onClick={openCreate}>
              {t('admin.plans.create')}
            </Button>
            <RefreshButton loading={plansQuery.isFetching || poolsQuery.isFetching} onClick={refresh} label={t('admin.plans.reload')} />
          </>
        }
        alerts={error ? <Alert type="error" showIcon message={t('admin.plans.saveFailed')} /> : null}
        rowKey="id"
        columns={columns}
        dataSource={plans}
        loading={loading}
        pagination={false}
        emptyState={{
          title: t('admin.plans.empty'),
          description: t('admin.plans.emptyDescription'),
          actionLabel: t('admin.plans.create'),
          onAction: openCreate,
        }}
        mobileCard={(plan) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>{plan.name}</Typography.Text>
                <Tag color={plan.enabled ? 'green' : 'default'}>
                  {plan.enabled ? t('admin.provisioningPools.enabled') : t('admin.provisioningPools.disabled')}
                </Tag>
              </Space>
              {plan.description ? <Typography.Text type="secondary">{plan.description}</Typography.Text> : null}
              <Typography.Text>{t('admin.plans.column.price')}: {formatYuan(plan.price_cents)}</Typography.Text>
              <Typography.Text>{t('admin.plans.column.traffic')}: {formatTraffic(plan.traffic_limit_bytes, t('admin.stats.unlimited'))}</Typography.Text>
              <Typography.Text>{t('admin.plans.column.duration')}: {plan.duration_days} {t('admin.plans.unitDays')}</Typography.Text>
              <Typography.Text>{t('admin.plans.provisioningPool')}: {poolName(pools, t('admin.plans.provisioningPoolNone'), plan.provisioning_pool_id)}</Typography.Text>
              <Space wrap>
                <Switch
                  checked={plan.enabled}
                  aria-label={`${plan.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${plan.name}`}
                  onChange={() => togglePlan(plan)}
                />
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(plan)}>
                  {t('admin.plans.edit')}
                </Button>
                <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(plan)}>
                  {t('admin.plans.delete')}
                </Button>
              </Space>
            </Space>
          </Card>
        )}
      />

      <Modal
        title={editing ? t('admin.plans.editTitle', { id: editing.id }) : t('admin.plans.createTitle')}
        open={modalOpen}
        onCancel={closeModal}
        onOk={submit}
        okText={saving ? t('admin.plans.saving') : t('admin.plans.submit')}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={blankForm()} preserve={false}>
          <Form.Item name="name" label={t('admin.plans.name')} rules={[{ required: true, whitespace: true, message: t('admin.plans.nameRequired') }]}>
            <Input placeholder={t('admin.plans.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('admin.plans.description')}>
            <Input placeholder={t('admin.plans.descriptionPlaceholder')} />
          </Form.Item>
          <Space align="start" style={{ width: '100%' }} wrap>
            <Form.Item
              name="price_yuan"
              label={t('admin.plans.price')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.plans.saveFailed') }]}
            >
              <InputNumber min={0} step={0.01} precision={2} prefix="¥" />
            </Form.Item>
            <Form.Item
              name="duration_days"
              label={t('admin.plans.durationDays')}
              rules={[{ required: true, type: 'number', min: 1, message: t('admin.plans.durationDays') }]}
            >
              <InputNumber min={1} precision={0} />
            </Form.Item>
            <Form.Item
              name="traffic_gb"
              label={t('admin.plans.trafficGB')}
              tooltip={t('admin.plans.provisioningPoolHint')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.plans.trafficGB') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item
              name="ip_limit"
              label={t('admin.plans.ipLimit')}
              tooltip={t('admin.provisioningPools.unlimited')}
              rules={[{ type: 'number', min: 0, message: t('admin.plans.ipLimit') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="provisioning_pool_id" label={t('admin.plans.provisioningPool')}>
            <Select
              options={[
                { label: t('admin.plans.provisioningPoolNone'), value: 0 },
                ...pools.map((pool) => ({
                  label: `${pool.name}${pool.enabled ? '' : ' (disabled)'}`,
                  value: pool.id,
                })),
              ]}
            />
          </Form.Item>
          <Form.Item name="enabled" label={t('admin.provisioningPools.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
