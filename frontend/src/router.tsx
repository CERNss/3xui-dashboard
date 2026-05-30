import { Navigate, Route, Routes } from 'react-router-dom'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AdminLayout, AuthLayout, PortalLayout } from './components/layout'
import { useAdminAuthStore } from './stores/adminAuth'
import { usePortalAuthStore } from './stores/portalAuth'
import AdminAuditLog from './views/admin/AuditLog'
import AdminClients from './views/admin/Clients'
import AdminInbounds from './views/admin/Inbounds'
import AdminInboundTemplates from './views/admin/InboundTemplates'
import AdminNodes from './views/admin/Nodes'
import AdminOpsMonitor from './views/admin/OpsMonitor'
import AdminOrders from './views/admin/Orders'
import AdminOverview from './views/admin/Overview'
import AdminPlans from './views/admin/Plans'
import AdminProvisioningPools from './views/admin/ProvisioningPools'
import AdminSettings from './views/admin/Settings'
import AdminUsers from './views/admin/Users'
import AdminWebhooks from './views/admin/Webhooks'
import Login from './views/Login'
import NotFound from './views/NotFound'
import OIDCCallback from './views/OIDCCallback'
import PortalOrders from './views/portal/Orders'
import PortalPlans from './views/portal/Plans'
import PortalProfile from './views/portal/Profile'
import Subscription from './views/portal/Subscription'
import Usage from './views/portal/Usage'

/**
 * RootRedirect picks the destination for `/` based on which session
 * is currently persisted in localStorage. Hard-coding `/admin` like
 * the previous version did meant a portal user who came back to the
 * site root got bounced through ProtectedRoute(admin) to /login even
 * though their portal session was still valid.
 */
function RootRedirect() {
  const adminToken = useAdminAuthStore((s) => s.token)
  const portalToken = usePortalAuthStore((s) => s.token)
  if (adminToken) return <Navigate replace to="/admin" />
  if (portalToken) return <Navigate replace to="/portal" />
  return <Navigate replace to="/login" />
}

export function AppRouter() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <AuthLayout>
            <Login />
          </AuthLayout>
        }
      />
      <Route
        path="/oidc/callback"
        element={
          <AuthLayout>
            <OIDCCallback />
          </AuthLayout>
        }
      />
      <Route path="/" element={<RootRedirect />} />
      <Route element={<ProtectedRoute area="admin" />}>
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<Navigate replace to="/admin/status" />} />
          <Route path="status" element={<AdminOverview />} />
          <Route path="stats" element={<Navigate replace to="/admin/status?tab=stats" />} />
          <Route path="ops-monitor" element={<AdminOpsMonitor />} />
          <Route path="nodes" element={<AdminNodes />} />
          <Route path="inbounds" element={<AdminInbounds />} />
          <Route path="inbound-templates" element={<AdminInboundTemplates />} />
          <Route path="clients" element={<AdminClients />} />
          <Route path="users" element={<AdminUsers />} />
          <Route path="plans" element={<AdminPlans />} />
          <Route path="provisioning-pools" element={<AdminProvisioningPools />} />
          <Route path="orders" element={<AdminOrders />} />
          <Route path="audit-log" element={<AdminAuditLog />} />
          <Route path="webhooks" element={<AdminWebhooks />} />
          <Route path="settings" element={<AdminSettings />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
      <Route element={<ProtectedRoute area="portal" />}>
        <Route path="/portal" element={<PortalLayout />}>
          <Route index element={<Navigate replace to="/portal/subscription" />} />
          <Route path="subscription" element={<Subscription />} />
          <Route path="usage" element={<Usage />} />
          <Route path="plans" element={<PortalPlans />} />
          <Route path="orders" element={<PortalOrders />} />
          <Route path="profile" element={<PortalProfile />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
      <Route
        path="*"
        element={
          <AuthLayout>
            <NotFound />
          </AuthLayout>
        }
      />
    </Routes>
  )
}
