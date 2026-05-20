import { adminClient } from '../client/admin'

export type UserStatus = 'active' | 'suspended'

export interface AdminUser {
  id: number
  email?: string | null
  email_verified: boolean
  status: UserStatus
  balance_cents: number
  sub_id: string
  oidc_subject?: string | null
  created_at: string
  updated_at: string
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

  update: (id: number, fields: Partial<Pick<AdminUser, 'email' | 'status'>>) =>
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
