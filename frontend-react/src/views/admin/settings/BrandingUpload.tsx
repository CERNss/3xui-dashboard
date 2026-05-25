import { UploadOutlined } from '@ant-design/icons'
import { Button, Card, Space, Typography, message } from 'antd'
import { useRef, useState } from 'react'
import { useUploadBrandIcon } from '@/hooks/queries/admin/settings'

export function BrandingUpload() {
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const upload = useUploadBrandIcon()

  const submit = async () => {
    if (!file) return
    await upload.mutateAsync(file)
    setFile(null)
    if (inputRef.current) inputRef.current.value = ''
    message.success('Brand icon uploaded')
  }

  return (
    <Card title="Brand icon">
      <Space direction="vertical" size={12}>
        <Typography.Text type="secondary">Upload a favicon or app icon used by the admin shell.</Typography.Text>
        <input
          ref={inputRef}
          aria-label="Brand icon file"
          accept="image/png,image/jpeg,image/webp,image/svg+xml"
          type="file"
          onChange={(event) => setFile(event.target.files?.[0] ?? null)}
        />
        <Button aria-label="Upload favicon" type="primary" icon={<UploadOutlined />} disabled={!file} loading={upload.isPending} onClick={submit}>
          Upload favicon
        </Button>
      </Space>
    </Card>
  )
}
