import { adminClient } from '../client/admin'

export interface ProvisioningPoolTarget {
  id: number
  pool_id: number
  node_id: number
  node_name?: string
  inbound_tag: string
  protocol: string
  max_clients: number
  used_clients?: number
  priority: number
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface ProvisioningPool {
  id: number
  name: string
  description?: string
  enabled: boolean
  allowed_protocols: string[]
  targets?: ProvisioningPoolTarget[]
  created_at?: string
  updated_at?: string
}

export interface ProvisioningPoolInput {
  name: string
  description?: string
  enabled: boolean
  allowed_protocols: string[]
}

export interface ProvisioningPoolTargetInput {
  node_id: number
  inbound_tag: string
  protocol?: string
  max_clients: number
  priority: number
  enabled: boolean
}

export const provisioningPoolsApi = {
  list: () =>
    adminClient.get<{ pools: ProvisioningPool[] }>('/provisioning-pools').then((r) => r.data.pools),

  create: (input: ProvisioningPoolInput) =>
    adminClient.post<ProvisioningPool>('/provisioning-pools', input).then((r) => r.data),

  update: (id: number, input: Partial<ProvisioningPoolInput>) =>
    adminClient.put<ProvisioningPool>(`/provisioning-pools/${id}`, input).then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/provisioning-pools/${id}`),

  addTarget: (poolID: number, input: ProvisioningPoolTargetInput) =>
    adminClient.post<ProvisioningPoolTarget>(`/provisioning-pools/${poolID}/targets`, input).then((r) => r.data),

  updateTarget: (targetID: number, input: Partial<ProvisioningPoolTargetInput>) =>
    adminClient.put<void>(`/provisioning-pools/targets/${targetID}`, input).then((r) => r.data),

  removeTarget: (targetID: number) =>
    adminClient.delete<void>(`/provisioning-pools/targets/${targetID}`).then((r) => r.data),
}
