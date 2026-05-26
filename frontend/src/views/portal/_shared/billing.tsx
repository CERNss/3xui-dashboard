import { Tag } from 'antd'
import type { ReactNode } from 'react'
import type { Order, PaymentMethod } from '@/api/portal/billing'

const BYTES_PER_GB = 1024 * 1024 * 1024

export function formatYuan(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

export function formatTraffic(bytes: number): string {
  if (bytes === 0) return '∞'

  const gb = bytes / BYTES_PER_GB
  if (gb >= 1024) return `${(gb / 1024).toFixed(1)} TB`
  if (gb >= 1) return `${Math.round(gb)} GB`
  return `${Math.round(bytes / (1024 * 1024))} MB`
}

export function paymentMethodLabel(method: PaymentMethod, labels: Record<PaymentMethod, string>): string {
  return labels[method] ?? method
}

export function orderStatusColor(status: Order['status']): string {
  switch (status) {
    case 'completed':
    case 'paid':
      return 'green'
    case 'failed':
    case 'payment_failed':
      return 'red'
    case 'payment_expired':
      return 'default'
    case 'refunded':
      return 'gold'
    case 'payment_pending':
      return 'orange'
    case 'pending':
    default:
      return 'blue'
  }
}

export function isPaidOrder(status: Order['status']): boolean {
  return status === 'completed' || status === 'paid'
}

export function canContinuePayment(order: Order): boolean {
  return order.status === 'payment_pending' && (order.payment_method === 'alipay' || order.payment_method === 'stripe')
}

export function OrderStatusTag({
  status,
  label,
}: {
  status: Order['status']
  label: ReactNode
}) {
  return <Tag color={orderStatusColor(status)}>{label}</Tag>
}
