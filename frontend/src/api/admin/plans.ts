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
  enabled: boolean
}

export type UpdatePlanInput = Partial<CreatePlanInput>

export const adminPlansApi = {
  /** Lists ALL plans including disabled. */
  list: () => adminClient.get<{ plans: AdminPlan[] }>('/plans').then((r) => r.data.plans),

  create: (input: CreatePlanInput) =>
    adminClient.post<AdminPlan>('/plans', input).then((r) => r.data),

  update: (id: number, input: UpdatePlanInput) =>
    adminClient.put<AdminPlan>(`/plans/${id}`, input).then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/plans/${id}`),
}
