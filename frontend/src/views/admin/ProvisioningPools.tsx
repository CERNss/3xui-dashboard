import { DeleteOutlined, DownOutlined, EditOutlined, LinkOutlined, PlusOutlined, RightOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type {
  ProvisioningPool,
  ProvisioningPoolInput,
  ProvisioningPoolTarget,
  ProvisioningPoolTargetInput,
} from '@/api/admin/provisioningPools'
import type { FleetInbound } from '@/api/admin/inbounds'
import { ConfigListPage, EmptyState, RefreshButton, ResponsiveListTable } from '@/components/common'
import { useInboundsFleet } from '@/hooks/queries/admin/inbounds'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import {
  useAddProvisioningPoolTarget,
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
  allowed_protocols?: string[]
}

interface TargetFormValues {
  inboundKey: string  // `${nodeID}|${tag}|${protocol}`
  priority: number
  max_clients: number
  enabled: boolean
}

function blankPool(): PoolFormValues {
  return {
    name: '',
    description: '',
    enabled: true,
    allowed_protocols: [],
  }
}

function poolToForm(pool: ProvisioningPool): PoolFormValues {
  return {
    name: pool.name,
    description: pool.description ?? '',
    enabled: pool.enabled,
    allowed_protocols: pool.allowed_protocols ?? [],
  }
}

function formToPoolPayload(values: PoolFormValues): ProvisioningPoolInput {
  return {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
    enabled: values.enabled,
    allowed_protocols: Array.from(new Set(values.allowed_protocols ?? [])),
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

function capacityText(target: ProvisioningPoolTarget, unlimited: string) {
  const used = target.used_clients ?? 0
  return target.max_clients ? `${used} / ${target.max_clients}` : `${used} / ${unlimited}`
}

function protocolsText(pool: ProvisioningPool, unlimited: string) {
  return pool.allowed_protocols?.length ? pool.allowed_protocols.join(', ') : unlimited
}

function nodeLabel(name: string | undefined, id: number) {
  const trimmed = name?.trim()
  return trimmed || `#${id}`
}

function inboundName(remark: string | undefined, tag: string) {
  const trimmed = remark?.trim()
  return trimmed || tag
}

function targetTitle(target: ProvisioningPoolTarget, fleetRow?: FleetInbound) {
  return `${inboundName(fleetRow?.inbound.remark, target.inbound_tag)} · ${nodeLabel(fleetRow?.node_name ?? target.node_name, target.node_id)}`
}

function targetDetail(target: ProvisioningPoolTarget, fleetRow?: FleetInbound) {
  const protocol = fleetRow?.inbound.protocol || target.protocol || '-'
  const port = fleetRow?.inbound.port
  return port ? `${target.inbound_tag} · ${protocol}:${port}` : `${target.inbound_tag} · ${protocol}`
}

export default function ProvisioningPools() {
  const { t } = useTranslation()
  const [poolForm] = Form.useForm<PoolFormValues>()
  const [targetForm] = Form.useForm<TargetFormValues>()
  const [poolModalOpen, setPoolModalOpen] = useState(false)
  const [editingPool, setEditingPool] = useState<ProvisioningPool | null>(null)
  const [targetPool, setTargetPool] = useState<ProvisioningPool | null>(null)
  const [collapsedTargetPools, setCollapsedTargetPools] = useState<Set<number>>(() => new Set())

  const poolsQuery = useProvisioningPoolsList()
  const nodesQuery = useNodesList()
  const fleetQuery = useInboundsFleet()
  const createPool = useCreateProvisioningPool()
  const updatePool = useUpdateProvisioningPool()
  const removePool = useRemoveProvisioningPool()
  const addTarget = useAddProvisioningPoolTarget()
  const updateTarget = useUpdateProvisioningPoolTarget()
  const removeTarget = useRemoveProvisioningPoolTarget()

  const pools = poolsQuery.data ?? []
  const loading = poolsQuery.isLoading || nodesQuery.isLoading || fleetQuery.isLoading
  const saving = createPool.isPending || updatePool.isPending
  const error =
    poolsQuery.error ??
    nodesQuery.error ??
    createPool.error ??
    updatePool.error ??
    removePool.error ??
    addTarget.error ??
    updateTarget.error ??
    removeTarget.error

  // Inbound options for the "Add inbound" modal — flat list across the
  // fleet, deduped by (node, tag). The allowed_protocols filter on the
  // pool, if set, gates the dropdown to matching protocols.
  const fleetInbounds = useMemo(() => fleetQuery.data?.inbounds ?? [], [fleetQuery.data])
  const fleetInboundByTarget = useMemo(() => {
    const rows = new Map<string, FleetInbound>()
    for (const row of fleetInbounds) {
      rows.set(`${row.node_id}|${row.inbound.tag}`, row)
    }
    return rows
  }, [fleetInbounds])
  const allowedSet = useMemo(
    () => new Set(targetPool?.allowed_protocols ?? []),
    [targetPool],
  )
  const targetAlreadyBound = useMemo(() => {
    const set = new Set<string>()
    for (const tgt of targetPool?.targets ?? []) {
      set.add(`${tgt.node_id}|${tgt.inbound_tag}`)
    }
    return set
  }, [targetPool])
  const inboundOptions = useMemo(
    () =>
      fleetInbounds
        .filter((row) => allowedSet.size === 0 || allowedSet.has(row.inbound.protocol))
        .map((row) => {
          const k = `${row.node_id}|${row.inbound.tag}`
          const isBound = targetAlreadyBound.has(k)
          const label = `${inboundName(row.inbound.remark, row.inbound.tag)} · ${nodeLabel(row.node_name, row.node_id)}`
          return {
            label: `${label}${isBound ? ` · ${t('admin.provisioningPools.alreadyBound')}` : ''}`,
            searchText: [
              label,
              row.inbound.tag,
              row.inbound.protocol,
              row.inbound.port,
            ].join(' '),
            value: `${k}|${row.inbound.protocol}`,
            disabled: isBound,
          }
        }),
    [fleetInbounds, allowedSet, targetAlreadyBound, t],
  )

  const refresh = () => {
    poolsQuery.refetch()
    nodesQuery.refetch()
    fleetQuery.refetch()
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

  const openAddTarget = (pool: ProvisioningPool) => {
    setTargetPool(pool)
    targetForm.resetFields()
    targetForm.setFieldsValue({
      priority: 0,
      max_clients: 0,
      enabled: true,
    })
  }

  const closeAddTarget = () => {
    setTargetPool(null)
    targetForm.resetFields()
  }

  const saveTarget = async () => {
    if (!targetPool) return
    const values = await targetForm.validateFields().catch(() => null)
    if (!values) return
    const parts = values.inboundKey.split('|')
    if (parts.length < 3) return
    const input: ProvisioningPoolTargetInput = {
      node_id: Number(parts[0]),
      inbound_tag: parts[1],
      protocol: parts[2],
      priority: values.priority ?? 0,
      max_clients: values.max_clients ?? 0,
      enabled: values.enabled,
    }
    await addTarget.mutateAsync({ poolID: targetPool.id, input })
    closeAddTarget()
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

  const toggleTargetsExpanded = (poolID: number) => {
    setCollapsedTargetPools((current) => {
      const next = new Set(current)
      if (next.has(poolID)) {
        next.delete(poolID)
      } else {
        next.add(poolID)
      }
      return next
    })
  }

  const targetColumns: ColumnsType<ProvisioningPoolTarget> = [
    {
      title: t('admin.provisioningPools.column.target'),
      dataIndex: 'node_name',
      render: (_value, target) => {
        const fleetRow = fleetInboundByTarget.get(`${target.node_id}|${target.inbound_tag}`)
        return (
          <Space direction="vertical" size={2}>
            <Typography.Text strong>{targetTitle(target, fleetRow)}</Typography.Text>
            <Typography.Text type="secondary">{targetDetail(target, fleetRow)}</Typography.Text>
          </Space>
        )
      },
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
            <RefreshButton loading={poolsQuery.isFetching || nodesQuery.isFetching || fleetQuery.isFetching} onClick={refresh} label={t('admin.nodes.reload')} />
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
                (() => {
                  const targets = pool.targets ?? []
                  const targetCount = targets.length
                  const targetsCollapsed = collapsedTargetPools.has(pool.id)
                  return (
                    <Card
                      key={pool.id}
                      className="provisioning-pool-card"
                      title={
                        <div className="provisioning-pool-card-title">
                          <Typography.Text strong>{pool.name}</Typography.Text>
                          <Tag color={pool.enabled ? 'green' : 'default'}>
                            {pool.enabled ? t('admin.provisioningPools.enabled') : t('admin.provisioningPools.disabled')}
                          </Tag>
                        </div>
                      }
                      extra={
                        <Space className="provisioning-pool-actions" size={8} wrap>
                          <Button
                            type="primary"
                            icon={<LinkOutlined />}
                            aria-label={`${t('admin.provisioningPools.addTarget')} ${pool.name}`}
                            onClick={() => openAddTarget(pool)}
                          >
                            {t('admin.provisioningPools.addTarget')}
                          </Button>
                          <Button aria-label={`${t('admin.provisioningPools.edit')} ${pool.name}`} icon={<EditOutlined />} onClick={() => openEditPool(pool)}>
                            {t('admin.provisioningPools.edit')}
                          </Button>
                          <Button
                            danger
                            aria-label={`${t('admin.provisioningPools.delete')} ${pool.name}`}
                            icon={<DeleteOutlined />}
                            onClick={() => confirmDeletePool(pool)}
                          >
                            {t('admin.provisioningPools.delete')}
                          </Button>
                        </Space>
                      }
                    >
                      <Space direction="vertical" size={12} style={{ width: '100%' }}>
                        <div className="provisioning-pool-meta">
                          <span>
                            <Typography.Text type="secondary">{t('admin.provisioningPools.allowedProtocols')}</Typography.Text>
                            <Typography.Text strong>{protocolsText(pool, t('admin.provisioningPools.unlimitedProtocols'))}</Typography.Text>
                          </span>
                          <span>
                            <Typography.Text type="secondary">{t('admin.provisioningPools.targetsCount')}</Typography.Text>
                            <Typography.Text strong>{targetCount}</Typography.Text>
                          </span>
                        </div>

                        <button
                          type="button"
                          className="provisioning-targets-toggle"
                          aria-expanded={!targetsCollapsed}
                          aria-controls={`provisioning-pool-targets-${pool.id}`}
                          onClick={() => toggleTargetsExpanded(pool.id)}
                        >
                          <span className="provisioning-targets-toggle-title">
                            {targetsCollapsed ? <RightOutlined /> : <DownOutlined />}
                            <span>{t('admin.provisioningPools.column.target')}</span>
                            <Tag>{targetCount}</Tag>
                          </span>
                          <Typography.Text type="secondary">
                            {targetsCollapsed ? t('admin.provisioningPools.expandTargets') : t('admin.provisioningPools.collapseTargets')}
                          </Typography.Text>
                        </button>

                        {!targetsCollapsed ? (
                          <div id={`provisioning-pool-targets-${pool.id}`} className="provisioning-targets-region">
                            {targetCount > 0 ? (
                              <ResponsiveListTable
                                rowKey="id"
                                columns={targetColumns}
                                dataSource={targets}
                                pagination={false}
                                mobileCard={(target) => (
                                  <Card size="small" style={{ width: '100%' }}>
                                    <Space direction="vertical" size={6}>
                                      <Typography.Text strong>{targetTitle(target, fleetInboundByTarget.get(`${target.node_id}|${target.inbound_tag}`))}</Typography.Text>
                                      <Typography.Text type="secondary">{targetDetail(target, fleetInboundByTarget.get(`${target.node_id}|${target.inbound_tag}`))}</Typography.Text>
                                      <Typography.Text>{t('admin.provisioningPools.capacity')}: {capacityText(target, t('admin.provisioningPools.unlimited'))}</Typography.Text>
                                      <Typography.Text>{t('admin.provisioningPools.priority')}: {target.priority}</Typography.Text>
                                      <Space>
                                        <Switch
                                          checked={target.enabled}
                                          aria-label={`${target.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`}
                                          onChange={() => toggleTarget(target)}
                                        />
                                        <Button danger aria-label={`${t('admin.provisioningPools.delete')} ${t('admin.provisioningPools.targetNoun')} ${target.inbound_tag}`} onClick={() => deleteTarget(target)}>
                                          {t('admin.provisioningPools.delete')}
                                        </Button>
                                      </Space>
                                    </Space>
                                  </Card>
                                )}
                              />
                            ) : (
                              <div className="provisioning-targets-empty">
                                <Typography.Text type="secondary">{t('admin.provisioningPools.noTargets')}</Typography.Text>
                                <Button type="primary" icon={<PlusOutlined />} onClick={() => openAddTarget(pool)}>
                                  {t('admin.provisioningPools.addTarget')}
                                </Button>
                              </div>
                            )}
                          </div>
                        ) : null}
                      </Space>
                    </Card>
                  )
                })()
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
          <Form.Item name="allowed_protocols" label={t('admin.provisioningPools.allowedProtocols')}>
            <Select
              mode="multiple"
              allowClear
              placeholder={t('admin.provisioningPools.protocolPlaceholder')}
              options={PROTOCOL_SELECT_OPTIONS}
            />
          </Form.Item>
          <div className="provisioning-pool-switch-row">
            <Space direction="vertical" size={2}>
              <Typography.Text strong>{t('admin.provisioningPools.enabled')}</Typography.Text>
              <Typography.Text type="secondary">{t('admin.provisioningPools.enabledHelp')}</Typography.Text>
            </Space>
            <Form.Item name="enabled" valuePropName="checked" noStyle>
              <Switch aria-label={t('admin.provisioningPools.enabled')} />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      <Modal
        title={targetPool ? t('admin.provisioningPools.addTargetTitle', { name: targetPool.name }) : ''}
        open={Boolean(targetPool)}
        onCancel={closeAddTarget}
        onOk={saveTarget}
        confirmLoading={addTarget.isPending}
        okText={t('admin.provisioningPools.submit')}
        destroyOnHidden
        width={560}
      >
        <Form form={targetForm} layout="vertical" preserve={false}>
          <Form.Item
            name="inboundKey"
            label={t('admin.provisioningPools.field.inbound')}
            rules={[{ required: true, message: t('admin.provisioningPools.field.inboundRequired') }]}
          >
            <Select
              showSearch
              filterOption={(input, option) =>
                String(option?.searchText ?? option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
              placeholder={t('admin.provisioningPools.field.inboundPlaceholder')}
              options={inboundOptions}
              notFoundContent={
                fleetQuery.isLoading
                  ? t('admin.provisioningPools.field.loading')
                  : t('admin.provisioningPools.field.noInboundCandidate')
              }
            />
          </Form.Item>
          <Space size={16} align="start" wrap>
            <Form.Item name="priority" label={t('admin.provisioningPools.priority')} tooltip={t('admin.provisioningPools.field.priorityHint')}>
              <InputNumber min={0} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name="max_clients" label={t('admin.provisioningPools.field.maxClients')} tooltip={t('admin.provisioningPools.field.maxClientsHint')}>
              <InputNumber min={0} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name="enabled" label={t('admin.provisioningPools.field.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}
