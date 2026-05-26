import {
  DeleteOutlined,
  EditOutlined,
  ExperimentOutlined,
  PlusOutlined,
  RetweetOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { adminWebhooksApi, type Webhook, type WebhookDelivery, type WebhookInput, type WebhookMethod } from '@/api/admin/webhooks'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import {
  useCreateWebhook,
  useRemoveWebhook,
  useReplayWebhookDelivery,
  useTestWebhook,
  useUpdateWebhook,
  useWebhookDeliveries,
  useWebhooksList,
} from '@/hooks/queries/admin/webhooks'
import {
  blankWebhookInput,
  eventsToText,
  headersToText,
  textToEvents,
  textToHeaders,
  webhookToInput,
} from './webhooks/parse'

interface WebhookFormValues extends Omit<WebhookInput, 'events' | 'headers'> {
  events_text: string
  headers_text: string
}

export interface WebhooksProps {
  embedded?: boolean
}

function inputToForm(input: WebhookInput): WebhookFormValues {
  return {
    ...input,
    events_text: eventsToText(input.events),
    headers_text: headersToText(input.headers),
  }
}

function formToInput(values: WebhookFormValues): WebhookInput {
  return {
    name: values.name.trim(),
    url: values.url.trim(),
    events: textToEvents(values.events_text),
    enabled: values.enabled,
    allow_private: values.allow_private,
    method: values.method,
    headers: textToHeaders(values.headers_text),
    body_template: values.body_template ?? '',
    template_format: values.template_format,
    secret: values.secret?.trim() || undefined,
  }
}

function methodColor(method: WebhookMethod) {
  return {
    DELETE: 'red',
    GET: 'blue',
    PATCH: 'purple',
    POST: 'green',
    PUT: 'gold',
  }[method]
}

function statusColor(status: string) {
  if (status === 'success') return 'green'
  if (status === 'failed') return 'red'
  return 'gold'
}

function DeliveryList({
  deliveries,
  loading,
  replaying,
  onReplay,
}: {
  deliveries: WebhookDelivery[]
  loading: boolean
  replaying?: number
  onReplay: (delivery: WebhookDelivery) => void
}) {
  if (loading) return <Typography.Text type="secondary">Loading deliveries...</Typography.Text>
  if (deliveries.length === 0) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No deliveries yet" />

  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      {deliveries.map((delivery) => (
        <Card key={delivery.id} size="small" styles={{ body: { padding: 12 } }}>
          <Space align="start" style={{ justifyContent: 'space-between', width: '100%' }}>
            <Space direction="vertical" size={2}>
              <Space wrap>
                <Tag color={statusColor(delivery.status)}>{delivery.status}</Tag>
                <Typography.Text code>{delivery.event_type}</Typography.Text>
                <Typography.Text type="secondary">
                  attempt {delivery.attempt} · HTTP {delivery.http_status || '-'}
                </Typography.Text>
              </Space>
              <Typography.Text type="secondary">{new Date(delivery.scheduled_at).toLocaleString()}</Typography.Text>
              {delivery.error ? <Typography.Text type="danger">err: {delivery.error}</Typography.Text> : null}
            </Space>
            <Button
              aria-label="Replay"
              size="small"
              icon={<RetweetOutlined />}
              loading={replaying === delivery.id}
              onClick={() => onReplay(delivery)}
            >
              Replay
            </Button>
          </Space>
        </Card>
      ))}
    </Space>
  )
}

