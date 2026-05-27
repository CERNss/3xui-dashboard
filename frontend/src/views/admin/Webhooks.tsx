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
import { useTranslation } from 'react-i18next'
import { adminWebhooksApi, type Webhook, type WebhookDelivery, type WebhookInput, type WebhookMethod } from '@/api/admin/webhooks'
import { ConfigListPage, RefreshButton } from '@/components/common'
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

function deliveryStatusLabel(status: string, t: ReturnType<typeof useTranslation>['t']) {
  return t(`admin.webhooks.deliveryStatus.${status}`, { defaultValue: t('admin.webhooks.deliveryStatus.unknown', { status }) })
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
  const { t } = useTranslation()

  if (loading) return <Typography.Text type="secondary">{t('admin.webhooks.deliveriesLoading')}</Typography.Text>
  if (deliveries.length === 0) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('admin.webhooks.deliveriesEmpty')} />

  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      {deliveries.map((delivery) => (
        <Card key={delivery.id} size="small" styles={{ body: { padding: 12 } }}>
          <Space align="start" style={{ justifyContent: 'space-between', width: '100%' }}>
            <Space direction="vertical" size={2}>
              <Space wrap>
                <Tag color={statusColor(delivery.status)}>{deliveryStatusLabel(delivery.status, t)}</Tag>
                <Typography.Text code>{delivery.event_type}</Typography.Text>
                <Typography.Text type="secondary">
                  {t('admin.webhooks.deliveryMeta', { attempt: delivery.attempt, status: delivery.http_status || '-' })}
                </Typography.Text>
              </Space>
              <Typography.Text type="secondary">{new Date(delivery.scheduled_at).toLocaleString()}</Typography.Text>
              {delivery.error ? <Typography.Text type="danger">{t('admin.webhooks.deliveryError', { error: delivery.error })}</Typography.Text> : null}
            </Space>
            <Button
              aria-label={t('admin.webhooks.replay')}
              size="small"
              icon={<RetweetOutlined />}
              loading={replaying === delivery.id}
              onClick={() => onReplay(delivery)}
            >
              {t('admin.webhooks.replay')}
            </Button>
          </Space>
        </Card>
      ))}
    </Space>
  )
}

