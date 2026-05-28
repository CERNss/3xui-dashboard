import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type {
  ProvisioningPool,
  ProvisioningPoolInput,
  ProvisioningPoolTarget,
} from '@/api/admin/provisioningPools'
import { ConfigListPage, EmptyState, RefreshButton, ResponsiveListTable } from '@/components/common'
import { useInboundTemplatesList } from '@/hooks/queries/admin/inboundTemplates'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import {
  useCreateProvisioningPool,
  useProvisioningPoolsList,
  useRemoveProvisioningPool,
  useRemoveProvisioningPoolTarget,
  useUpdateProvisioningPool,
  useUpdateProvisioningPoolTarget,
} from '@/hooks/queries/admin/provisioningPools'
import { PROTOCOL_OPTIONS } from './inbounds/utils'

interface PoolFormValues {
  name: string
  description?: string
  enabled: boolean
  auto_create: boolean
  template_id?: number | null
  port_min?: number | null
  port_max?: number | null
  max_clients: number
  allowed_protocols?: string[]
  node_ids?: number[]
}

function blankPool(): PoolFormValues {
  return {
    name: '',
    description: '',
    enabled: true,
    auto_create: true,
    template_id: null,
    port_min: null,
    port_max: null,
    max_clients: 0,
    allowed_protocols: [],
    node_ids: [],
  }
}

function poolToForm(pool: ProvisioningPool): PoolFormValues {
  return {
    name: pool.name,
    description: pool.description ?? '',
    enabled: pool.enabled,
    auto_create: pool.auto_create,
    template_id: pool.template_id ?? null,
    port_min: pool.port_min ?? null,
    port_max: pool.port_max ?? null,
    max_clients: pool.max_clients ?? 0,
    allowed_protocols: pool.allowed_protocols ?? [],
    node_ids: pool.node_ids ?? [],
  }
}

function formToPoolPayload(values: PoolFormValues): ProvisioningPoolInput {
  return {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
    enabled: values.enabled,
    auto_create: values.auto_create,
    template_id: values.template_id ? Number(values.template_id) : null,
    port_min: values.port_min ? Math.round(values.port_min) : null,
    port_max: values.port_max ? Math.round(values.port_max) : null,
    max_clients: Math.max(0, Math.round(values.max_clients ?? 0)),
    allowed_protocols: Array.from(new Set(values.allowed_protocols ?? [])),
    node_ids: Array.from(new Set((values.node_ids ?? []).map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0))),
  }
}

const PROTOCOL_SELECT_OPTIONS = PROTOCOL_OPTIONS.map((protocol) => ({
  value: protocol,
  label:
    protocol === 'vmess'
      ? 'VMess'
      : protocol === 'vless'
        ? 'VLESS'
        : protocol === 'shadowsocks'
          ? 'Shadowsocks'
          : protocol === 'wireguard'
            ? 'WireGuard'
            : protocol === 'hysteria'
              ? 'Hysteria'
              : 'Trojan',
}))

function templateLabel(pool: ProvisioningPool, fallback: string) {
  if (pool.template?.name) return `${pool.template.name} · ${pool.template.protocol}`
  if (pool.template_id) return `#${pool.template_id}`
  return fallback
}

function capacityText(target: ProvisioningPoolTarget, unlimited: string) {
  const used = target.used_clients ?? 0
  return target.max_clients ? `${used} / ${target.max_clients}` : `${used} / ${unlimited}`
}

function protocolsText(pool: ProvisioningPool, unlimited: string) {
  return pool.allowed_protocols?.length ? pool.allowed_protocols.join(', ') : unlimited
}

function nodeScopeText(pool: ProvisioningPool, allNodes: string, countLabel: string) {
  const count = pool.node_ids?.length ?? 0
  if (!count) return allNodes
  return countLabel.replace('{n}', String(count))
}