export default function Webhooks({ embedded = false }: WebhooksProps) {
  const [form] = Form.useForm<WebhookFormValues>()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Webhook | null>(null)
  const [expandedID, setExpandedID] = useState<number | null>(null)
  const [deliveryCache, setDeliveryCache] = useState<Record<number, WebhookDelivery[]>>({})
  const [replayingID, setReplayingID] = useState<number>()

  const webhooksQuery = useWebhooksList()
  const deliveriesQuery = useWebhookDeliveries(expandedID ?? 0, expandedID !== null && !deliveryCache[expandedID])
  const createWebhook = useCreateWebhook()
  const updateWebhook = useUpdateWebhook()
  const removeWebhook = useRemoveWebhook()
  const testWebhook = useTestWebhook()
  const replayDelivery = useReplayWebhookDelivery()

  const webhooks = webhooksQuery.data ?? []
  const saving = createWebhook.isPending || updateWebhook.isPending
  const error =
    webhooksQuery.error ??
    createWebhook.error ??
    updateWebhook.error ??
    removeWebhook.error ??
    testWebhook.error ??
    replayDelivery.error

  const visibleDeliveries = expandedID ? deliveryCache[expandedID] ?? deliveriesQuery.data ?? [] : []

  useEffect(() => {
    if (expandedID && deliveriesQuery.data && !deliveryCache[expandedID]) {
      setDeliveryCache((current) => ({ ...current, [expandedID]: deliveriesQuery.data ?? [] }))
    }
  }, [deliveriesQuery.data, deliveryCache, expandedID])

  const openCreate = () => {
    setEditing(null)
    form.setFieldsValue(inputToForm(blankWebhookInput()))
    setDrawerOpen(true)
  }

  const openEdit = (webhook: Webhook) => {
    setEditing(webhook)
    form.setFieldsValue(inputToForm(webhookToInput(webhook)))
    setDrawerOpen(true)
  }

  const closeDrawer = () => {
    setDrawerOpen(false)
    setEditing(null)
    form.resetFields()
  }

  const submit = async () => {
    const values = await form.validateFields().catch(() => null)
    if (!values) return

    let payload: WebhookInput
    try {
      payload = formToInput(values)
    } catch (err) {
      form.setFields([
        {
          name: 'headers_text',
          errors: [err instanceof Error ? err.message : 'Invalid headers'],
        },
      ])
      return
    }

    if (editing) {
      await updateWebhook.mutateAsync({ id: editing.id, patch: payload })
    } else {
      await createWebhook.mutateAsync(payload)
    }
    closeDrawer()
  }

  const toggleDeliveries = (webhook: Webhook) => {
    setExpandedID((current) => (current === webhook.id ? null : webhook.id))
  }

  const refreshDeliveries = async (webhookID: number) => {
    const rows = await adminWebhooksApi.deliveries(webhookID)
    setDeliveryCache((current) => ({ ...current, [webhookID]: rows }))
  }

  const fireTest = async (webhook: Webhook) => {
    await testWebhook.mutateAsync(webhook.id)
    setExpandedID(webhook.id)
    setDeliveryCache((current) => {
      const next = { ...current }
      delete next[webhook.id]
      return next
    })
    await refreshDeliveries(webhook.id)
    message.success('Test delivery queued')
  }

  const replay = async (webhookID: number, delivery: WebhookDelivery) => {
    setReplayingID(delivery.id)
    try {
      await replayDelivery.mutateAsync(delivery.id)
      setDeliveryCache((current) => {
        const next = { ...current }
        delete next[webhookID]
        return next
      })
      await refreshDeliveries(webhookID)
      message.success('Delivery replay queued')
    } finally {
      setReplayingID(undefined)
    }
  }

  const confirmDelete = (webhook: Webhook) => {
    Modal.confirm({
      title: 'Delete webhook',
      content: `Delete ${webhook.name}? Delivery history will remain as audit trail.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => removeWebhook.mutateAsync(webhook.id),
    })
  }

  const refresh = () => {
    webhooksQuery.refetch()
    if (expandedID) {
      setDeliveryCache((current) => {
        const next = { ...current }
        delete next[expandedID]
        return next
      })
      deliveriesQuery.refetch()
    }
  }

  const columns: ColumnsType<Webhook> = [
    {
      title: 'Name',
      dataIndex: 'name',
      render: (_value, webhook) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{webhook.name}</Typography.Text>
          <Typography.Text code copyable>
            {webhook.url}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Method',
      dataIndex: 'method',
      width: 100,
      render: (method: WebhookMethod) => <Tag color={methodColor(method)}>{method}</Tag>,
    },
    {
      title: 'Events',
      dataIndex: 'events',
      render: (events: string[]) => <Typography.Text code>{events.join(', ') || '*'}</Typography.Text>,
    },
    {
      title: 'Template',
      key: 'template',
      render: (_value, webhook) =>
        webhook.body_template ? `${webhook.template_format} · ${webhook.body_template.length} chars` : `default (${webhook.template_format})`,
    },
    {
      title: 'Status',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <Tag color={enabled ? 'green' : 'default'}>{enabled ? 'Enabled' : 'Disabled'}</Tag>,
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, webhook) => (
        <Space wrap>
          <Button size="small" onClick={() => toggleDeliveries(webhook)}>
            {expandedID === webhook.id ? 'Collapse' : 'Deliveries'}
          </Button>
          <Button
            aria-label="Test"
            size="small"
            icon={<ExperimentOutlined />}
            loading={testWebhook.isPending}
            onClick={() => fireTest(webhook)}
          >
            Test
          </Button>
          <Button aria-label={`Edit ${webhook.name}`} size="small" icon={<EditOutlined />} onClick={() => openEdit(webhook)} />
          <Button
            aria-label={`Delete ${webhook.name}`}
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(webhook)}
          />
        </Space>
      ),
    },
  ]

  return (
    <div>
      {embedded ? (
        <div style={{ alignItems: 'center', display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
          <div>
            <Typography.Title level={4} style={{ margin: 0 }}>
              Webhooks
            </Typography.Title>
            <Typography.Text type="secondary">Send admin events to external systems.</Typography.Text>
          </div>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            New Webhook
          </Button>
        </div>
      ) : (
        <PageHeader
          title="Webhooks"
          subtitle="Send admin events to external systems."
          actions={
            <>
              <Button type="primary" aria-label="New Webhook" icon={<PlusOutlined />} onClick={openCreate}>
                New Webhook
              </Button>
              <RefreshButton loading={webhooksQuery.isFetching || deliveriesQuery.isFetching} onClick={refresh} />
            </>
          }
        />
      )}

      {error ? <Alert type="error" showIcon message="Webhook operation failed" style={{ marginBottom: 16 }} /> : null}

      {webhooks.length > 0 || webhooksQuery.isLoading ? (
        <ResponsiveListTable
          rowKey="id"
          columns={columns}
          dataSource={webhooks}
          expandable={{
            expandedRowKeys: expandedID ? [expandedID] : [],
            expandedRowRender: (webhook) => (
              <DeliveryList
                deliveries={expandedID === webhook.id ? visibleDeliveries : []}
                loading={deliveriesQuery.isFetching && expandedID === webhook.id && !deliveryCache[webhook.id]}
                replaying={replayingID}
                onReplay={(delivery) => replay(webhook.id, delivery)}
              />
            ),
            rowExpandable: (webhook) => expandedID === webhook.id,
            showExpandColumn: false,
          }}
          loading={webhooksQuery.isLoading}
          pagination={false}
          mobileCard={(webhook) => (
            <Card size="small" style={{ width: '100%' }}>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                  <Typography.Text strong>{webhook.name}</Typography.Text>
                  <Tag color={webhook.enabled ? 'green' : 'default'}>{webhook.enabled ? 'Enabled' : 'Disabled'}</Tag>
                </Space>
                <Typography.Text code>{webhook.url}</Typography.Text>
                <Descriptions size="small" column={1}>
                  <Descriptions.Item label="Method">{webhook.method}</Descriptions.Item>
                  <Descriptions.Item label="Events">{webhook.events.join(', ') || '*'}</Descriptions.Item>
                </Descriptions>
                <Space wrap>
                  <Button size="small" onClick={() => toggleDeliveries(webhook)}>
                    Deliveries
                  </Button>
                  <Button aria-label="Test" size="small" icon={<ExperimentOutlined />} onClick={() => fireTest(webhook)}>
                    Test
                  </Button>
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(webhook)}>
                    Edit
                  </Button>
                  <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(webhook)}>
                    Delete
                  </Button>
                </Space>
              </Space>
            </Card>
          )}
        />
      ) : (
        <EmptyState title="No webhooks" description="Create a webhook to send events to external receivers." />
      )}

      <Drawer
        title={editing ? `Edit webhook #${editing.id}` : 'Create webhook'}
        open={drawerOpen}
        width={720}
        destroyOnClose
        onClose={closeDrawer}
        extra={
          <Space>
            <Button onClick={closeDrawer}>Cancel</Button>
            <Button type="primary" loading={saving} onClick={submit}>
              Save
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical" initialValues={inputToForm(blankWebhookInput())}>
          <Form.Item name="name" label="Name" rules={[{ required: true, message: 'Name is required' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true, type: 'url', message: 'Valid URL is required' }]}>
            <Input />
          </Form.Item>
          <Space style={{ width: '100%' }} align="start">
            <Form.Item name="method" label="Method" style={{ width: 180 }}>
              <Select
                options={(['POST', 'GET', 'PUT', 'DELETE', 'PATCH'] as WebhookMethod[]).map((value) => ({
                  label: value,
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item name="template_format" label="Body format" style={{ width: 180 }}>
              <Select
                options={['json', 'form', 'text', 'raw'].map((value) => ({
                  label: value,
                  value,
                }))}
              />
            </Form.Item>
          </Space>
          <Form.Item name="events_text" label="Events">
            <Input placeholder="*, user.created, order.paid" />
          </Form.Item>
          <Form.Item name="body_template" label="Body template">
            <Input.TextArea rows={6} />
          </Form.Item>
          <Form.Item name="headers_text" label="Headers">
            <Input.TextArea rows={4} placeholder="Authorization: Bearer token" />
          </Form.Item>
          <Space size={24}>
            <Form.Item name="enabled" label="Enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="allow_private" label="Allow private addresses" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Drawer>
    </div>
  )
}
