import { Navigate, Route, Routes } from 'react-router-dom'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AdminLayout, AuthLayout, PortalLayout } from './components/layout'
import AdminAuditLog from './views/admin/AuditLog'
import AdminOrders from './views/admin/Orders'
import AdminPlans from './views/admin/Plans'
import AdminProvisioningPools from './views/admin/ProvisioningPools'
import Login from './views/Login'
import NotFound from './views/NotFound'
import OIDCCallback from './views/OIDCCallback'
import PortalOrders from './views/portal/Orders'
import PortalPlans from './views/portal/Plans'
import Subscription from './views/portal/Subscription'
import Usage from './views/portal/Usage'

interface PlaceholderViewProps {
  title: string
}

function PlaceholderView({ title }: PlaceholderViewProps) {
  return (
    <section aria-label={title} style={{ padding: 24 }}>
      <h1>{title}</h1>
    </section>
  )
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
      <Route path="/" element={<Navigate replace to="/admin" />} />
      <Route element={<ProtectedRoute area="admin" />}>
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<Navigate replace to="/admin/status" />} />
          <Route path="status" element={<PlaceholderView title="Admin Status" />} />
          <Route path="stats" element={<PlaceholderView title="Admin Stats" />} />
          <Route path="ops-monitor" element={<PlaceholderView title="Ops Monitor" />} />
          <Route path="nodes" element={<PlaceholderView title="Nodes" />} />
          <Route path="inbounds" element={<PlaceholderView title="Inbounds" />} />
          <Route path="users" element={<PlaceholderView title="Users" />} />
          <Route path="plans" element={<AdminPlans />} />
          <Route path="provisioning-pools" element={<AdminProvisioningPools />} />
          <Route path="orders" element={<AdminOrders />} />
          <Route path="audit-log" element={<AdminAuditLog />} />
          <Route path="settings" element={<PlaceholderView title="Settings" />} />
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
          <Route path="profile" element={<PlaceholderView title="Profile" />} />
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
