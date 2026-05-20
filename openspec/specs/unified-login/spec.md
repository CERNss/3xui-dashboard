# unified-login

A single `/login` route handles both admin and portal sign-in, plus
self-serve portal registration, in one consistent SPA chrome.

## Purpose & boundaries

Operators and portal users share the same email-shaped account format,
so we cannot route by URL alone (the historical `/admin/login` vs
`/portal/login` split required users to know their role in advance).
This module defines:

- The unified `/login` SPA route (frontend).
- The auto-detection logic that picks which auth endpoint to call.
- The login/register tab switch on the same page.
- Legacy URL redirects so old bookmarks keep working.

Token issuance and credential checking are out of scope — those live
in `admin-auth`, `user-accounts`, and (for registration) `email-verification`.

## Frontend route topology

Defined in `frontend/src/router/index.ts`:

```
/                          → redirect /admin
/login                     → views/Login.vue
/admin/login               → redirect /login?hint=admin (preserves ?next)
/portal/login              → redirect /login?hint=portal
/admin/*                   → AdminLayout, meta.requiresAdmin
/portal/*                  → PortalLayout, meta.requiresUser
```

The 401 axios interceptor (`api/client/factory.ts`) redirects to
`/login` directly (no hint), since unified login auto-detects.

## Requirements

### Requirement: Single login route serves both roles

The system SHALL provide a unified `/login` SPA route that authenticates
users into either the admin console or the portal based solely on the
credentials presented.

#### Scenario: Admin email + password lands the user in /admin

- **GIVEN** the operator has configured `ADMIN_USERNAME=alice@example.com` and a known `ADMIN_PASSWORD`
- **WHEN** the user submits the login form with `alice@example.com` and that password
- **THEN** the SPA SHALL store the admin token in `adminAuthStore`
- **AND** navigate to `/admin`
- **AND** SHALL NOT call the portal login endpoint

#### Scenario: Portal user falls back after admin 401

- **GIVEN** the user `bob@example.com` is a portal user, not the admin
- **WHEN** the user submits the login form
- **THEN** the SPA SHALL attempt `POST /api/admin/auth/login` first
- **AND** on a 400/401/403/404 response, SHALL attempt `POST /api/user/auth/login` with the same credentials
- **AND** on success, store the user token in `portalAuthStore` and navigate to `/portal`

#### Scenario: Both endpoints reject the credentials

- **WHEN** neither admin nor portal recognizes the credentials
- **THEN** the SPA SHALL display "邮箱或密码错误"
- **AND** SHALL NOT redirect or clear the form fields

#### Scenario: Network or 5xx aborts the fallback

- **WHEN** the first attempt yields a network error or HTTP 5xx
- **THEN** the SPA SHALL NOT fall back to the second endpoint
- **AND** SHALL display the message returned by `utils/format.ts::formatError(e)`

### Requirement: Login/register tabs share the same view

The system SHALL expose a tab control at the top of `views/Login.vue`
that toggles between "登录" and "注册" without leaving the route.

#### Scenario: User switches from 登录 to 注册

- **WHEN** the user clicks the 注册 tab
- **THEN** the form SHALL show additional fields: 确认密码, verification code
- **AND** clear any prior error state
- **AND** clear the verification code field (do not preserve across mode switches)

#### Scenario: Registration mode never targets admin

- **WHEN** the user submits the form in 注册 mode
- **THEN** the SPA SHALL call `POST /api/user/auth/register` exclusively
- **AND** SHALL NOT attempt admin auth — admin is bootstrapped via env, not self-served

#### Scenario: Initial mode honors ?mode=register

- **WHEN** the route is loaded with `?mode=register`
- **THEN** the 注册 tab SHALL be active on first render

### Requirement: `?next=` is honored when role matches

The system SHALL respect a `next` query parameter after successful login,
but ONLY when it points into the area the authenticated role can access.

#### Scenario: Admin redirected back to /admin/inbounds after 401

- **GIVEN** an admin's session expired while viewing `/admin/inbounds`
- **WHEN** the axios interceptor redirects to `/login?next=%2Fadmin%2Finbounds`
- **AND** the user re-authenticates as admin
- **THEN** the SPA SHALL navigate to `/admin/inbounds`

#### Scenario: Portal user blocked from admin next

- **GIVEN** the URL is `/login?next=%2Fadmin%2Fstatus`
- **WHEN** the user logs in with portal credentials (admin attempt 401, portal succeeds)
- **THEN** the SPA SHALL navigate to `/portal` (not `/admin/status`)
- **AND** SHALL NOT raise an error — silently sidesteps the mismatched next

### Requirement: Legacy login URLs preserved as redirects

The system SHALL preserve the old `/admin/login` and `/portal/login`
URLs as router-level redirects so existing bookmarks keep working.

#### Scenario: Old admin bookmark

- **WHEN** a request lands on `/admin/login?next=/admin/nodes`
- **THEN** the router SHALL redirect to `/login?next=/admin/nodes&hint=admin`

#### Scenario: Old portal bookmark

- **WHEN** a request lands on `/portal/login`
- **THEN** the router SHALL redirect to `/login?hint=portal`

### Requirement: Independent admin and portal sessions

The system SHALL store admin and portal tokens in separate stores so
a single browser may be simultaneously authenticated as both.

#### Scenario: Operator who is also a portal user

- **GIVEN** the operator has signed in with admin credentials
- **WHEN** the operator signs in again with portal credentials
- **THEN** `adminAuthStore.token` and `portalAuthStore.token` SHALL coexist in localStorage
- **AND** the admin console SHALL remain accessible without re-authentication

## Implementation notes

- Attempt order is hard-coded admin-first (not heuristic) because both
  account formats are emails. The 1 wasted request per portal sign-in is
  acceptable for a self-hosted small-fleet deployment.
- `tryRole(role, account, password)` in `views/Login.vue` returns:
  - `true` on success (store updated as side effect)
  - `false` on 400/401/403/404 → fall through to the next role
  - throws on 5xx / network errors → caller surfaces via `formatError`
- The `?hint=` query param exists for the legacy redirects but is no
  longer used to pick attempt order. Frontend reads it for future
  optimization (preferring the hinted role first); the current code path
  ignores it for simplicity. The behavior is identical either way.

## Out of scope

- The visual chrome of the login page — see `layouts-and-chrome` (AuthLayout).
- Email verification mechanics — see `email-verification`.
- OIDC providers list at the bottom of the form — see `oidc-providers`.
- Backend admin/login implementation — see `admin-auth`.
- Backend register/login implementation — see `user-accounts`.
