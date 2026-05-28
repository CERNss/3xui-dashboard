import { Alert, Button, Drawer, Form, Input, InputNumber, Select, Space, Switch, Tabs } from 'antd'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateInbound, useUpdateInbound } from '@/hooks/queries/admin/inbounds'
import { useInboundTemplatesList } from '@/hooks/queries/admin/inboundTemplates'
import { AdvancedJsonForm } from './inbound-editor/AdvancedJsonForm'
import { blankInboundValues, inboundToValues, valuesToInboundBody } from './inbound-editor/model'
import { SniffingForm } from './inbound-editor/SniffingForm'
import { StreamSettingsForm } from './inbound-editor/StreamSettingsForm'
import type { InboundEditorProps, InboundEditorValues, ProtocolName } from './inbound-editor/types'
import { HysteriaProtocol } from './inbound-editor/protocols/HysteriaProtocol'
import { ShadowsocksProtocol } from './inbound-editor/protocols/ShadowsocksProtocol'
import { TrojanProtocol } from './inbound-editor/protocols/TrojanProtocol'
import { VlessProtocol } from './inbound-editor/protocols/VlessProtocol'
import { VmessProtocol } from './inbound-editor/protocols/VmessProtocol'
import { WireguardProtocol } from './inbound-editor/protocols/WireguardProtocol'

