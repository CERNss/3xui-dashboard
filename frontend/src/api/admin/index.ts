// Admin API surface. Feature modules (nodes, inbounds, clients,
// traffic, users, plans, orders, webhooks, settings) land under this
// directory in later task groups. Keep this index file as the single
// re-export point so views import from '@/api/admin'.

export { adminClient, ADMIN_TOKEN_KEY } from '../client/admin'
export * from './inboundTemplates'
export * from './utils'
