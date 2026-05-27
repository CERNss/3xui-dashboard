import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { FleetInbound } from '@/api/admin/inbounds'
import type {
  ProvisioningPool,
  ProvisioningPoolInput,
  ProvisioningPoolTarget,
  ProvisioningPoolTargetInput,
} from '@/api/admin/provisioningPools'
import { ConfigListPage, EmptyState, RefreshButton, ResponsiveListTable } from '@/components/common'
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

function capacityText(target: ProvisioningPoolTarget, unlimited: string) {
  const used = target.used_clients ?? 0
  return target.max_clients ? `${used} / ${target.max_clients}` : `${used} / ${unlimited}`
}

function protocolsText(pool: ProvisioningPool, unlimited: string) {
  return pool.allowed_protocols?.length ? pool.allowed_protocols.join(', ') : unlimited
}

export default function ProvisioningPools() {
  const { t } = useTranslation()
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
      title: t('admin.provisioningPools.confirmDelete'),
      content: t('admin.provisioningPools.confirmDeleteMsg', { name: pool.name }),
      okText: t('admin.provisioningPools.delete'),
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
      title: t('admin.provisioningPools.column.target'),
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
      title: t('admin.provisioningPools.column.capacity'),
      key: 'capacity',
      align: 'right',
      className: 'table-cell-number',
      width: 140,
      render: (_value, target) => capacityText(target, t('admin.provisioningPools.unlimited')),
    },
    {
      title: t('admin.provisioningPools.priority'),
      dataIndex: 'priority',
      align: 'center',
      className: 'table-cell-number',
      width: 96,
    },
    {
      title: t('admin.provisioningPools.column.status'),
      dataIndex: 'enabled',
      align: 'center',
      width: 96,
      render: (_value, target) => (
        <Switch
          checked={target.enabled}
          aria-label={`${target.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`}
          loading={updateTarget.isPending}
          onChange={() => toggleTarget(target)}
        />
      ),
    },
    {
      title: t('admin.provisioningPools.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 120,
      render: (_value, target) => (
        <Button
          danger
          size="small"
          aria-label={`${t('admin.provisioningPools.delete')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`}
          icon={<DeleteOutlined />}
          onClick={() => deleteTarget(target)}
        >
          {t('admin.provisioningPools.delete')}
        </Button>
      ),
    },
  ]

  const portRangeValidator = async () => {
    const min = poolForm.getFieldValue('port_min') as number | null | undefined
    const max = poolForm.getFieldValue('port_max') as number | null | undefined
    if (min && max && min > max) {
      throw new Error(t('admin.provisioningPools.portRangeInvalid'))
    }
  }

  return (
    <div>
      <ConfigListPage<ProvisioningPool>
        title={t('admin.provisioningPools.title')}
        subtitle={t('admin.provisioningPools.subtitle')}
        actions={
          <>
            <Button type="primary" aria-label={t('admin.provisioningPools.add')} icon={<PlusOutlined />} onClick={openCreatePool}>
              {t('admin.provisioningPools.add')}
            </Button>
            <RefreshButton loading={poolsQuery.isFetching || fleetQuery.isFetching} onClick={refresh} label={t('admin.nodes.reload')} />
          </>
        }
        alerts={error ? <Alert type="error" showIcon message={t('admin.provisioningPools.saveFailed')} /> : null}
        dataSource={pools}
        loading={loading}
        listClassName="config-list-page-card-stack"
        listContent={
          pools.length > 0 || loading ? (
            <>
              {pools.map((pool) => (
                <Card
                  key={pool.id}
                  title={
                    <Space>
                      <Typography.Text strong>{pool.name}</Typography.Text>
                      <Tag color={pool.enabled ? 'green' : 'default'}>{pool.enabled ? t('admin.provisioningPools.enabled') : t('admin.provisioningPools.disabled')}</Tag>
                      {pool.auto_create ? <Tag color="blue">{t('admin.provisioningPools.autoCreate')}</Tag> : null}
                    </Space>
                  }
                  extra={
                    <Space>
                      <Button size="small" aria-label={t('admin.provisioningPools.addTarget')} icon={<PlusOutlined />} onClick={() => openAddTarget(pool)}>
                        {t('admin.provisioningPools.addTarget')}
                      </Button>
                      <Button size="small" aria-label={`${t('admin.provisioningPools.edit')} ${pool.name}`} icon={<EditOutlined />} onClick={() => openEditPool(pool)} />
                      <Button
                        danger
                        size="small"
                        aria-label={`${t('admin.provisioningPools.delete')} ${pool.name}`}
                        icon={<DeleteOutlined />}
                        onClick={() => confirmDeletePool(pool)}
                      />
                    </Space>
                  }
                >
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    <Typography.Text type="secondary">{pool.description || protocolsText(pool, t('admin.provisioningPools.unlimitedProtocols'))}</Typography.Text>
                    <Typography.Text>
                      {t('admin.provisioningPools.portRange')}: {pool.port_min && pool.port_max ? `${pool.port_min}-${pool.port_max}` : t('admin.commonAny', { defaultValue: t('common.all') })}
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
                              <Typography.Text>{t('admin.provisioningPools.capacity')}: {capacityText(target, t('admin.provisioningPools.unlimited'))}</Typography.Text>
                              <Typography.Text>{t('admin.provisioningPools.priority')}: {target.priority}</Typography.Text>
                              <Space>
                                <Switch
                                  checked={target.enabled}
                                  aria-label={`${target.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`}
                                  onChange={() => toggleTarget(target)}
                                />
                                <Button size="small" danger aria-label={`${t('admin.provisioningPools.delete')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`} onClick={() => deleteTarget(target)}>
                                  {t('admin.provisioningPools.delete')}
                                </Button>
                              </Space>
                            </Space>
                          </Card>
                        )}
                      />
                    ) : (
                      <Typography.Text type="secondary">{t('admin.provisioningPools.noTargets')}</Typography.Text>
                    )}
                  </Space>
                </Card>
              ))}
            </>
          ) : (
            <EmptyState
              title={t('admin.provisioningPools.empty')}
              description={t('admin.provisioningPools.emptyDescription')}
              actionLabel={t('admin.provisioningPools.add')}
              onAction={openCreatePool}
            />
          )
        }
      />

      <Modal
        title={editingPool ? t('admin.provisioningPools.editTitle', { id: editingPool.id }) : t('admin.provisioningPools.createTitle')}
        open={poolModalOpen}
        onCancel={closePoolModal}
        onOk={savePool}
        okText={saving ? t('admin.provisioningPools.saving') : t('admin.provisioningPools.submit')}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={poolForm} layout="vertical" initialValues={blankPool()} preserve={false}>
          <Form.Item name="name" label={t('admin.provisioningPools.name')} rules={[{ required: true, whitespace: true, message: t('admin.provisioningPools.name') }]}>
            <Input placeholder={t('admin.provisioningPools.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('admin.provisioningPools.description')}>
            <Input placeholder={t('admin.provisioningPools.descriptionPlaceholder')} />
          </Form.Item>
          <Space align="start" wrap>
            <Form.Item
              name="port_min"
              label={t('admin.provisioningPools.portMin')}
              dependencies={['port_max']}
              rules={[
                { type: 'number', min: 1, max: 65535, message: t('admin.inboundEditor.errPort') },
                { validator: portRangeValidator },
              ]}
            >
              <InputNumber min={1} max={65535} precision={0} placeholder="10000" />
            </Form.Item>
            <Form.Item
              name="port_max"
              label={t('admin.provisioningPools.portMax')}
              dependencies={['port_min']}
              rules={[
                { type: 'number', min: 1, max: 65535, message: t('admin.inboundEditor.errPort') },
                { validator: portRangeValidator },
              ]}
            >
              <InputNumber min={1} max={65535} precision={0} placeholder="20000" />
            </Form.Item>
          </Space>
          <Form.Item name="allowed_protocols_text" label={t('admin.provisioningPools.allowedProtocols')}>
            <Input placeholder={t('admin.provisioningPools.protocolPlaceholder')} />
          </Form.Item>
          <Space>
            <Form.Item name="enabled" label={t('admin.provisioningPools.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="auto_create" label={t('admin.provisioningPools.autoCreate')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal
        title={t('admin.provisioningPools.targetCreateTitle')}
        open={targetModalOpen}
        onCancel={closeTargetModal}
        onOk={saveTarget}
        okText={saving ? t('admin.provisioningPools.saving') : t('admin.provisioningPools.submit')}
        confirmLoading={saving}
        okButtonProps={{ disabled: enabledInbounds.length === 0 }}
        destroyOnHidden
      >
        <Form form={targetForm} layout="vertical" preserve={false}>
          <Form.Item
            name="inbound_key"
            label={t('admin.provisioningPools.inbound')}
            rules={[{ required: true, message: t('admin.provisioningPools.chooseInbound') }]}
          >
            <Select
              placeholder={t('admin.provisioningPools.chooseInbound')}
              options={enabledInbounds.map((row) => ({
                value: inboundKey(row),
                label: `${row.node_name} · ${row.inbound.remark || row.inbound.tag} · :${row.inbound.port} · ${row.inbound.protocol}`,
              }))}
            />
          </Form.Item>
          <Space align="start" wrap>
            <Form.Item
              name="max_clients"
              label={t('admin.provisioningPools.maxClients')}
              tooltip={t('admin.provisioningPools.unlimited')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.provisioningPools.maxClients') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item
              name="priority"
              label={t('admin.provisioningPools.priority')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.provisioningPools.priority') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="enabled" label={t('admin.provisioningPools.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
