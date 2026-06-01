import { useEffect, useMemo } from 'react'
import type { Order } from '@/api/portal/billing'
import type { ClientUsage } from '@/api/portal/traffic'

const RECENT_ORDER_WINDOW_MS = 5 * 60_000
const PROVISIONING_POLL_INTERVAL_MS = 2_000
const PROVISIONING_POLL_TIMEOUT_MS = 45_000

export function hasRecentProvisioningOrder(orders: Order[], now = Date.now()) {
  return orders.some((order) => {
    if (order.status !== 'completed' && order.status !== 'paid') return false
    const timestamp = order.completed_at ?? order.created_at
    const time = new Date(timestamp).getTime()
    if (!Number.isFinite(time)) return false
    return now - time >= 0 && now - time <= RECENT_ORDER_WINDOW_MS
  })
}

export function useProvisioningStatus({
  clients,
  orders,
  ordersFetching,
  trafficFetching,
  refetchOrders,
  refetchTraffic,
}: {
  clients: ClientUsage[]
  orders?: Order[]
  ordersFetching?: boolean
  trafficFetching?: boolean
  refetchOrders?: () => Promise<unknown>
  refetchTraffic: () => Promise<unknown>
}) {
  const isProvisioning = useMemo(
    () => clients.length === 0 && hasRecentProvisioningOrder(orders ?? []),
    [clients.length, orders],
  )

  useEffect(() => {
    if (!isProvisioning) return undefined

    const startedAt = Date.now()
    const timer = window.setInterval(() => {
      if (Date.now() - startedAt > PROVISIONING_POLL_TIMEOUT_MS) {
        window.clearInterval(timer)
        return
      }
      void refetchTraffic()
      void refetchOrders?.()
    }, PROVISIONING_POLL_INTERVAL_MS)

    return () => window.clearInterval(timer)
  }, [isProvisioning, refetchOrders, refetchTraffic])

  return {
    isProvisioning,
    isRefreshingProvisioning: Boolean(isProvisioning && (ordersFetching || trafficFetching)),
  }
}
