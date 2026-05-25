import { adminClient } from '../client/admin'

export interface AdminAction {
  id: number
  admin_username: string
  method: string
  path: string
  target_resource: string
  target_id: string
  query_string?: string
  ip: string
  user_agent?: string
  status_code: number
  error_msg?: string
  created_at: string
}

export interface ListAuditParams {
  username?: string
  resource?: string
  id?: string
  method?: string
  limit?: number
  offset?: number
}

export const adminAuditApi = {
  list: (params?: ListAuditParams) =>
    adminClient
      .get<{ actions: AdminAction[]; limit: number; offset: number }>('/audit-log', { params })
      .then((r) => r.data),
}
