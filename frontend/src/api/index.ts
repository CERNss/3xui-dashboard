import axios, { type AxiosInstance } from 'axios'
import type {
  AuthResponse,
  LoginPayload,
  RegisterPayload,
  User,
  AdminStats,
  Inbound,
  Node,
  SubscriptionInfo,
  ClientTraffic
} from '@/types'

// ---- Axios instance ----

const api: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
})

// Attach JWT on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// On 401, clear token and redirect to login
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

// ---- Auth ----

export const authApi = {
  login: (payload: LoginPayload) =>
    api.post<AuthResponse>('/auth/login', payload).then((r) => r.data),
  register: (payload: RegisterPayload) =>
    api.post<AuthResponse>('/auth/register', payload).then((r) => r.data)
}

// ---- Admin: Stats ----

export const adminStatsApi = {
  get: () => api.get<AdminStats>('/admin/stats').then((r) => r.data)
}

// ---- Admin: Inbounds ----

export const adminInboundsApi = {
  list: () => api.get<Inbound[]>('/admin/inbounds').then((r) => r.data),
  create: (payload: Partial<Inbound>) =>
    api.post<Inbound>('/admin/inbounds', payload).then((r) => r.data),
  update: (id: number, payload: Partial<Inbound>) =>
    api.put<Inbound>(`/admin/inbounds/${id}`, payload).then((r) => r.data),
  delete: (id: number) => api.delete(`/admin/inbounds/${id}`)
}

// ---- Admin: Clients ----

export interface AddClientPayload {
  id: number
  settings: { clients: object[] }
}

export const adminClientsApi = {
  list: () => api.get<Inbound[]>('/admin/clients').then((r) => r.data),
  create: (payload: AddClientPayload) =>
    api.post('/admin/clients', payload).then((r) => r.data),
  update: (uuid: string, payload: object) =>
    api.put(`/admin/clients/${uuid}`, payload).then((r) => r.data),
  delete: (uuid: string) => api.delete(`/admin/clients/${uuid}`)
}

// ---- Admin: Nodes ----

export const adminNodesApi = {
  list: () => api.get<Node[]>('/admin/nodes').then((r) => r.data)
}

// ---- Admin: Users ----

export interface UpdateUserPayload {
  username: string
  email: string
  role: 'admin' | 'user'
  xuiClientEmail: string
  xuiSubId: string
}

export const adminUsersApi = {
  list: () => api.get<User[]>('/admin/users').then((r) => r.data),
  update: (id: number, payload: UpdateUserPayload) =>
    api.put<User>(`/admin/users/${id}`, payload).then((r) => r.data),
  delete: (id: number) => api.delete(`/admin/users/${id}`)
}

// ---- User: Profile ----

export const userProfileApi = {
  get: () => api.get<User>('/user/profile').then((r) => r.data),
  update: (payload: { email: string }) =>
    api.put<User>('/user/profile', payload).then((r) => r.data),
  changePassword: (payload: { oldPassword: string; newPassword: string }) =>
    api.post('/user/change-password', payload).then((r) => r.data)
}

// ---- User: Subscription ----

export const userSubscriptionApi = {
  get: () => api.get<SubscriptionInfo>('/user/subscription').then((r) => r.data)
}

// ---- User: Traffic ----

export const userTrafficApi = {
  get: () => api.get<ClientTraffic>('/user/traffic').then((r) => r.data)
}

export default api
