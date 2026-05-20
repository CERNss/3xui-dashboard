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

export type OrderStatus =
  | 'pending'
  | 'completed'
  | 'failed'
  | 'refunded'
  | 'payment_pending'
  | 'paid'
  | 'payment_failed'
  | 'payment_expired'

export type PaymentMethod = 'balance' | 'alipay' | 'stripe'

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
  payment_method: PaymentMethod
  payment_provider_order_id?: string
  /** Gateway-agnostic "send the user here to complete payment" URL.
   *  alipay → QR-source URL (frontend renders as QR).
   *  stripe → Checkout redirect URL (frontend navigates via location.href). */
  payment_target_url?: string
  payment_expires_at?: string | null
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

  /** Configured payment methods. Always includes "balance"; alipay
   *  appears only when ALIPAY_APP_ID/PRIVATE_KEY/PUBLIC_KEY are set. */
  paymentMethods: () =>
    portalClient.get<{ methods: PaymentMethod[] }>('/payment-methods').then((r) => r.data.methods),

  /** Buy via a payment gateway (alipay / stripe). Returns the order
   *  with payment_target_url + payment_expires_at populated; the
   *  portal either renders a QR (alipay) or redirects (stripe). */
  purchaseViaPayment: (provider: PaymentMethod, input: PurchaseInput) =>
    portalClient.post<Order>(`/purchase/${provider}`, input).then((r) => r.data),

  /** Poll one order — used by the alipay QR modal to flip to "支付成功"
   *  when the notify endpoint advances the order. */
  getOrder: (id: number) =>
    portalClient.get<Order>(`/orders/${id}`).then((r) => r.data),
}
