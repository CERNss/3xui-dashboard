# unified-login

The single `/login` SPA entry for administrator password login and portal OIDC
start.

## Purpose & Boundaries

This module defines the frontend route and navigation rules for the shared
auth entry. It does not define token issuance or credential verification:

- Admin password authentication is owned by `admin-auth`.
- Portal account APIs are owned by `user-accounts`.
- OIDC provider discovery, start, callback, and completion are owned by
  `oidc-providers`.
- Email verification mechanics are owned by `email-verification`.

## Frontend Route Topology

Defined in `frontend/src/router.tsx`:

```
/                 -> redirect /admin
/login            -> AuthLayout + Login
/oidc/callback    -> AuthLayout + OIDCCallback
/admin/*          -> ProtectedRoute(area="admin") + AdminLayout
/portal/*         -> ProtectedRoute(area="portal") + PortalLayout
*                 -> AuthLayout + NotFound
```

## Requirements

### Requirement: Login Route Uses Admin Password Form

The system SHALL provide `/login` as the only password-login page in the SPA,
and that form SHALL target admin authentication.

#### Scenario: Admin credentials accepted

- **WHEN** the user submits username and password on `/login`
- **THEN** `Login.tsx` SHALL call the admin login mutation
- **AND** on success SHALL store the admin token in `useAdminAuthStore`
- **AND** navigate to the safe admin `next` path or `/admin/status`.

#### Scenario: Admin login fails

- **WHEN** the admin login mutation rejects
- **THEN** the page SHALL render the formatted error inline
- **AND** SHALL start a short retry cooldown before another submit is accepted.

### Requirement: Login Route Renders OIDC Provider Buttons

The login page SHALL also expose configured portal SSO providers below the
admin password form.

#### Scenario: Providers hidden when unavailable

- **GIVEN** `GET /api/user/auth/oidc/providers` returns `[]` or fails
- **WHEN** `/login` renders
- **THEN** no SSO divider or provider buttons SHALL be shown.

#### Scenario: Provider starts portal SSO

- **GIVEN** at least one OIDC provider is returned
- **WHEN** the user clicks a provider button
- **THEN** `Login.tsx` SHALL call the OIDC start mutation with the provider key
- **AND** navigate the browser to the returned `authorize_url`.

### Requirement: `next` Is Sanitized Per Auth Area

The system SHALL accept only local paths in `next` query parameters and SHALL
not let one auth area redirect into the other through SSO.

#### Scenario: Admin next path

- **GIVEN** `/login?next=/admin/users`
- **WHEN** admin password login succeeds
- **THEN** the SPA SHALL navigate to `/admin/users`.

#### Scenario: Unsafe admin next path

- **GIVEN** `/login?next=https://evil.example/path`
- **WHEN** admin password login succeeds
- **THEN** the SPA SHALL navigate to `/admin/status`.

#### Scenario: OIDC next path

- **GIVEN** `/login?next=/portal/orders`
- **WHEN** OIDC start runs
- **THEN** `redirect_after` SHALL be `/portal/orders`.

#### Scenario: Admin path is not passed to OIDC

- **GIVEN** `/login?next=/admin/status`
- **WHEN** OIDC start runs
- **THEN** `redirect_after` SHALL fall back to `/portal/subscription`.

### Requirement: Old Role-Specific Auth Entry Routes Are Absent

The SPA SHALL NOT register separate role-specific auth entry routes.

#### Scenario: Old admin login URL

- **WHEN** the browser opens `/admin/login`
- **THEN** the route SHALL resolve through the admin route tree and render NotFound when unauthenticated handling does not intercept it.

#### Scenario: Old portal auth URLs

- **WHEN** the browser opens `/portal/login` or `/portal/register`
- **THEN** the route SHALL not render a dedicated login or registration page.

### Requirement: Admin And Portal Sessions Remain Independent

The frontend SHALL store admin and portal sessions in separate Zustand stores.

#### Scenario: Portal OIDC does not clear admin session

- **GIVEN** an admin token exists in `useAdminAuthStore`
- **WHEN** a portal OIDC callback stores a portal token
- **THEN** the admin token SHALL remain available until explicit admin logout or expiry.

#### Scenario: Admin login does not clear portal session

- **GIVEN** a portal token exists in `usePortalAuthStore`
- **WHEN** admin password login succeeds
- **THEN** the portal token SHALL remain available until explicit portal logout or expiry.

## Out of Scope

- Portal password login UI.
- Portal self-registration UI.
- OIDC callback account completion UI, covered by `oidc-providers`.
