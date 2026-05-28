import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { InboundTemplate, InboundTemplateInput } from '@/api/admin/inboundTemplates'
import { ConfigListPage, RefreshButton } from '@/components/common'
import {
  useCreateInboundTemplate,
  useInboundTemplatesList,
  useRemoveInboundTemplate,
  useUpdateInboundTemplate,
} from '@/hooks/queries/admin/inboundTemplates'

const PROTOCOL_OPTIONS = ['vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'hysteria'] as const
type ProtocolName = (typeof PROTOCOL_OPTIONS)[number]

const PROTOCOL_LABELS: Record<ProtocolName, string> = {
  vless: 'VLESS',
  vmess: 'VMess',
  trojan: 'Trojan',
  shadowsocks: 'Shadowsocks',
  wireguard: 'WireGuard',
  hysteria: 'Hysteria',
}

const TRAFFIC_RESET_OPTIONS = ['never', 'daily', 'weekly', 'monthly', 'yearly'] as const

interface TemplateFormValues {
  name: string
  description: string
  enabled: boolean
  protocol: ProtocolName
  remark: string
  listen: string
  total: number
  expiryTime: number
  trafficReset: string
  settings: string
  streamSettings: string
  sniffing: string
}

function prettyJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

function parseJSON(value: string, fallback: Record<string, unknown> = {}) {
  try {
    return JSON.parse(value || '{}') as Record<string, unknown>
  } catch {
    return fallback
  }
}

function defaultSettings(protocol: ProtocolName) {
  if (protocol === 'vless') return { clients: [], decryption: 'none', fallbacks: [] }
  if (protocol === 'vmess') return { clients: [], disableInsecureEncryption: false }
  if (protocol === 'trojan') return { clients: [], fallbacks: [] }
  if (protocol === 'shadowsocks') {
    return { clients: [], method: 'chacha20-ietf-poly1305', network: 'tcp,udp', password: '' }
  }
  if (protocol === 'wireguard') return { clients: [], peers: [], mtu: 1420, secretKey: '', noKernelTun: false }
  return { clients: [], version: 2, auth: '', obfs: '', up_mbps: 100, down_mbps: 100 }
}

function defaultStreamSettings(protocol: ProtocolName) {
  if (protocol === 'hysteria') {
    return {
      network: 'hysteria',
      security: 'tls',
      tlsSettings: { serverName: '', alpn: ['h3'], allowInsecure: false, certificates: [] },
      hysteriaSettings: { version: 2, udpIdleTimeout: 60 },
    }
  }
  return { network: 'tcp', security: 'none' }
}

function blankTemplate(protocol: ProtocolName = 'vless'): TemplateFormValues {
  return {
    name: '',
    description: '',
    enabled: true,
    protocol,
    remark: '',
    listen: '',
    total: 0,
    expiryTime: 0,
    trafficReset: 'never',
    settings: prettyJSON(defaultSettings(protocol)),
    streamSettings: prettyJSON(defaultStreamSettings(protocol)),
    sniffing: prettyJSON({ enabled: true, destOverride: ['http', 'tls'] }),
  }
}

function templateToForm(template: InboundTemplate): TemplateFormValues {
  const protocol = PROTOCOL_OPTIONS.includes(template.protocol as ProtocolName)
    ? (template.protocol as ProtocolName)
    : 'vless'
  return {
    name: template.name,
    description: template.description ?? '',
    enabled: template.enabled,
    protocol,
    remark: template.remark ?? '',
    listen: template.listen ?? '',
    total: template.total ?? 0,
    expiryTime: template.expiryTime ?? 0,
    trafficReset: template.trafficReset || 'never',
    settings: template.settings || prettyJSON(defaultSettings(protocol)),
    streamSettings: template.streamSettings || prettyJSON(defaultStreamSettings(protocol)),
    sniffing: template.sniffing || prettyJSON({ enabled: true, destOverride: ['http', 'tls'] }),
  }
}

function normalizeJSONText(value: string, fallback = '{}') {
  return JSON.stringify(JSON.parse((value || fallback).trim()))
}

function formToPayload(values: TemplateFormValues): InboundTemplateInput {
  return {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
    enabled: values.enabled,
    protocol: values.protocol,
    remark: values.remark?.trim() ?? '',
    listen: values.listen?.trim() ?? '',
    total: Math.max(0, Math.round(values.total || 0)),
    expiryTime: Math.max(0, Math.round(values.expiryTime || 0)),
    trafficReset: values.trafficReset || 'never',
    settings: normalizeJSONText(values.settings),
    streamSettings: normalizeJSONText(values.streamSettings),
    sniffing: normalizeJSONText(values.sniffing),
  }
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let size = Math.abs(value)
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${unit === 0 ? size.toFixed(0) : size.toFixed(2)} ${units[unit]}`
}

function formatExpiry(value: number, never: string) {
  return value ? new Date(value).toLocaleString() : never
}

function clientsCount(template: InboundTemplate) {
  const settings = parseJSON(template.settings)
  return Array.isArray(settings.clients) ? settings.clients.length : 0
}

function transportText(template: InboundTemplate) {
  const stream = parseJSON(template.streamSettings)
  return `${stream.network || 'tcp'} / ${stream.security || 'none'}`
}

function hasClientsArray(value: unknown) {
  return Boolean(value && typeof value === 'object' && Array.isArray((value as { clients?: unknown }).clients))
}

export default function InboundTemplates() {
  const { t } = useTranslation()
  const [form] = Form.useForm<TemplateFormValues>()
  const [query, setQuery] = useState('')
  const [protocols, setProtocols] = useState<ProtocolName[]>([...PROTOCOL_OPTIONS])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<InboundTemplate | null>(null)

  const templatesQuery = useInboundTemplatesList()
  const createTemplate = useCreateInboundTemplate()
  const updateTemplate = useUpdateInboundTemplate()
  const removeTemplate = useRemoveInboundTemplate()

  const templates = templatesQuery.data ?? []
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    const selected = new Set(protocols)
    return templates.filter((template) => {
      if (!selected.has(template.protocol as ProtocolName)) return false
      if (!needle) return true
      return (
        template.name.toLowerCase().includes(needle) ||
        template.description.toLowerCase().includes(needle) ||
        template.protocol.toLowerCase().includes(needle) ||
        template.remark.toLowerCase().includes(needle) ||
        template.listen.toLowerCase().includes(needle)
      )
    })
  }, [protocols, query, templates])
  const enabledCount = templates.filter((template) => template.enabled).length
  const loading = templatesQuery.isLoading
  const saving = createTemplate.isPending || updateTemplate.isPending
  const error = templatesQuery.error ?? createTemplate.error ?? updateTemplate.error ?? removeTemplate.error

  const formInitialValues = editing ? templateToForm(editing) : blankTemplate()
  const formKey = editing ? `edit-${editing.id}` : 'create'

  const refresh = () => {
    templatesQuery.refetch()
  }

  const openCreate = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (template: InboundTemplate) => {
    setEditing(template)
    setModalOpen(true)
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditing(null)
    form.resetFields()
  }

  const applyProtocolDefaults = (protocol: ProtocolName) => {
    form.setFieldsValue({
      settings: prettyJSON(defaultSettings(protocol)),
      streamSettings: prettyJSON(defaultStreamSettings(protocol)),
      sniffing: prettyJSON({ enabled: true, destOverride: ['http', 'tls'] }),
    })
  }

  const saveTemplate = async () => {
    const values = await form.validateFields().catch(() => null)
    if (!values) return
    const payload = formToPayload(values)
    if (editing) {
      await updateTemplate.mutateAsync({ id: editing.id, input: payload })
    } else {
      await createTemplate.mutateAsync(payload)
    }
    closeModal()
  }

  const toggleTemplate = async (template: InboundTemplate) => {
    await updateTemplate.mutateAsync({ id: template.id, input: { enabled: !template.enabled } })
  }

  const confirmDelete = (template: InboundTemplate) => {
    Modal.confirm({
      title: t('admin.inboundTemplates.confirmDelete'),
      content: t('admin.inboundTemplates.confirmDeleteMsg', { name: template.name }),
      okText: t('admin.inboundTemplates.delete'),
      okButtonProps: { danger: true },
      onOk: () => removeTemplate.mutateAsync(template.id),
    })
  }

  const jsonValidator = (label: string, requireClients = false) => async (_: unknown, value?: string) => {
    let parsed: unknown
    try {
      parsed = JSON.parse((value || '{}').trim())
    } catch {
      throw new Error(t('admin.inboundTemplates.invalidJson', { field: label }))
    }
    if (requireClients && !hasClientsArray(parsed)) {
      throw new Error(t('admin.inboundTemplates.clientsRequired'))
    }
  }

  const columns: ColumnsType<InboundTemplate> = [
    {
      title: t('admin.inboundTemplates.column.name'),
      dataIndex: 'name',
      width: 260,
      render: (_value, template) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{template.name}</Typography.Text>
          <Typography.Text type="secondary">{template.description || template.remark || '-'}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.inboundTemplates.column.protocol'),
      dataIndex: 'protocol',
      align: 'center',
      width: 150,
      render: (_value, template) => (
        <Space direction="vertical" size={2} align="center">
          <Tag>{PROTOCOL_LABELS[template.protocol as ProtocolName] ?? template.protocol}</Tag>
          <Typography.Text type="secondary">{transportText(template)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.inboundTemplates.column.limits'),
      key: 'limits',
      width: 220,
      render: (_value, template) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{formatBytes(template.total)}</Typography.Text>
          <Typography.Text type="secondary">
            {formatExpiry(template.expiryTime, t('admin.inboundTemplates.never'))} · {template.trafficReset || 'never'}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.inboundTemplates.column.clients'),
      key: 'clients',
      align: 'right',
      className: 'table-cell-number',
      width: 110,
      render: (_value, template) => clientsCount(template),
    },
    {
      title: t('admin.inboundTemplates.column.status'),
      dataIndex: 'enabled',
      align: 'center',
      width: 100,
      render: (_value, template) => (
        <Switch
          checked={template.enabled}
          aria-label={`${template.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${template.name}`}
          loading={updateTemplate.isPending}
          onChange={() => toggleTemplate(template)}
        />
      ),
    },
    {
      title: t('admin.inboundTemplates.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 120,
      render: (_value, template) => (
        <Space>
          <Button aria-label={`${t('admin.inboundTemplates.edit')} ${template.name}`} icon={<EditOutlined />} onClick={() => openEdit(template)} />
          <Button
            danger
            aria-label={`${t('admin.inboundTemplates.delete')} ${template.name}`}
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(template)}
          />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <ConfigListPage<InboundTemplate>
        title={t('admin.inboundTemplates.title')}
        subtitle={t('admin.inboundTemplates.subtitle')}
        actions={
          <>
            <Button type="primary" aria-label={t('admin.inboundTemplates.create')} icon={<PlusOutlined />} onClick={openCreate}>
              {t('admin.inboundTemplates.create')}
            </Button>
            <RefreshButton loading={templatesQuery.isFetching} onClick={refresh} label={t('common.refresh')} />
          </>
        }
        filters={
          <Space wrap>
            <Input.Search
              allowClear
              aria-label={t('admin.inboundTemplates.searchPlaceholder')}
              placeholder={t('admin.inboundTemplates.searchPlaceholder')}
              style={{ width: 260 }}
              onChange={(event) => setQuery(event.target.value)}
            />
            <Select
              mode="multiple"
              aria-label={t('admin.inboundTemplates.protocolFilter')}
              value={protocols}
              style={{ minWidth: 320 }}
              options={PROTOCOL_OPTIONS.map((protocol) => ({ label: PROTOCOL_LABELS[protocol], value: protocol }))}
              onChange={(value) => setProtocols(value)}
            />
            <Tag>{filtered.length} / {templates.length}</Tag>
          </Space>
        }
        footer={
          <Space size={[12, 4]} wrap>
            <span className="config-list-page-footer-summary">{t('common.resultCount', { n: filtered.length })}</span>
            <Typography.Text type="secondary">
              {t('admin.inboundTemplates.enabledCount', { enabled: enabledCount, total: templates.length })}
            </Typography.Text>
          </Space>
        }
        alerts={error ? <Alert type="error" showIcon message={t('admin.inboundTemplates.operationFailed')} /> : null}
        rowKey="id"
        columns={columns}
        dataSource={filtered}
        loading={loading}
        pagination={false}
        emptyState={{
          title: t('admin.inboundTemplates.empty'),
          description: t('admin.inboundTemplates.emptyDescription'),
          actionLabel: t('admin.inboundTemplates.create'),
          onAction: openCreate,
        }}
        mobileCard={(template) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>{template.name}</Typography.Text>
                <Tag>{PROTOCOL_LABELS[template.protocol as ProtocolName] ?? template.protocol}</Tag>
              </Space>
              <Typography.Text type="secondary">{template.description || template.remark || '-'}</Typography.Text>
              <Typography.Text>{t('admin.inboundTemplates.column.limits')}: {formatBytes(template.total)} / {formatExpiry(template.expiryTime, t('admin.inboundTemplates.never'))}</Typography.Text>
              <Typography.Text>{t('admin.inboundTemplates.column.clients')}: {clientsCount(template)}</Typography.Text>
              <Space wrap>
                <Switch
                  checked={template.enabled}
                  aria-label={`${template.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${template.name}`}
                  onChange={() => toggleTemplate(template)}
                />
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(template)}>
                  {t('admin.inboundTemplates.edit')}
                </Button>
                <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(template)}>
                  {t('admin.inboundTemplates.delete')}
                </Button>
              </Space>
            </Space>
          </Card>
        )}
      />

      <Modal
        title={editing ? t('admin.inboundTemplates.editTitle', { id: editing.id }) : t('admin.inboundTemplates.createTitle')}
        open={modalOpen}
        width={840}
        onCancel={closeModal}
        onOk={saveTemplate}
        okText={saving ? t('common.saving') : t('common.save')}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form key={formKey} form={form} layout="vertical" initialValues={formInitialValues} preserve={false}>
          <Space align="start" wrap>
            <Form.Item name="enabled" label={t('admin.inboundTemplates.enabled')} valuePropName="checked">
              <Switch aria-label={t('admin.inboundTemplates.enabled')} />
            </Form.Item>
            <Form.Item name="protocol" label={t('admin.inboundTemplates.protocol')} rules={[{ required: true }]}>
              <Select
                style={{ width: 180 }}
                options={PROTOCOL_OPTIONS.map((protocol) => ({ label: PROTOCOL_LABELS[protocol], value: protocol }))}
                onChange={(protocol: ProtocolName) => applyProtocolDefaults(protocol)}
              />
            </Form.Item>
          </Space>
          <Space align="start" wrap style={{ width: '100%' }}>
            <Form.Item
              name="name"
              label={t('admin.inboundTemplates.name')}
              rules={[{ required: true, whitespace: true, message: t('admin.inboundTemplates.nameRequired') }]}
            >
              <Input style={{ minWidth: 240 }} placeholder={t('admin.inboundTemplates.namePlaceholder')} />
            </Form.Item>
            <Form.Item name="remark" label={t('admin.inboundTemplates.remark')}>
              <Input style={{ minWidth: 240 }} placeholder={t('admin.inboundTemplates.remarkPlaceholder')} />
            </Form.Item>
            <Form.Item name="listen" label={t('admin.inboundTemplates.listen')}>
              <Input style={{ minWidth: 180 }} placeholder="0.0.0.0" />
            </Form.Item>
          </Space>
          <Form.Item name="description" label={t('admin.inboundTemplates.description')}>
            <Input.TextArea rows={2} placeholder={t('admin.inboundTemplates.descriptionPlaceholder')} />
          </Form.Item>
          <Space align="start" wrap>
            <Form.Item
              name="total"
              label={t('admin.inboundTemplates.total')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.inboundTemplates.nonNegative') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item
              name="expiryTime"
              label={t('admin.inboundTemplates.expiryTime')}
              rules={[{ required: true, type: 'number', min: 0, message: t('admin.inboundTemplates.nonNegative') }]}
            >
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item name="trafficReset" label={t('admin.inboundTemplates.trafficReset')} rules={[{ required: true }]}>
              <Select
                style={{ width: 160 }}
                options={TRAFFIC_RESET_OPTIONS.map((value) => ({
                  label: t(`admin.inboundTemplates.trafficResetOptions.${value}`),
                  value,
                }))}
              />
            </Form.Item>
          </Space>
          <Form.Item
            name="settings"
            label={t('admin.inboundTemplates.settings')}
            rules={[{ required: true }, { validator: jsonValidator(t('admin.inboundTemplates.settings'), true) }]}
          >
            <Input.TextArea rows={6} spellCheck={false} />
          </Form.Item>
          <Form.Item
            name="streamSettings"
            label={t('admin.inboundTemplates.streamSettings')}
            rules={[{ required: true }, { validator: jsonValidator(t('admin.inboundTemplates.streamSettings')) }]}
          >
            <Input.TextArea rows={5} spellCheck={false} />
          </Form.Item>
          <Form.Item
            name="sniffing"
            label={t('admin.inboundTemplates.sniffing')}
            rules={[{ required: true }, { validator: jsonValidator(t('admin.inboundTemplates.sniffing')) }]}
          >
            <Input.TextArea rows={4} spellCheck={false} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
