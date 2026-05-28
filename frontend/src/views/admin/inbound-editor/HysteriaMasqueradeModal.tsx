import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { Button, Form, Input, InputNumber, Modal, Radio, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

interface HysteriaMasqueradeModalProps {
  open: boolean
  onClose: () => void
}

export function HysteriaMasqueradeModal({ open, onClose }: HysteriaMasqueradeModalProps) {
  const { t } = useTranslation()
  const masqType = Form.useWatch<'' | 'proxy' | 'file' | 'string'>('hysteriaMasqueradeType')

  return (
    <Modal
      title={t('admin.inboundEditor.hysteriaMasquerade.configure')}
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>{t('admin.inboundEditor.stream.done')}</Button>}
      width={640}
      destroyOnClose={false}
      maskClosable
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Form.Item name="hysteriaMasqueradeType" label={t('admin.inboundEditor.hysteriaMasquerade.type')}>
          <Radio.Group
            optionType="button"
            buttonStyle="solid"
            options={[
              { label: t('admin.inboundEditor.hysteriaMasquerade.none'), value: '' },
              { label: 'Proxy', value: 'proxy' },
              { label: 'File', value: 'file' },
              { label: 'String', value: 'string' },
            ]}
          />
        </Form.Item>

        {masqType === 'proxy' ? (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Form.Item name="hysteriaMasqueradeProxyUrl" label="URL">
              <Input placeholder="https://example.com" />
            </Form.Item>
            <Space align="start" wrap>
              <Form.Item name="hysteriaMasqueradeProxyRewriteHost" label={t('admin.inboundEditor.hysteriaMasquerade.rewriteHost')} valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name="hysteriaMasqueradeProxyInsecure" label={t('admin.inboundEditor.hysteriaMasquerade.insecure')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Space>
          </Space>
        ) : null}

        {masqType === 'file' ? (
          <Form.Item name="hysteriaMasqueradeFileDir" label={t('admin.inboundEditor.hysteriaMasquerade.dir')}>
            <Input placeholder="/path/to/www" />
          </Form.Item>
        ) : null}

        {masqType === 'string' ? (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Form.Item name="hysteriaMasqueradeStringStatusCode" label={t('admin.inboundEditor.hysteriaMasquerade.statusCode')}>
              <InputNumber min={100} max={599} />
            </Form.Item>
            <Form.Item name="hysteriaMasqueradeStringContent" label={t('admin.inboundEditor.hysteriaMasquerade.content')}>
              <Input.TextArea rows={4} placeholder="Hello, world!" />
            </Form.Item>
            <Form.List name="hysteriaMasqueradeStringHeaders">
              {(items, { add, remove }) => (
                <Space direction="vertical" size={6} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <strong>{t('admin.inboundEditor.hysteriaMasquerade.headers')}</strong>
                    <Button size="small" icon={<PlusOutlined />} onClick={() => add({ key: '', value: '' })}>
                      {t('admin.inboundEditor.hysteriaMasquerade.addHeader')}
                    </Button>
                  </Space>
                  {items.map((item) => (
                    <Space key={item.key} align="baseline">
                      <Form.Item name={[item.name, 'key']} noStyle>
                        <Input placeholder="Content-Type" style={{ width: 200 }} />
                      </Form.Item>
                      <Form.Item name={[item.name, 'value']} noStyle>
                        <Input placeholder="text/html" style={{ width: 260 }} />
                      </Form.Item>
                      <Button danger size="small" icon={<MinusCircleOutlined />} onClick={() => remove(item.name)} />
                    </Space>
                  ))}
                </Space>
              )}
            </Form.List>
          </Space>
        ) : null}
      </Space>
    </Modal>
  )
}
