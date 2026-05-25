import { adminClient } from '../client/admin'

export type OrderStatus = 'created' | 'paid' | 'completed' | 'failed' | 'refunded'

export interface AdminOrder {
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

export interface ListOrdersParams {
  limit?: number
  offset?: number
  user_id?: number
  status?: OrderStatus
}

export interface ListOrdersResponse {
  orders: AdminOrder[]
  limit: number
  offset: number
}

export const adminOrdersApi = {
  list: (params?: ListOrdersParams) =>
    adminClient.get<ListOrdersResponse>('/orders', { params }).then((r) => r.data),

  refund: (id: number, reason: string) =>
    adminClient.post<AdminOrder>(`/orders/${id}/refund`, { reason }).then((r) => r.data),
}
