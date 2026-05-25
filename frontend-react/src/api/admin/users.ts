import { adminClient } from '../client/admin'

export type UserStatus = 'active' | 'suspended'

export interface AdminUser {
  id: number
  email?: string | null
  email_verified: boolean
  status: UserStatus
  balance_cents: number
  auto_renew: boolean
  sub_id: string
  oidc_subject?: string | null
  created_at: string
  updated_at: string
  // Optional — backend may not have wired this column yet.
  // The Users page renders "—" when undefined.
  last_active_at?: string | null
}

export interface ListUsersResponse {
  users: AdminUser[]
  limit: number
  offset: number
}

export const adminUsersApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminClient
      .get<ListUsersResponse>('/users', { params })
      .then((r) => r.data),

  get: (id: number) =>
    adminClient.get<AdminUser>(`/users/${id}`).then((r) => r.data),

  // Create a brand-new user. Backend assigns id + sub_id; the optional
  // initial_balance_cents is recorded as a balance_log entry so the
  // audit trail captures "where this money came from".
  // Errors: 400 (invalid input) / 409 (email already exists).
  create: (body: { email: string; password: string; initial_balance_cents?: number }) =>
    adminClient.post<AdminUser>('/users', body).then((r) => r.data),

  update: (id: number, fields: Partial<Pick<AdminUser, 'email' | 'email_verified' | 'status' | 'auto_renew' | 'balance_cents'>> & { password?: string }) =>
    adminClient.put<AdminUser>(`/users/${id}`, fields).then((r) => r.data),

  suspend: (id: number) =>
    adminClient.post<{ id: number; status: UserStatus }>(`/users/${id}/suspend`),

  unsuspend: (id: number) =>
    adminClient.post<{ id: number; status: UserStatus }>(`/users/${id}/unsuspend`),

  /** Adjusts balance by `delta_cents` (positive credits, negative debits). */
  adjustBalance: (id: number, deltaCents: number, reason: string) =>
    adminClient
      .post<{ id: number; balance_cents: number }>(`/users/${id}/balance`, {
        delta_cents: deltaCents,
        reason,
      })
      .then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/users/${id}`),
}
