import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import type { FleetInbound } from '@/api/admin/inbounds'
import type {
  ProvisioningPool,
  ProvisioningPoolInput,
  ProvisioningPoolTarget,
  ProvisioningPoolTargetInput,
} from '@/api/admin/provisioningPools'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import { useInboundsFleet } from '@/hooks/queries/admin/inbounds'
import {
  useAddProvisioningPoolTarget,
  useCreateProvisioningPool,
  useProvisioningPoolsList,
  useRemoveProvisioningPool,
  useRemoveProvisioningPoolTarget,
  useUpdateProvisioningPool,
  useUpdateProvisioningPoolTarget,
} from '@/hooks/queries/admin/provisioningPools'

interface PoolFormValues {
  name: string
  description?: string
  enabled: boolean
  auto_create: boolean
  port_min?: number | null
  port_max?: number | null
  allowed_protocols_text?: string
}

interface TargetFormValues {
  inbound_key: string
  max_clients: number
  priority: number
  enabled: boolean
}

function blankPool(): PoolFormValues {
  return {
    name: '',
    description: '',
    enabled: true,
    auto_create: false,
    port_min: null,
    port_max: null,
    allowed_protocols_text: '',
  }
}

function poolToForm(pool: ProvisioningPool): PoolFormValues {
  return {
    name: pool.name,
    description: pool.description ?? '',
    enabled: pool.enabled,
    auto_create: pool.auto_create,
    port_min: pool.port_min ?? null,
    port_max: pool.port_max ?? null,
    allowed_protocols_text: (pool.allowed_protocols ?? []).join(', '),
  }
}

function formToPoolPayload(values: PoolFormValues): ProvisioningPoolInput {
  return {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
    enabled: values.enabled,
    auto_create: values.auto_create,
    port_min: values.port_min ? Math.round(values.port_min) : null,
    port_max: values.port_max ? Math.round(values.port_max) : null,
    allowed_protocols: (values.allowed_protocols_text ?? '')
      .split(',')
      .map((item) => item.trim().toLowerCase())
      .filter(Boolean),
  }
}

function inboundKey(row: FleetInbound) {
  return `${row.node_id}|${row.inbound.tag}`
}

function parseInboundKey(key: string) {
  const [nodeID, tag] = key.split('|')
  return { nodeID: Number(nodeID), tag }
}

function capacityText(target: ProvisioningPoolTarget) {
  const used = target.used_clients ?? 0
  return target.max_clients ? `${used} / ${target.max_clients}` : `${used} / Unlimited`
}

function protocolsText(pool: ProvisioningPool) {
  return pool.allowed_protocols?.length ? pool.allowed_protocols.join(', ') : 'Unlimited protocols'
}

