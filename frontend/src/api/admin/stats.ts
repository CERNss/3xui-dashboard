import { adminClient } from '../client/admin'

// Wire shape mirrors backend/internal/handler/admin/stats.go.
// Keep field names in lockstep — JSON tags on the Go side are
// snake_case; the frontend reads exactly those keys.

export interface UserStats {
  total: number
  active: number
  suspended: number
  month_new: number
  prev_month_new: number
  total_balance_cents: number
  avg_balance_cents: number
}

export interface TrafficStats {
  month_up_bytes: number
  month_down_bytes: number
  today_up_bytes: number
  today_down_bytes: number
}

export interface TrafficRanking {
  key: string
  up_bytes: number
  down_bytes: number
  bytes: number
}

export interface AuditSeverity {
  info: number
  warn: number
  err: number
}

export interface PlanStats {
  total: number
  enabled: number
  disabled: number
}

export interface OrderStats {
  total: number
  completed: number
  failed: number
  refunded: number
  revenue_cents: number
  month_count: number
  month_revenue_cents: number
}

export interface RecentOrder {
  id: number
  user_id: number
  user_email: string
  plan_id: number
  plan_name: string
  price_cents: number
  status: string
  created_at: string
}

export interface AdminStats {
  users: UserStats
  plans: PlanStats
  orders: OrderStats
  traffic: TrafficStats
  top_nodes: TrafficRanking[]
  top_users: TrafficRanking[]
  audit: AuditSeverity
  recent_orders: RecentOrder[]
}

export const adminStatsApi = {
  async get(): Promise<AdminStats> {
    const { data } = await adminClient.get<AdminStats>('/stats')
    return data
  },
}
