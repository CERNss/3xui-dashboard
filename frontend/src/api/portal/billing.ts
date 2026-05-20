import { portalClient } from '../client/portal'

export interface Plan {
  id: number
  name: string
  description?: string
  price_cents: number
  /** Backend ships bytes; frontend converts to GB for display. 0 = unlimited. */
  traffic_limit_bytes: number
  duration_days: number
  ip_limit?: number
  enabled: boolean
  created_at?: string
}

export type OrderStatus = 'created' | 'paid' | 'completed' | 'failed' | 'refunded'

export interface Order {
  id: number
  user_id: number
  plan_id: number
  idempotency_key: string
  price_cents: number
  status: OrderStatus
  client_ownership_id?: number
  error_message?: string
  created_at: string
  completed_at?: string | null
}

export interface PurchaseInput {
  plan_id: number
  /** RFC 4122 UUID — same key on retries deduplicates server-side. */
  idempotency_key: string
  /** Which node + inbound to provision the new client on. Backend
   *  requires both — sspanel-style "user picks where", not auto-pick. */
  node_id: number
  inbound_tag: string
}

/** PortalInbound is one user-purchasable inbound surfaced by
 *  GET /api/user/inbounds. Backend filters to enabled-only and strips
 *  admin-only fields (settings JSON, traffic, etc.). */
export interface PortalInbound {
  node_id: number
  node_name: string
  inbound_tag: string
  protocol: string
  remark: string
  port: number
}

export const portalBillingApi = {
  /** Public plan catalog — only enabled plans. */
  listPlans: () =>
    portalClient.get<{ plans: Plan[] }>('/plans').then((r) => r.data.plans),

  /** Caller's own order history, newest first. */
  listOrders: () =>
    portalClient.get<{ orders: Order[] }>('/orders').then((r) => r.data.orders),

  /**
   * Buy a plan. The idempotency key SHOULD be a freshly generated UUID
   * — the server uses it to dedupe a double-clicked purchase across
   * retries. Returns the resulting order (may be the original on a
   * repeated key).
   */
  purchase: (input: PurchaseInput) =>
    portalClient.post<Order>('/purchase', input).then((r) => r.data),

  /** List inbounds the user may purchase a plan onto. */
  listInbounds: () =>
    portalClient.get<{ inbounds: PortalInbound[] }>('/inbounds').then((r) => r.data.inbounds),
}