export default function ProvisioningPools() {
  const [poolForm] = Form.useForm<PoolFormValues>()
  const [targetForm] = Form.useForm<TargetFormValues>()
  const [poolModalOpen, setPoolModalOpen] = useState(false)
  const [targetModalOpen, setTargetModalOpen] = useState(false)
  const [editingPool, setEditingPool] = useState<ProvisioningPool | null>(null)
  const [targetPool, setTargetPool] = useState<ProvisioningPool | null>(null)

  const poolsQuery = useProvisioningPoolsList()
  const fleetQuery = useInboundsFleet()
  const createPool = useCreateProvisioningPool()
  const updatePool = useUpdateProvisioningPool()
  const removePool = useRemoveProvisioningPool()
  const addTarget = useAddProvisioningPoolTarget()
  const updateTarget = useUpdateProvisioningPoolTarget()
  const removeTarget = useRemoveProvisioningPoolTarget()

  const pools = poolsQuery.data ?? []
  const fleet = useMemo(() => fleetQuery.data?.inbounds ?? [], [fleetQuery.data])
  const enabledInbounds = useMemo(
    () =>
      fleet
        .filter((row) => row.inbound.enable)
        .sort((a, b) => a.node_name.localeCompare(b.node_name) || a.inbound.port - b.inbound.port),
    [fleet],
  )
  const loading = poolsQuery.isLoading || fleetQuery.isLoading
  const saving = createPool.isPending || updatePool.isPending || addTarget.isPending
  const error =
    poolsQuery.error ??
    fleetQuery.error ??
    createPool.error ??
    updatePool.error ??
    removePool.error ??
    addTarget.error ??
    updateTarget.error ??
    removeTarget.error

  const refresh = () => {
    poolsQuery.refetch()
    fleetQuery.refetch()
  }

  const openCreatePool = () => {
    setEditingPool(null)
    poolForm.setFieldsValue(blankPool())
    setPoolModalOpen(true)
  }

  const openEditPool = (pool: ProvisioningPool) => {
    setEditingPool(pool)
    poolForm.setFieldsValue(poolToForm(pool))
    setPoolModalOpen(true)
  }

  const closePoolModal = () => {
    setPoolModalOpen(false)
    setEditingPool(null)
    poolForm.resetFields()
  }

  const savePool = async () => {
    const values = await poolForm.validateFields().catch(() => null)
    if (!values) return
    const payload = formToPoolPayload(values)
    if (editingPool) {
      await updatePool.mutateAsync({ id: editingPool.id, input: payload })
    } else {
      await createPool.mutateAsync(payload)
    }
    closePoolModal()
  }

  const confirmDeletePool = (pool: ProvisioningPool) => {
    Modal.confirm({
      title: 'Delete provisioning pool',
      content: `Delete ${pool.name}? Existing targets in the pool will be removed.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => removePool.mutateAsync(pool.id),
    })
  }

  const openAddTarget = (pool: ProvisioningPool) => {
    setTargetPool(pool)
    targetForm.setFieldsValue({
      inbound_key: enabledInbounds[0] ? inboundKey(enabledInbounds[0]) : '',
      max_clients: 0,
      priority: 100,
      enabled: true,
    })
    setTargetModalOpen(true)
  }

  const closeTargetModal = () => {
    setTargetModalOpen(false)
    setTargetPool(null)
    targetForm.resetFields()
  }

  const saveTarget = async () => {
    if (!targetPool) return
    const values = await targetForm.validateFields().catch(() => null)
    if (!values) return
    const { nodeID, tag } = parseInboundKey(values.inbound_key)
    const row = enabledInbounds.find((item) => item.node_id === nodeID && item.inbound.tag === tag)
    if (!row) return

    const payload: ProvisioningPoolTargetInput = {
      node_id: nodeID,
      inbound_tag: tag,
      protocol: row.inbound.protocol,
      max_clients: Math.max(0, Math.round(values.max_clients)),
      priority: Math.max(0, Math.round(values.priority)),
      enabled: values.enabled,
    }
    await addTarget.mutateAsync({ poolID: targetPool.id, input: payload })
    closeTargetModal()
  }

  const toggleTarget = async (target: ProvisioningPoolTarget) => {
    await updateTarget.mutateAsync({ targetID: target.id, input: { enabled: !target.enabled } })
  }

  const deleteTarget = async (target: ProvisioningPoolTarget) => {
    await removeTarget.mutateAsync(target.id)
  }

  const targetColumns: ColumnsType<ProvisioningPoolTarget> = [
    {
      title: 'Target',
      dataIndex: 'node_name',
      render: (_value, target) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{target.node_name || `#${target.node_id}`}</Typography.Text>
          <Typography.Text type="secondary">
            {target.inbound_tag} · {target.protocol || '-'}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Capacity',
      key: 'capacity',
      render: (_value, target) => capacityText(target),
    },
    {
      title: 'Priority',
      dataIndex: 'priority',
    },
    {
      title: 'Status',
      dataIndex: 'enabled',
      render: (_value, target) => (
        <Switch
          checked={target.enabled}
          aria-label={`${target.enabled ? 'Disable' : 'Enable'} target ${target.inbound_tag}`}
          loading={updateTarget.isPending}
          onChange={() => toggleTarget(target)}
        />
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, target) => (
        <Button
          danger
          size="small"
          aria-label={`Delete target ${target.inbound_tag}`}
          icon={<DeleteOutlined />}
          onClick={() => deleteTarget(target)}
        >
          Delete
        </Button>
      ),
    },
  ]

  const portRangeValidator = async () => {
    const min = poolForm.getFieldValue('port_min') as number | null | undefined
    const max = poolForm.getFieldValue('port_max') as number | null | undefined
    if (min && max && min > max) {
      throw new Error('Port minimum must not exceed maximum')
    }
  }

  return (
    <div>
      <PageHeader
        title="Provisioning Pools"
        subtitle="Map plans to enabled fleet inbounds."
        actions={
          <>
            <Button type="primary" aria-label="New Pool" icon={<PlusOutlined />} onClick={openCreatePool}>
              New Pool
            </Button>
            <RefreshButton loading={poolsQuery.isFetching || fleetQuery.isFetching} onClick={refresh} />
          </>
        }
      />

      {error ? <Alert type="error" showIcon message="Provisioning pool operation failed" style={{ marginBottom: 16 }} /> : null}

      {pools.length > 0 || loading ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {pools.map((pool) => (
            <Card
              key={pool.id}
              title={
                <Space>
                  <Typography.Text strong>{pool.name}</Typography.Text>
                  <Tag color={pool.enabled ? 'green' : 'default'}>{pool.enabled ? 'Enabled' : 'Disabled'}</Tag>
                  {pool.auto_create ? <Tag color="blue">Auto create</Tag> : null}
                </Space>
              }
              extra={
                <Space>
                  <Button size="small" aria-label="Add Target" icon={<PlusOutlined />} onClick={() => openAddTarget(pool)}>
                    Add Target
                  </Button>
                  <Button size="small" aria-label={`Edit ${pool.name}`} icon={<EditOutlined />} onClick={() => openEditPool(pool)} />
                  <Button
                    danger
                    size="small"
                    aria-label={`Delete ${pool.name}`}
                    icon={<DeleteOutlined />}
                    onClick={() => confirmDeletePool(pool)}
                  />
                </Space>
              }
            >
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Typography.Text type="secondary">{pool.description || protocolsText(pool)}</Typography.Text>
                <Typography.Text>
                  Ports: {pool.port_min && pool.port_max ? `${pool.port_min}-${pool.port_max}` : 'Any'}
                </Typography.Text>
                {pool.targets?.length ? (
                  <ResponsiveListTable
                    rowKey="id"
                    columns={targetColumns}
                    dataSource={pool.targets}
                    pagination={false}
                    mobileCard={(target) => (
                      <Card size="small" style={{ width: '100%' }}>
                        <Space direction="vertical" size={6}>
                          <Typography.Text strong>{target.node_name || `#${target.node_id}`}</Typography.Text>
                          <Typography.Text type="secondary">
                            {target.inbound_tag} · {target.protocol || '-'}
                          </Typography.Text>
                          <Typography.Text>Capacity: {capacityText(target)}</Typography.Text>
                          <Typography.Text>Priority: {target.priority}</Typography.Text>
                          <Space>
                            <Switch
                              checked={target.enabled}
                              aria-label={`${target.enabled ? 'Disable' : 'Enable'} target ${target.inbound_tag}`}
                              onChange={() => toggleTarget(target)}
                            />
                            <Button size="small" danger aria-label={`Delete target ${target.inbound_tag}`} onClick={() => deleteTarget(target)}>
                              Delete
                            </Button>
                          </Space>
                        </Space>
                      </Card>
                    )}
                  />
                ) : (
                  <Typography.Text type="secondary">No targets yet.</Typography.Text>
                )}
              </Space>
            </Card>
          ))}
        </Space>
      ) : (
        <EmptyState
          title="No provisioning pools"
          description="Create a pool and attach enabled fleet inbounds."
          actionLabel="New Pool"
          onAction={openCreatePool}
        />
      )}

      <Modal
        title={editingPool ? `Edit pool #${editingPool.id}` : 'New Pool'}
        open={poolModalOpen}
        onCancel={closePoolModal}
        onOk={savePool}
        okText={saving ? 'Saving...' : 'Save'}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={poolForm} layout="vertical" initialValues={blankPool()} preserve={false}>
          <Form.Item name="name" label="Name" rules={[{ required: true, whitespace: true, message: 'Name is required' }]}>
            <Input placeholder="Default vless pool" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input placeholder="Optional pool description" />
          </Form.Item>
          <Space align="start" wrap>
            <Form.Item
              name="port_min"
              label="Port min"
              dependencies={['port_max']}
              rules={[
                { type: 'number', min: 1, max: 65535, message: 'Port must be between 1 and 65535' },
                { validator: portRangeValidator },
              ]}
            >
              <InputNumber min={1} max={65535} precision={0} placeholder="10000" />
            </Form.Item>
            <Form.Item
              name="port_max"
              label="Port max"
              dependencies={['port_min']}
              rules={[
                { type: 'number', min: 1, max: 65535, message: 'Port must be between 1 and 65535' },
                { validator: portRangeValidator },
              ]}
            >
              <InputNumber min={1} max={65535} precision={0} placeholder="20000" />
            </Form.Item>
          </Space>
          <Form.Item name="allowed_protocols_text" label="Allowed protocols">
            <Input placeholder="vless, vmess, trojan" />
          </Form.Item>
          <Space>
            <Form.Item name="enabled" label="Enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="auto_create" label="Auto create" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal
        title="Add Target"
        open={targetModalOpen}
        onCancel={closeTargetModal}
        onOk={saveTarget}
        okText={saving ? 'Saving...' : 'Save'}
        confirmLoading={saving}
        okButtonProps={{ disabled: enabledInbounds.length === 0 }}
        destroyOnHidden
      >
        <Form form={targetForm} layout="vertical" preserve={false}>
          <Form.Item
            name="inbound_key"
            label="Inbound"
            rules={[{ required: true, message: 'Choose an enabled inbound' }]}
          >
            <Select
              placeholder="Choose an enabled inbound"
              options={enabledInbounds.map((row) => ({
                value: inboundKey(row),
                label: `${row.node_name} · ${row.inbound.remark || row.inbound.tag} · :${row.inbound.port} · ${row.inbound.protocol}`,
              }))}
            />
          </Form.Item>
          <Space align="start" wrap>
            <Form.Item
              name="max_clients"
              label="Max clients"
              tooltip="0 means unlimited"
              rules={[{ required: true, type: 'number', min: 0, message: 'Max clients must be zero or greater' }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item
              name="priority"
              label="Priority"
              rules={[{ required: true, type: 'number', min: 0, message: 'Priority must be zero or greater' }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