function targetSourceText(target: ProvisioningPoolTarget, generated: string, manual: string) {
  if (target.generated) return target.template_name ? `${generated} · ${target.template_name}` : generated
  return manual
}

function portRangeText(pool: ProvisioningPool, anyLabel: string) {
  return pool.port_min && pool.port_max ? `${pool.port_min}-${pool.port_max}` : anyLabel
}

export default function ProvisioningPools() {
  const { t } = useTranslation()
  const [poolForm] = Form.useForm<PoolFormValues>()
  const [poolModalOpen, setPoolModalOpen] = useState(false)
  const [editingPool, setEditingPool] = useState<ProvisioningPool | null>(null)

  const poolsQuery = useProvisioningPoolsList()
  const templatesQuery = useInboundTemplatesList()
  const nodesQuery = useNodesList()
  const createPool = useCreateProvisioningPool()
  const updatePool = useUpdateProvisioningPool()
  const removePool = useRemoveProvisioningPool()
  const updateTarget = useUpdateProvisioningPoolTarget()
  const removeTarget = useRemoveProvisioningPoolTarget()

  const pools = poolsQuery.data ?? []
  const templates = useMemo(() => (templatesQuery.data ?? []).filter((template) => template.enabled), [templatesQuery.data])
  const nodes = useMemo(() => (nodesQuery.data ?? []).filter((node) => node.enabled), [nodesQuery.data])
  const loading = poolsQuery.isLoading || templatesQuery.isLoading || nodesQuery.isLoading
  const saving = createPool.isPending || updatePool.isPending
  const error =
    poolsQuery.error ??
    templatesQuery.error ??
    nodesQuery.error ??
    createPool.error ??
    updatePool.error ??
    removePool.error ??
    updateTarget.error ??
    removeTarget.error

  const refresh = () => {
    poolsQuery.refetch()
    templatesQuery.refetch()
    nodesQuery.refetch()
  }

  const poolInitialValues = editingPool ? poolToForm(editingPool) : blankPool()
  const poolFormKey = editingPool ? `edit-${editingPool.id}` : 'create'

  const openCreatePool = () => {
    setEditingPool(null)
    setPoolModalOpen(true)
  }

  const openEditPool = (pool: ProvisioningPool) => {
    setEditingPool(pool)
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
          <Typography.Text type="secondary">
            {targetSourceText(target, t('admin.provisioningPools.generatedTarget'), t('admin.provisioningPools.manualTarget'))}
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
            <RefreshButton loading={poolsQuery.isFetching || templatesQuery.isFetching || nodesQuery.isFetching} onClick={refresh} label={t('admin.nodes.reload')} />
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
                  className="provisioning-pool-card"
                  title={
                    <div className="provisioning-pool-card-title">
                      <Typography.Text strong>{pool.name}</Typography.Text>
                      <Space size={6} wrap>
                        <Tag color={pool.enabled ? 'green' : 'default'}>
                          {pool.enabled ? t('admin.provisioningPools.enabled') : t('admin.provisioningPools.disabled')}
                        </Tag>
                        {pool.auto_create ? <Tag color="blue">{t('admin.provisioningPools.autoCreate')}</Tag> : null}
                      </Space>
                    </div>
                  }
                  extra={
                    <Space>
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
                  <Space direction="vertical" size={14} style={{ width: '100%' }}>
                    <div className="provisioning-pool-summary">
                      <Typography.Text type="secondary">
                        {pool.description || templateLabel(pool, t('admin.provisioningPools.noTemplate'))}
                      </Typography.Text>
                      <Space size={[8, 6]} wrap>
                        <Tag color={pool.template_id ? 'cyan' : 'default'}>
                          {t('admin.provisioningPools.template')}: {templateLabel(pool, t('admin.provisioningPools.noTemplate'))}
                        </Tag>
                        <Tag>
                          {t('admin.provisioningPools.nodes')}: {nodeScopeText(pool, t('admin.provisioningPools.allEnabledNodes'), t('admin.provisioningPools.selectedNodeCount'))}
                        </Tag>
                        <Tag>{protocolsText(pool, t('admin.provisioningPools.unlimitedProtocols'))}</Tag>
                        <Tag>
                          {t('admin.provisioningPools.portRange')}: {portRangeText(pool, t('admin.commonAny', { defaultValue: t('common.all') }))}
                        </Tag>
                        <Tag>
                          {t('admin.provisioningPools.maxClients')}: {pool.max_clients || t('admin.provisioningPools.unlimited')}
                        </Tag>
                        <Tag>
                          {t('admin.provisioningPools.generatedTargets')}: {(pool.targets ?? []).filter((target) => target.generated).length}
                        </Tag>
                      </Space>
                    </div>
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
                              <Typography.Text>{t('admin.provisioningPools.source')}: {targetSourceText(target, t('admin.provisioningPools.generatedTarget'), t('admin.provisioningPools.manualTarget'))}</Typography.Text>
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
                      <Typography.Text type="secondary">{t('admin.provisioningPools.noGeneratedTargets')}</Typography.Text>
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
        <Form key={poolFormKey} form={poolForm} layout="vertical" initialValues={poolInitialValues} preserve={false}>
          <Form.Item name="name" label={t('admin.provisioningPools.name')} rules={[{ required: true, whitespace: true, message: t('admin.provisioningPools.name') }]}>
            <Input placeholder={t('admin.provisioningPools.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('admin.provisioningPools.description')}>
            <Input placeholder={t('admin.provisioningPools.descriptionPlaceholder')} />
          </Form.Item>
          <Form.Item
            name="template_id"
            label={t('admin.provisioningPools.template')}
            rules={[{ required: true, message: t('admin.provisioningPools.chooseTemplate') }]}
          >
            <Select
              allowClear
              placeholder={t('admin.provisioningPools.chooseTemplate')}
              options={templates.map((template) => ({
                value: template.id,
                label: `${template.name} · ${template.protocol}`,
              }))}
            />
          </Form.Item>
          <Form.Item name="node_ids" label={t('admin.provisioningPools.nodes')} tooltip={t('admin.provisioningPools.nodesHelp')}>
            <Select
              mode="multiple"
              allowClear
              placeholder={t('admin.provisioningPools.nodesPlaceholder')}
              options={nodes.map((node) => ({
                value: node.id,
                label: `${node.name} · ${node.host}:${node.port}`,
              }))}
            />
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
            <Form.Item
              name="max_clients"
              label={t('admin.provisioningPools.maxClients')}
              tooltip={t('admin.provisioningPools.unlimited')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.provisioningPools.maxClients') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="allowed_protocols" label={t('admin.provisioningPools.allowedProtocols')}>
            <Select
              mode="multiple"
              allowClear
              placeholder={t('admin.provisioningPools.protocolPlaceholder')}
              options={PROTOCOL_SELECT_OPTIONS}
            />
          </Form.Item>
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            <div className="provisioning-pool-switch-row">
              <Space direction="vertical" size={2}>
                <Typography.Text strong>{t('admin.provisioningPools.enabled')}</Typography.Text>
                <Typography.Text type="secondary">{t('admin.provisioningPools.enabledHelp')}</Typography.Text>
              </Space>
              <Form.Item name="enabled" valuePropName="checked" noStyle>
                <Switch aria-label={t('admin.provisioningPools.enabled')} />
              </Form.Item>
            </div>
            <div className="provisioning-pool-switch-row">
              <Space direction="vertical" size={2}>
                <Typography.Text strong>{t('admin.provisioningPools.autoCreate')}</Typography.Text>
                <Typography.Text type="secondary">{t('admin.provisioningPools.autoCreateHelp')}</Typography.Text>
              </Space>
              <Form.Item name="auto_create" valuePropName="checked" noStyle>
                <Switch aria-label={t('admin.provisioningPools.autoCreate')} />
              </Form.Item>
            </div>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}
