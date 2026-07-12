import { ThunderboltOutlined } from '@ant-design/icons'
import { Button, Space } from 'antd'
import { subscriptionUrl } from './subscriptionFormats'

interface ImportButtonsProps {
  baseUrl: string
  subId: string
  /** Profile name shown inside the client app (branding title). */
  name: string
}

/**
 * One-click deep links that hand the subscription URL straight to an
 * installed client app — no copy/paste. Each app gets the format it
 * natively consumes, independent of the format selected on the page.
 */
export function ImportButtons({ baseUrl, subId, name }: ImportButtonsProps) {
  const enc = encodeURIComponent
  const links = [
    {
      key: 'clash',
      label: 'Clash / Mihomo',
      href: `clash://install-config?url=${enc(subscriptionUrl(baseUrl, subId, 'clash'))}&name=${enc(name)}`,
    },
    {
      key: 'singbox',
      label: 'sing-box',
      href: `sing-box://import-remote-profile?url=${enc(subscriptionUrl(baseUrl, subId, 'singbox'))}#${enc(name)}`,
    },
    {
      key: 'shadowrocket',
      label: 'Shadowrocket',
      // Shadowrocket takes the base64 subscription URL wrapped in its
      // sub:// pseudo-scheme.
      href: `shadowrocket://add/sub://${btoa(subscriptionUrl(baseUrl, subId, 'base64')).replace(/=+$/, '')}?remark=${enc(name)}`,
    },
  ]
  return (
    <Space wrap>
      {links.map((link) => (
        <Button key={link.key} icon={<ThunderboltOutlined />} href={link.href}>
          {link.label}
        </Button>
      ))}
    </Space>
  )
}
