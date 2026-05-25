# frontend-rewrite-p3-auth-surface

P3 milestone. Lands the three pre-auth views: Login,
OIDCCallback, NotFound. After P3 a developer can run
`make dev-frontend-react`, log in with admin credentials, and
land on `/admin/status` end-to-end — independent of the Vue tree.

**Entry criteria.** P2 (`frontend-rewrite-p2-layout-routing`) is
complete. ProtectedRoute, AuthLayout, and the auth Zustand stores
are wired.

**Exit criteria.** Every requirement below holds. The React tree
is the first time a real user-flow works in it.

## ADDED Requirements

### Requirement: Login supports password + OIDC entry points

`Login.tsx` SHALL render an AntD `Form` with the same fields and
validation behavior as `frontend/src/views/Login.vue`. It SHALL
also surface every configured OIDC provider as a button below the
password form, using the same provider metadata the Vue tree
reads from `/api/public/oidc/providers`.

#### Scenario: Password form submits to the backend

- **GIVEN** the operator has navigated to `/login`
- **WHEN** the operator types valid admin credentials and clicks
  submit
- **THEN** the form SHALL POST to the same admin login endpoint
  the Vue tree uses
- **AND** on 2xx the JWT SHALL land in the `adminAuth` Zustand
  store (and `localStorage`)
- **AND** the page SHALL navigate to `/admin/status` (or to the
  `next=` query if present)

#### Scenario: Password form rejects empty submission

- **GIVEN** the operator has navigated to `/login`
- **WHEN** the operator submits with empty username or password
- **THEN** the form SHALL display per-field validation messages
- **AND** SHALL NOT issue an HTTP request

#### Scenario: OIDC providers render as buttons

- **GIVEN** the backend returns 2 configured OIDC providers
- **WHEN** the Login page renders
- **THEN** there SHALL be 2 buttons below the password form,
  each labeled with the provider's `displayName`
- **AND** clicking a provider button SHALL redirect to its
  authorization URL (the same URL the Vue tree visits)

#### Scenario: `next=` query routes the user back

- **GIVEN** the operator arrives at `/login?next=%2Fadmin%2Fusers`
  after a guard redirect
- **WHEN** the operator completes login successfully
- **THEN** the page SHALL navigate to `/admin/users`
- **AND** the `next=` value SHALL NOT survive into the next URL

### Requirement: OIDCCallback exchanges code+state for a JWT

`OIDCCallback.tsx` SHALL be registered at `/oidc/callback` and
SHALL POST `code` + `state` from the URL query to
`/api/user/auth/oidc/callback`. On success it SHALL store the
JWT in the portal Zustand store and navigate to
`/portal/subscription` (or the `next=` if present).

#### Scenario: Successful callback lands on portal

- **GIVEN** the IdP has redirected to
  `/oidc/callback?code=abc&state=xyz`
- **WHEN** the page mounts and the backend returns a JWT
- **THEN** the `portalAuth` store SHALL hold the JWT
- **AND** the page SHALL navigate to `/portal/subscription`

#### Scenario: Failed callback shows a recoverable error

- **GIVEN** the backend returns 4xx for the callback exchange
- **WHEN** the page mounts and the request fails
- **THEN** the page SHALL render an error state with the failure
  message
- **AND** SHALL surface a button to retry the login flow

#### Scenario: Missing code or state is treated as an invalid entry

- **GIVEN** the page is opened directly without `code` and `state`
  query parameters
- **WHEN** the page mounts
- **THEN** no HTTP request SHALL be issued
- **AND** an explanatory error state SHALL render

### Requirement: NotFound renders an AntD Result

`NotFound.tsx` SHALL render AntD's `<Result status="404" />`
with a CTA that routes back to `/admin` (or `/portal`, depending
on the closest authenticated context).

#### Scenario: 404 page is reachable

- **WHEN** the operator navigates to `/nonexistent`
- **THEN** the page SHALL render `<Result status="404">`
- **AND** SHALL show a "Back to home" CTA

### Requirement: End-to-end auth round-trip works in the React tree

By P3 exit, a fresh operator SHALL be able to launch the React
tree alone (no Vue dev server), authenticate as admin, and reach
the Overview page — even if the Overview is still a placeholder.

#### Scenario: Manual smoke check passes

- **GIVEN** the backend is running with seeded admin credentials
- **WHEN** the operator runs `make dev-frontend-react`, opens
  `http://localhost:5174/login`, enters the admin credentials,
  and submits
- **THEN** the operator SHALL land at `http://localhost:5174/admin/status`
- **AND** the URL SHALL show the AdminLayout chrome (sidebar +
  header) from P2
- **AND** the `adminAuth` localStorage key SHALL hold a JWT

#### Scenario: Logout clears auth and routes to login

- **GIVEN** the operator is on `/admin/status` with a valid JWT
- **WHEN** the operator clicks logout from the `AccountMenu`
- **THEN** the `adminAuth` store SHALL clear its JWT
- **AND** `localStorage` SHALL no longer contain the admin JWT
- **AND** the page SHALL navigate to `/login` (no `next=` query)

### Requirement: P3 specs pass

Every view added in P3 SHALL ship with a `.spec.tsx` file at the
same path covering at least the scenarios above. Test count
SHALL meet or exceed the corresponding Vue spec.

#### Scenario: P3 test files exist and pass

- **WHEN** the operator runs
  `npm run test -- src/views/Login.spec src/views/OIDCCallback.spec src/views/NotFound.spec`
- **THEN** all three test files SHALL be present
- **AND** all assertions SHALL pass
- **AND** each file SHALL have at least as many `it(...)` blocks
  as the corresponding Vue `.spec.ts`
