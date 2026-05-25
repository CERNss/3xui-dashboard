import { Checkbox, Form, Space, Switch } from 'antd'

export function SniffingForm() {
  const enabled = Form.useWatch('sniffEnabled')
  return (
    <Space direction="vertical" size={12}>
      <Form.Item name="sniffEnabled" label="Sniffing enabled" valuePropName="checked">
        <Switch />
      </Form.Item>
      {enabled ? (
        <>
          <Space wrap>
            <Form.Item name="sniffHttp" valuePropName="checked">
              <Checkbox>http</Checkbox>
            </Form.Item>
            <Form.Item name="sniffTls" valuePropName="checked">
              <Checkbox>tls</Checkbox>
            </Form.Item>
            <Form.Item name="sniffQuic" valuePropName="checked">
              <Checkbox>quic</Checkbox>
            </Form.Item>
            <Form.Item name="sniffFakedns" valuePropName="checked">
              <Checkbox>fakedns</Checkbox>
            </Form.Item>
          </Space>
          <Space wrap>
            <Form.Item name="sniffMetadataOnly" label="Metadata only" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="sniffRouteOnly" label="Route only" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </>
      ) : null}
    </Space>
  )
}
