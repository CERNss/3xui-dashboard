import { adminClient } from '../client/admin'

export interface AdminPlan {
  id: number
  name: string
  description?: string
  duration_days: number
  /** Bytes. 0 = unlimited. */
  traffic_limit_bytes: number
  price_cents: number
  ip_limit?: number
  provisioning_pool_id?: number | null
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface CreatePlanInput {
  name: string
  description?: string
  duration_days: number
  traffic_limit_bytes: number
  price_cents: number
  ip_limit?: number
  provisioning_pool_id?: number | null
  enabled: boolean
}

export type UpdatePlanInput = Partial<CreatePlanInput>

/** What a plan→subscriber mirror pass did (backend SyncSummary). */
export interface PlanSyncSummary {
  plan_id: number
  users: number
  refreshed: number
  added: number
  removed: number
  errors?: string[]
}

/**
 * PUT /plans/:id saves AND immediately mirrors the plan onto its
 * subscribers; `sync` reports the pass, `sync_error` is set when the
 * save landed but the mirror pass itself failed (retry via sync()).
 */
export interface UpdatePlanResult {
  plan: AdminPlan
  sync?: PlanSyncSummary
  sync_error?: string
}

export const adminPlansApi = {
  /** Lists ALL plans including disabled. */
  list: () => adminClient.get<{ plans: AdminPlan[] }>('/plans').then((r) => r.data.plans),

  create: (input: CreatePlanInput) =>
    adminClient.post<AdminPlan>('/plans', input).then((r) => r.data),

  update: (id: number, input: UpdatePlanInput) =>
    adminClient.put<UpdatePlanResult>(`/plans/${id}`, input).then((r) => r.data),

  /** Manual re-run of the subscriber mirror (retry after node outages). */
  sync: (id: number) =>
    adminClient.post<PlanSyncSummary>(`/plans/${id}/sync`).then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/plans/${id}`),
}