export default function InboundEditor({ open, mode, nodeID, tag, source, nodes, onClose, onSaved }: InboundEditorProps) {
  const { t } = useTranslation()
  const [form] = Form.useForm<InboundEditorValues>()
  const [protocol, setProtocol] = useState<ProtocolName>('vless')
  const [templateID, setTemplateID] = useState<number | null>(null)
  const createInbound = useCreateInbound()
  const updateInbound = useUpdateInbound()
  const templatesQuery = useInboundTemplatesList()
  const busy = createInbound.isPending || updateInbound.isPending
  const error = createInbound.error ?? updateInbound.error

  useEffect(() => {
    if (!open) return
    const values = source && mode === 'edit' ? inboundToValues(source, nodeID) : blankInboundValues(nodeID)
    form.setFieldsValue(values as unknown as Parameters<typeof form.setFieldsValue>[0])
    setProtocol(values.protocol)
    setTemplateID(null)
  }, [form, mode, nodeID, open, source])

  const save = async () => {
    const validated = await form.validateFields().catch(() => null)
    if (typeof validated?.node_id !== 'number') return
    const values = { ...form.getFieldsValue(true), ...validated } as InboundEditorValues
    values.node_id = validated.node_id
    const body = valuesToInboundBody(values)
    const result =
      mode === 'create'
        ? await createInbound.mutateAsync({
            nodeID: validated.node_id,
            body: templateID ? { ...body, template_id: templateID } : body,
          })
        : await updateInbound.mutateAsync({ nodeID: validated.node_id, tag, body })
    onSaved?.(result)
    onClose()
  }

  const protocolFields = () => {
    if (protocol === 'vmess') return <VmessProtocol />
    if (protocol === 'trojan') return <TrojanProtocol />
    if (protocol === 'shadowsocks') return <ShadowsocksProtocol />
    if (protocol === 'wireguard') return <WireguardProtocol />
    if (protocol === 'hysteria') return <HysteriaProtocol />
    return <VlessProtocol />
  }

  const tabs = [
    {
      key: 'basic',
      label: t('admin.inboundEditor.tab.basic'),
      children: (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {mode === 'create' ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 4 }}
              message={
                <Space align="center" wrap>
                  <span>{t('admin.inboundEditor.fromTemplateLabel')}</span>
                  <Select
                    allowClear
                    aria-label={t('admin.inboundEditor.fromTemplateLabel')}
                    placeholder={t('admin.inboundEditor.fromTemplatePlaceholder')}
                    style={{ minWidth: 240 }}
                    value={templateID ?? undefined}
                    loading={templatesQuery.isLoading}
                    onChange={(value) => setTemplateID(typeof value === 'number' ? value : null)}
                    options={(templatesQuery.data ?? [])
                      .filter((t) => t.enabled)
                      .map((t) => ({ label: `${t.name} · ${t.protocol}`, value: t.id }))}
                  />
                </Space>
              }
              description={templateID ? t('admin.inboundEditor.fromTemplateHint') : null}
            />
          ) : null}
          <Space align="start" wrap>
            <Form.Item name="enable" label={t('admin.inboundEditor.basicEnable')} valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="node_id" label={t('admin.inboundEditor.basicNode')} rules={[{ required: true, message: t('admin.inboundEditor.errSelectNode') }]}>
              <Select
                disabled={mode === 'edit'}
                style={{ minWidth: 220 }}
                options={nodes.map((node) => ({
                  label: `${node.name}${node.enabled ? '' : ` ${t('admin.inboundEditor.nodeDisabledSuffix')}`}`,
                  value: node.id,
                  disabled: !node.enabled,
                }))}
              />
            </Form.Item>
            <Form.Item name="protocol" label={t('admin.inboundEditor.basicProtocol')} rules={[{ required: true }]}>
              <Select
                style={{ width: 180 }}
                onChange={(value) => {
                  form.setFieldValue('protocol', value)
                  setProtocol(value)
                }}
                options={['vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'hysteria'].map((value) => ({
                  label: value,
                  value,
                }))}
              />
            </Form.Item>
          </Space>
          {protocol === 'wireguard' ? <Alert type="info" showIcon message={t('admin.inboundEditor.wireguardStreamHidden')} /> : null}
          {protocol === 'hysteria' ? <Alert type="info" showIcon message={t('admin.inboundEditor.hysteriaStreamFixed')} /> : null}
          <Space align="start" wrap>
            <Form.Item name="remark" label={t('admin.inboundEditor.basicRemark')} rules={[{ required: true, whitespace: true, message: t('admin.inboundEditor.errRemark') }]}>
              <Input placeholder={t('admin.inboundEditor.basicRemarkPlaceholder')} />
            </Form.Item>
            <Form.Item name="listen" label={t('admin.inboundEditor.basicAddress')}>
              <Input placeholder={t('admin.inboundEditor.basicAddressPlaceholder')} />
            </Form.Item>
            <Form.Item
              name="port"
              label={t('admin.inboundEditor.basicPort')}
              rules={[
                { required: true, type: 'number', message: t('admin.inboundEditor.errPortRequired') },
                { type: 'number', min: 1, max: 65535, message: t('admin.inboundEditor.errPort') },
              ]}
            >
              <InputNumber precision={0} />
            </Form.Item>
            <Form.Item name="total_gb" label={t('admin.inboundEditor.basicTotalGB')}>
              <InputNumber min={0} step={0.01} />
            </Form.Item>
            <Form.Item name="trafficReset" label={t('admin.inboundEditor.basicTrafficReset')}>
              <Select
                style={{ width: 160 }}
                options={['never', 'daily', 'weekly', 'monthly', 'yearly'].map((value) => ({ label: t(`admin.inboundEditor.trafficReset.${value}`), value }))}
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
    ...(protocol === 'wireguard' || protocol === 'hysteria'
      ? []
      : [
          { key: 'stream', label: t('admin.inboundEditor.tab.stream'), children: <StreamSettingsForm /> },
          { key: 'sniffing', label: t('admin.inboundEditor.tab.sniffing'), children: <SniffingForm /> },
        ]),
    { key: 'advanced', label: t('admin.inboundEditor.tab.advanced'), children: <AdvancedJsonForm /> },
  ]

  return (
    <Drawer
      title={mode === 'create' ? t('admin.inboundEditor.createTitle') : t('admin.inboundEditor.editTitle', { tag })}
      open={open}
      width={920}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>{t('admin.inboundEditor.close')}</Button>
          <Button type="primary" loading={busy} onClick={save}>
            {mode === 'create' ? t('admin.inboundEditor.submitCreate') : t('admin.inboundEditor.submitSave')}
          </Button>
        </Space>
      }
    >
      {error ? <Alert type="error" showIcon message={t('admin.inboundEditor.operationFailed')} style={{ marginBottom: 16 }} /> : null}
      <Form
        form={form}
        layout="vertical"
        initialValues={blankInboundValues(nodeID)}
        onValuesChange={(changed) => {
          if (changed.protocol) setProtocol(changed.protocol)
        }}
      >
        <Tabs items={tabs} />
      </Form>
    </Drawer>
  )
}