export default function Webhooks({ embedded = false }: WebhooksProps) {
  const { t } = useTranslation()
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
          errors: [err instanceof Error ? err.message : t('admin.webhooks.invalidHeaders')],
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
    message.success(t('admin.webhooks.testSent'))
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
      message.success(t('admin.webhooks.replayQueued'))
    } finally {
      setReplayingID(undefined)
    }
  }

  const confirmDelete = (webhook: Webhook) => {
    Modal.confirm({
      title: t('admin.webhooks.confirmDelete'),
      content: t('admin.webhooks.confirmDeleteMsg', { name: webhook.name }),
      okText: t('admin.webhooks.delete'),
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
      title: t('admin.webhooks.column.name'),
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
      title: t('admin.webhooks.column.method'),
      dataIndex: 'method',
      align: 'center',
      width: 100,
      render: (method: WebhookMethod) => <Tag color={methodColor(method)}>{method}</Tag>,
    },
    {
      title: t('admin.webhooks.column.events'),
      dataIndex: 'events',
      render: (events: string[]) => <Typography.Text code>{events.join(', ') || '*'}</Typography.Text>,
    },
    {
      title: t('admin.webhooks.column.template'),
      key: 'template',
      render: (_value, webhook) =>
        webhook.body_template
          ? t('admin.webhooks.templateCustom', { format: webhook.template_format, n: webhook.body_template.length })
          : t('admin.webhooks.templateDefault', { format: webhook.template_format }),
    },
    {
      title: t('admin.webhooks.column.status'),
      dataIndex: 'enabled',
      align: 'center',
      width: 110,
      render: (enabled: boolean) => <Tag color={enabled ? 'green' : 'default'}>{enabled ? t('admin.webhooks.enabled') : t('admin.webhooks.disabled')}</Tag>,
    },
    {
      title: t('admin.webhooks.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 260,
      render: (_value, webhook) => (
        <Space wrap>
          <Button size="small" onClick={() => toggleDeliveries(webhook)}>
            {expandedID === webhook.id ? t('admin.webhooks.collapse') : t('admin.webhooks.deliveries')}
          </Button>
          <Button
            aria-label={t('admin.webhooks.test')}
            size="small"
            icon={<ExperimentOutlined />}
            loading={testWebhook.isPending}
            onClick={() => fireTest(webhook)}
          >
            {t('admin.webhooks.test')}
          </Button>
          <Button aria-label={t('admin.webhooks.editNamed', { name: webhook.name })} size="small" icon={<EditOutlined />} onClick={() => openEdit(webhook)} />
          <Button
            aria-label={t('admin.webhooks.deleteNamed', { name: webhook.name })}
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
      <ConfigListPage
        title={embedded ? undefined : t('admin.webhooks.title')}
        subtitle={embedded ? undefined : t('admin.webhooks.subtitle')}
        actions={
          embedded ? undefined : (
            <>
              <Button type="primary" aria-label={t('admin.webhooks.createNew')} icon={<PlusOutlined />} onClick={openCreate}>
                {t('admin.webhooks.createNew')}
              </Button>
              <RefreshButton loading={webhooksQuery.isFetching || deliveriesQuery.isFetching} onClick={refresh} />
            </>
          )
        }
        header={
          embedded ? (
            <div style={{ alignItems: 'center', display: 'flex', justifyContent: 'space-between' }}>
              <div>
                <Typography.Title level={4} style={{ margin: 0 }}>
                  {t('admin.webhooks.title')}
                </Typography.Title>
                <Typography.Text type="secondary">{t('admin.webhooks.subtitle')}</Typography.Text>
              </div>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                {t('admin.webhooks.createNew')}
              </Button>
            </div>
          ) : undefined
        }
        alerts={error ? <Alert type="error" showIcon message={t('admin.webhooks.operationFailed')} /> : null}
        viewport={!embedded}
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
        emptyState={{
          title: t('admin.webhooks.empty'),
          description: t('admin.webhooks.emptyDescription'),
        }}
        mobileCard={(webhook) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>{webhook.name}</Typography.Text>
                <Tag color={webhook.enabled ? 'green' : 'default'}>
                  {webhook.enabled ? t('admin.webhooks.enabled') : t('admin.webhooks.disabled')}
                </Tag>
              </Space>
              <Typography.Text code>{webhook.url}</Typography.Text>
              <Descriptions size="small" column={1}>
                <Descriptions.Item label={t('admin.webhooks.column.method')}>{webhook.method}</Descriptions.Item>
                <Descriptions.Item label={t('admin.webhooks.column.events')}>{webhook.events.join(', ') || '*'}</Descriptions.Item>
              </Descriptions>
              <Space wrap>
                <Button size="small" onClick={() => toggleDeliveries(webhook)}>
                  {t('admin.webhooks.deliveries')}
                </Button>
                <Button aria-label={t('admin.webhooks.test')} size="small" icon={<ExperimentOutlined />} onClick={() => fireTest(webhook)}>
                  {t('admin.webhooks.test')}
                </Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(webhook)}>
                  {t('admin.webhooks.edit')}
                </Button>
                <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(webhook)}>
                  {t('admin.webhooks.delete')}
                </Button>
              </Space>
            </Space>
          </Card>
        )}
      />

      <Drawer
        title={editing ? t('admin.webhooks.editTitle', { id: editing.id }) : t('admin.webhooks.createTitle')}
        open={drawerOpen}
        width={720}
        destroyOnClose
        onClose={closeDrawer}
        extra={
          <Space>
            <Button onClick={closeDrawer}>{t('admin.webhooks.cancel')}</Button>
            <Button type="primary" loading={saving} onClick={submit}>
              {t('admin.webhooks.save')}
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical" initialValues={inputToForm(blankWebhookInput())}>
          <Form.Item name="name" label={t('admin.webhooks.name')} rules={[{ required: true, message: t('admin.webhooks.nameRequired') }]}>
            <Input />
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true, type: 'url', message: t('admin.webhooks.urlRequired') }]}>
            <Input placeholder={t('admin.webhooks.urlPlaceholder')} />
          </Form.Item>
          <Space style={{ width: '100%' }} align="start">
            <Form.Item name="method" label={t('admin.webhooks.column.method')} style={{ width: 180 }}>
              <Select
                options={(['POST', 'GET', 'PUT', 'DELETE', 'PATCH'] as WebhookMethod[]).map((value) => ({
                  label: value,
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item name="template_format" label={t('admin.webhooks.bodyFormat')} style={{ width: 180 }}>
              <Select
                options={['json', 'form', 'text', 'raw'].map((value) => ({
                  label: t(`admin.webhooks.bodyFormatOpts.${value}`),
                  value,
                }))}
              />
            </Form.Item>
          </Space>
          <Form.Item name="events_text" label={t('admin.webhooks.eventsLabel')} extra={t('admin.webhooks.eventsCommon')}>
            <Input placeholder={t('admin.webhooks.eventsPlaceholder')} />
          </Form.Item>
          <Form.Item name="body_template" label={t('admin.webhooks.body')} extra={t('admin.webhooks.bodyVars')}>
            <Input.TextArea rows={6} placeholder={t('admin.webhooks.bodyPlaceholder')} />
          </Form.Item>
          <Form.Item name="headers_text" label={t('admin.webhooks.headersLabel')}>
            <Input.TextArea rows={4} placeholder={t('admin.webhooks.headersPlaceholder')} />
          </Form.Item>
          <Space size={24}>
            <Form.Item name="enabled" label={t('admin.webhooks.enabledCheckbox')} valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="allow_private" label={t('admin.webhooks.privateAllowed')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Drawer>
    </div>
  )
}
