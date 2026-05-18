// ---- Auth & Users ----

export interface User {
  id: number
  username: string
  email: string
  role: 'admin' | 'user'
  xuiClientEmail: string
  xuiSubId: string
  balance: number
  createdAt: string
  updatedAt: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface LoginPayload {
  username: string
  password: string
}

export interface RegisterPayload {
  username: string
  email: string
  password: string
}

// ---- 3x-ui Data Types ----

export interface ClientTraffic {
  id: number
  inboundId: number
  enable: boolean
  email: string
  up: number
  down: number
  expiryTime: number
  total: number
  reset: number
}

export interface Client {
  id?: string
  email: string
  enable: boolean
  expiryTime: number
  flow?: string
  limitIp: number
  totalGB: number
  subId: string
  tgId?: number
  comment?: string
  reset?: number
}

export interface Inbound {
  id: number
  userId: number
  up: number
  down: number
  total: number
  remark: string
  enable: boolean
  expiryTime: number
  port: number
  protocol: string
  settings: string
  streamSettings: string
  tag: string
  sniffing: string
  listen?: string
  nodeId?: number
  clientStats?: ClientTraffic[]
}

export interface Node {
  id: number
  name: string
  remark: string
  scheme: string
  address: string
  port: number
  basePath: string
  enable: boolean
  status: 'online' | 'offline' | 'unknown'
  lastHeartbeat: number
  latencyMs: number
  xrayVersion: string
  cpuPct: number
  memPct: number
  uptimeSecs: number
  lastError: string
  createdAt: number
  updatedAt: number
}

// ---- Stats ----

export interface AdminStats {
  totalClients: number
  activeClients: number
  totalUp: number
  totalDown: number
  nodesOnline: number
}

// ---- Subscription ----

export interface SubscriptionInfo {
  subId: string
  subUrl: string
  email: string
}

// ---- API Generic Response ----

export interface ApiError {
  error: string
}

// ---- UI Types ----

export type ToastType = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
  id: number
  type: ToastType
  message: string
}
