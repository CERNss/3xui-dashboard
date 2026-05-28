import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Switch, Tabs, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { InboundTemplate, InboundTemplateInput } from '@/api/admin/inboundTemplates'
import { ConfigListPage, RefreshButton } from '@/components/common'
import {
  useCreateInboundTemplate,
  useInboundTemplatesList,
  useRemoveInboundTemplate,
  useUpdateInboundTemplate,
} from '@/hooks/queries/admin/inboundTemplates'
import { AdvancedJsonForm } from './inbound-editor/AdvancedJsonForm'
import { blankInboundValues, templateToValues, valuesToTemplateBody } from './inbound-editor/model'
import { SniffingForm } from './inbound-editor/SniffingForm'
import { StreamSettingsForm } from './inbound-editor/StreamSettingsForm'
import { HttpProtocol } from './inbound-editor/protocols/HttpProtocol'
import { HysteriaProtocol } from './inbound-editor/protocols/HysteriaProtocol'
import { MixedProtocol } from './inbound-editor/protocols/MixedProtocol'
import { ShadowsocksProtocol } from './inbound-editor/protocols/ShadowsocksProtocol'
import { TrojanProtocol } from './inbound-editor/protocols/TrojanProtocol'
import { TunnelProtocol } from './inbound-editor/protocols/TunnelProtocol'
import { TunProtocol } from './inbound-editor/protocols/TunProtocol'
import { VlessProtocol } from './inbound-editor/protocols/VlessProtocol'
import { VmessProtocol } from './inbound-editor/protocols/VmessProtocol'
import { WireguardProtocol } from './inbound-editor/protocols/WireguardProtocol'
import type { InboundEditorValues, ProtocolName } from './inbound-editor/types'

const PROTOCOL_OPTIONS = [
  'vless',
  'vmess',
  'trojan',
  'shadowsocks',
  'wireguard',
  'hysteria',
  'http',
  'mixed',
  'tunnel',
  'tun',
] as const

