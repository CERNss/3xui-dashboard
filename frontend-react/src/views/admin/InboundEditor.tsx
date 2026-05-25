import { Alert, Button, Drawer, Form, Input, InputNumber, Select, Space, Switch, Tabs } from 'antd'
import { useEffect, useState } from 'react'
import { useCreateInbound, useUpdateInbound } from '@/hooks/queries/admin/inbounds'
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

function ProtocolFields({ protocol }: { protocol: ProtocolName }) {
  if (protocol === 'vmess') return <VmessProtocol />
  if (protocol === 'trojan') return <TrojanProtocol />
  if (protocol === 'shadowsocks') return <ShadowsocksProtocol />
  if (protocol === 'wireguard') return <WireguardProtocol />
  if (protocol === 'hysteria') return <HysteriaProtocol />
  return <VlessProtocol />
}

export default function InboundEditor({ open, mode, nodeID, tag, source, nodes, onClose, onSaved }: InboundEditorProps) {
  const [form] = Form.useForm<InboundEditorValues>()
  const [protocol, setProtocol] = useState<ProtocolName>('vless')
  const createInbound = useCreateInbound()
  const updateInbound = useUpdateInbound()
  const busy = createInbound.isPending || updateInbound.isPending
  const error = createInbound.error ?? updateInbound.error

  useEffect(() => {
    if (!open) return
    const values = source && mode === 'edit' ? inboundToValues(source, nodeID) : blankInboundValues(nodeID)
    form.setFieldsValue(values as unknown as Parameters<typeof form.setFieldsValue>[0])
    setProtocol(values.protocol)
  }, [form, mode, nodeID, open, source])

  const save = async () => {
    const validated = await form.validateFields().catch(() => null)
    if (typeof validated?.node_id !== 'number') return
    const values = { ...form.getFieldsValue(true), ...validated } as InboundEditorValues
    values.node_id = validated.node_id
    const body = valuesToInboundBody(values)
    const result =
      mode === 'create'
        ? await createInbound.mutateAsync({ nodeID: validated.node_id, body })
        : await updateInbound.mutateAsync({ nodeID: validated.node_id, tag, body })
    onSaved?.(result)
    onClose()
  }

  const tabs = [
    {
      key: 'basic',
      label: 'Basic',
      children: (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Space align="start" wrap>
            <Form.Item name="enable" label="Enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="node_id" label="Node" rules={[{ required: true, message: 'Node is required' }]}>
              <Select
                disabled={mode === 'edit'}
                style={{ minWidth: 220 }}
                options={nodes.map((node) => ({
                  label: `${node.name}${node.enabled ? '' : ' (disabled)'}`,
                  value: node.id,
                  disabled: !node.enabled,
                }))}
              />
            </Form.Item>
            <Form.Item name="protocol" label="Protocol" rules={[{ required: true }]}>
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
          {protocol === 'wireguard' ? <Alert type="info" showIcon message="WireGuard hides Stream and Sniffing because those settings do not apply." /> : null}
          {protocol === 'hysteria' ? <Alert type="info" showIcon message="Hysteria uses mandatory TLS and fixed hysteria stream settings." /> : null}
          <Space align="start" wrap>
            <Form.Item name="remark" label="Remark" rules={[{ required: true, whitespace: true, message: 'Remark is required' }]}>
              <Input placeholder="inbound remark" />
            </Form.Item>
            <Form.Item name="listen" label="Listen address">
              <Input placeholder="0.0.0.0 or empty" />
            </Form.Item>
            <Form.Item
              name="port"
              label="Port"
              rules={[
                { required: true, type: 'number', message: 'Port is required' },
                { type: 'number', min: 1, max: 65535, message: 'Port must be between 1 and 65535' },
              ]}
            >
              <InputNumber precision={0} />
            </Form.Item>
            <Form.Item name="total_gb" label="Traffic GB">
              <InputNumber min={0} step={0.01} />
            </Form.Item>
            <Form.Item name="trafficReset" label="Traffic reset">
              <Select
                style={{ width: 160 }}
                options={['never', 'daily', 'weekly', 'monthly', 'yearly'].map((value) => ({ label: value, value }))}
              />
            </Form.Item>
            <Form.Item name="expiryTime" label="Expiry">
              <Input type="datetime-local" />
            </Form.Item>
          </Space>
        </Space>
      ),
    },
    { key: 'protocol', label: 'Protocol', children: <ProtocolFields protocol={protocol} /> },
    ...(protocol === 'wireguard' || protocol === 'hysteria'
      ? []
      : [
          { key: 'stream', label: 'Stream', children: <StreamSettingsForm /> },
          { key: 'sniffing', label: 'Sniffing', children: <SniffingForm /> },
        ]),
    { key: 'advanced', label: 'Advanced', children: <AdvancedJsonForm /> },
  ]

  return (
    <Drawer
      title={mode === 'create' ? 'New Inbound' : `Edit inbound ${tag}`}
      open={open}
      width={920}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>Close</Button>
          <Button type="primary" loading={busy} onClick={save}>
            {mode === 'create' ? 'Create' : 'Save'}
          </Button>
        </Space>
      }
    >
      {error ? <Alert type="error" showIcon message="Inbound operation failed" style={{ marginBottom: 16 }} /> : null}
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
