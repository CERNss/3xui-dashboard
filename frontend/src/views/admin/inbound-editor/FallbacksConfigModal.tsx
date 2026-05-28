import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space } from 'antd'
import { useTranslation } from 'react-i18next'

interface FallbacksConfigModalProps {
  open: boolean
  onClose: () => void
}

export function FallbacksConfigModal({ open, onClose }: FallbacksConfigModalProps) {
  const { t } = useTranslation()
  return (
    <Modal
      title={t('admin.inboundEditor.fallbacks.configure')}
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>{t('admin.inboundEditor.stream.done')}</Button>}
      width={720}
      destroyOnClose={false}
      maskClosable
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert type="info" showIcon message={t('admin.inboundEditor.fallbacks.hint')} />
        <Form.List name="fallbacks">
          {(items, { add, remove }) => (
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <strong>{t('admin.inboundEditor.fallbacks.title')}</strong>
                <Button
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={() =>
                    add({ name: '', alpn: '', path: '', dest: '', xver: 0 })
                  }
                >
                  {t('admin.inboundEditor.fallbacks.add')}
                </Button>
              </Space>
              {items.length === 0 ? (
                <Card size="small">{t('admin.inboundEditor.fallbacks.empty')}</Card>
              ) : null}
              {items.map((item) => (
                <Card
                  key={item.key}
                  size="small"
                  title={t('admin.inboundEditor.fallbacks.itemTitle', { n: item.name + 1 })}
                  extra={
                    <Button danger size="small" icon={<MinusCircleOutlined />} onClick={() => remove(item.name)} />
                  }
                >
                  <Space align="start" wrap size={[12, 0]}>
                    <Form.Item name={[item.name, 'name']} label="SNI">
                      <Input style={{ width: 200 }} placeholder={t('admin.inboundEditor.fallbacks.sniPlaceholder')} />
                    </Form.Item>
                    <Form.Item name={[item.name, 'alpn']} label="ALPN">
                      <Select
                        style={{ width: 140 }}
                        options={[
                          { label: 'any', value: '' },
                          { label: 'h2', value: 'h2' },
                          { label: 'http/1.1', value: 'http/1.1' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item name={[item.name, 'path']} label="Path">
                      <Input style={{ width: 200 }} placeholder="/ws" />
                    </Form.Item>
                    <Form.Item name={[item.name, 'dest']} label="Dest" required>
                      <Input style={{ width: 220 }} placeholder="127.0.0.1:8080" />
                    </Form.Item>
                    <Form.Item name={[item.name, 'xver']} label="xver">
                      <InputNumber min={0} max={2} style={{ width: 80 }} />
                    </Form.Item>
                  </Space>
                </Card>
              ))}
            </Space>
          )}
        </Form.List>
      </Space>
    </Modal>
  )
}