const PROTOCOL_LABELS: Record<ProtocolName, string> = {
  vless: 'VLESS',
  vmess: 'VMess',
  trojan: 'Trojan',
  shadowsocks: 'Shadowsocks',
  wireguard: 'WireGuard',
  hysteria: 'Hysteria',
  http: 'HTTP',
  mixed: 'Mixed',
  tunnel: 'Tunnel',
  tun: 'TUN',
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

function transportText(template: InboundTemplate) {
  try {
    const stream = JSON.parse(template.streamSettings || '{}') as { network?: string; security?: string }
    return `${stream.network || 'tcp'} / ${stream.security || 'none'}`
  } catch {
    return 'tcp / none'
  }
}

// ---- Editor (Modal + Tabs) -------------------------------------------------

interface EditorState {
  open: boolean
  mode: 'create' | 'edit'
  source: InboundTemplate | null
}

interface TemplateMeta {
  name: string
  description: string
  enabled: boolean
}

function blankMeta(): TemplateMeta {
  return { name: '', description: '', enabled: true }
}

function metaFromTemplate(template: InboundTemplate): TemplateMeta {
  return {
    name: template.name,
    description: template.description ?? '',
    enabled: template.enabled,
  }
}

interface TemplateEditorProps {
  open: boolean
  mode: 'create' | 'edit'
  source: InboundTemplate | null
  onClose: () => void
  onSaved?: (template: InboundTemplate) => void
}

function TemplateEditor({ open, mode, source, onClose, onSaved }: TemplateEditorProps) {
  const { t } = useTranslation()
  const [form] = Form.useForm<InboundEditorValues>()
  const [protocol, setProtocol] = useState<ProtocolName>('vless')
  const [meta, setMeta] = useState<TemplateMeta>(blankMeta)
  const createTemplate = useCreateInboundTemplate()
  const updateTemplate = useUpdateInboundTemplate()
  const busy = createTemplate.isPending || updateTemplate.isPending
  const error = createTemplate.error ?? updateTemplate.error

  useEffect(() => {
    if (!open) return
    if (source && mode === 'edit') {
      const values = templateToValues(source)
      form.setFieldsValue(values as unknown as Parameters<typeof form.setFieldsValue>[0])
      setProtocol(values.protocol)
      setMeta(metaFromTemplate(source))
    } else {
      form.setFieldsValue(blankInboundValues(null) as unknown as Parameters<typeof form.setFieldsValue>[0])
      setProtocol('vless')
      setMeta(blankMeta())
    }
  }, [form, mode, open, source])

  const save = async () => {
    if (!meta.name.trim()) {
      Modal.error({ title: t('admin.inboundTemplates.nameRequired') })
      return
    }
    const validated = await form.validateFields().catch(() => null)
    if (!validated) return
    const values = { ...form.getFieldsValue(true), ...validated } as InboundEditorValues
    const wire = valuesToTemplateBody(values)
    const payload: InboundTemplateInput = {
      name: meta.name.trim(),
      description: meta.description.trim(),
      enabled: meta.enabled,
      ...wire,
    }
    const result =
      mode === 'create'
        ? await createTemplate.mutateAsync(payload)
        : await updateTemplate.mutateAsync({ id: source!.id, input: payload })
    onSaved?.(result)
    onClose()
  }

  const protocolFields = () => {
    if (protocol === 'vmess') return <VmessProtocol hideClients />
    if (protocol === 'trojan') return <TrojanProtocol hideClients />
    if (protocol === 'shadowsocks') return <ShadowsocksProtocol hideClients />
    if (protocol === 'wireguard') return <WireguardProtocol hideClients />
    if (protocol === 'hysteria') return <HysteriaProtocol hideClients />
    if (protocol === 'http') return <HttpProtocol hideClients />
    if (protocol === 'mixed') return <MixedProtocol hideClients />
    if (protocol === 'tunnel') return <TunnelProtocol />
    if (protocol === 'tun') return <TunProtocol />
    return <VlessProtocol hideClients />
  }

  const tabs = [
    {
      key: 'basic',
      label: t('admin.inboundEditor.tab.basic'),
      children: (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message={t('admin.inboundTemplates.editorHint')}
          />
          <Space align="start" wrap>
            <Form.Item label={t('admin.inboundTemplates.enabled')}>
              <Switch
                checked={meta.enabled}
                aria-label={t('admin.inboundTemplates.enabled')}
                onChange={(checked) => setMeta((m) => ({ ...m, enabled: checked }))}
              />
            </Form.Item>
            <Form.Item label={t('admin.inboundTemplates.name')} required>
              <Input
                value={meta.name}
                style={{ minWidth: 240 }}
                placeholder={t('admin.inboundTemplates.namePlaceholder')}
                onChange={(event) => setMeta((m) => ({ ...m, name: event.target.value }))}
              />
            </Form.Item>
            <Form.Item name="protocol" label={t('admin.inboundEditor.basicProtocol')} rules={[{ required: true }]}>
              <Select
                style={{ width: 180 }}
                onChange={(value) => {
                  form.setFieldValue('protocol', value)
                  setProtocol(value)
                }}
                options={PROTOCOL_OPTIONS.map((value) => ({ label: PROTOCOL_LABELS[value], value }))}
              />
            </Form.Item>
          </Space>
          <Form.Item label={t('admin.inboundTemplates.description')}>
            <Input.TextArea
              value={meta.description}
              rows={2}
              placeholder={t('admin.inboundTemplates.descriptionPlaceholder')}
              onChange={(event) => setMeta((m) => ({ ...m, description: event.target.value }))}
            />
          </Form.Item>
          {protocol === 'wireguard' ? <Alert type="info" showIcon message={t('admin.inboundEditor.wireguardStreamHidden')} /> : null}
          {protocol === 'hysteria' ? <Alert type="info" showIcon message={t('admin.inboundEditor.hysteriaStreamFixed')} /> : null}
          <Space align="start" wrap>
            <Form.Item name="remark" label={t('admin.inboundTemplates.defaultRemarkLabel')}>
              <Input placeholder={t('admin.inboundTemplates.defaultRemarkPlaceholder')} />
            </Form.Item>
            <Form.Item name="listen" label={t('admin.inboundEditor.basicAddress')}>
              <Input placeholder={t('admin.inboundEditor.basicAddressPlaceholder')} />
            </Form.Item>
            <Form.Item name="total_gb" label={t('admin.inboundEditor.basicTotalGB')}>
              <InputNumber min={0} step={0.01} />
            </Form.Item>
            <Form.Item name="trafficReset" label={t('admin.inboundEditor.basicTrafficReset')}>
              <Select
                style={{ width: 160 }}
                options={['never', 'daily', 'weekly', 'monthly', 'yearly'].map((value) => ({
                  label: t(`admin.inboundEditor.trafficReset.${value}`),
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item name="expiryTime" label={t('admin.inboundEditor.basicExpiry')}>
              <Input type="datetime-local" />
            </Form.Item>
          </Space>
        </Space>
      ),
    },
    { key: 'protocol', label: t('admin.inboundEditor.tab.protocol'), children: protocolFields() },
    ...(['wireguard', 'hysteria', 'tunnel', 'tun'].includes(protocol)
      ? []
      : [
          { key: 'stream', label: t('admin.inboundEditor.tab.stream'), children: <StreamSettingsForm /> },
          { key: 'sniffing', label: t('admin.inboundEditor.tab.sniffing'), children: <SniffingForm /> },
        ]),
    { key: 'advanced', label: t('admin.inboundEditor.tab.advanced'), children: <AdvancedJsonForm /> },
  ]

  return (
    <Modal
      title={mode === 'create' ? t('admin.inboundTemplates.createTitle') : t('admin.inboundTemplates.editTitle', { id: source?.id ?? '' })}
      open={open}
      width={920}
      onCancel={onClose}
      destroyOnClose
      okText={mode === 'create' ? t('common.create') : t('common.save')}
      cancelText={t('admin.inboundEditor.close')}
      confirmLoading={busy}
      onOk={save}
      maskClosable={false}
    >
      {error ? <Alert type="error" showIcon message={t('admin.inboundTemplates.operationFailed')} style={{ marginBottom: 16 }} /> : null}
      <Form
        form={form}
        layout="vertical"
        initialValues={blankInboundValues(null)}
        onValuesChange={(changed) => {
          if (changed.protocol) setProtocol(changed.protocol)
        }}
      >
        <Tabs items={tabs} />
      </Form>
    </Modal>
  )
}

// ---- List view -------------------------------------------------------------

export default function InboundTemplates() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [protocols, setProtocols] = useState<ProtocolName[]>([...PROTOCOL_OPTIONS])
  const [editor, setEditor] = useState<EditorState>({ open: false, mode: 'create', source: null })

  const templatesQuery = useInboundTemplatesList()
  const updateTemplate = useUpdateInboundTemplate()
  const removeTemplate = useRemoveInboundTemplate()

  const templates = useMemo(() => templatesQuery.data ?? [], [templatesQuery.data])
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
  const error = templatesQuery.error ?? updateTemplate.error ?? removeTemplate.error

  const refresh = () => {
    templatesQuery.refetch()
  }

  const openCreate = () => setEditor({ open: true, mode: 'create', source: null })
  const openEdit = (template: InboundTemplate) => setEditor({ open: true, mode: 'edit', source: template })

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

      <TemplateEditor
        open={editor.open}
        mode={editor.mode}
        source={editor.source}
        onClose={() => setEditor((state) => ({ ...state, open: false }))}
      />
    </div>
  )
}
