import { Button, Card, Form, Input, InputNumber, Space, Switch } from 'antd'
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import type { ReactNode } from 'react'

interface FieldConfig {
  name: string
  label: string
  placeholder?: string
  numeric?: boolean
  switch?: boolean
  defaultValue?: unknown
}

interface ProtocolClientsProps {
  title: string
  name?: string
  fields: FieldConfig[]
  addLabel?: string
  children?: ReactNode
}

export function ProtocolClients({ title, name = 'clients', fields, addLabel = 'Add client', children }: ProtocolClientsProps) {
  const initialValue = fields.reduce<Record<string, unknown>>((acc, field) => {
    acc[field.name] = field.defaultValue ?? (field.switch ? true : field.numeric ? 0 : '')
    return acc
  }, {})

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      {children}
      <Form.List name={name}>
        {(items, { add, remove }) => (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Space style={{ justifyContent: 'space-between', width: '100%' }}>
              <strong>{title}</strong>
              <Button size="small" icon={<PlusOutlined />} onClick={() => add(initialValue)}>
                {addLabel}
              </Button>
            </Space>
            {items.length === 0 ? <Card size="small">No clients configured.</Card> : null}
            {items.map((item) => (
              <Card
                key={item.key}
                size="small"
                title={`Client ${item.name + 1}`}
                extra={<Button danger size="small" icon={<MinusCircleOutlined />} onClick={() => remove(item.name)} />}
              >
                <Space align="start" wrap>
                  {fields.map((field) => (
                    <Form.Item
                      key={field.name}
                      name={[item.name, field.name]}
                      label={field.label}
                      valuePropName={field.switch ? 'checked' : undefined}
                    >
                      {field.switch ? (
                        <Switch />
                      ) : field.numeric ? (
                        <InputNumber min={0} />
                      ) : (
                        <Input placeholder={field.placeholder} />
                      )}
                    </Form.Item>
                  ))}
                </Space>
              </Card>
            ))}
          </Space>
        )}
      </Form.List>
    </Space>
  )
}
