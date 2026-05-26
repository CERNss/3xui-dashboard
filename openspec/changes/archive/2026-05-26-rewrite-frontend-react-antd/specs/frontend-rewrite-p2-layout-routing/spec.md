# frontend-rewrite-p2-layout-routing

P2 milestone. Lays the three chrome layouts (admin, portal, auth)
and the React Router tree on top of P1's infrastructure, so view
work in P3–P5 only writes the inner content.

**Entry criteria.** P1 (`frontend-rewrite-p1-cross-cutting`) is
complete. Zustand auth stores, shared `PageHeader` /
`AccountMenu` / `LocaleSwitcher`, and i18n are ready.

**Exit criteria.** Every requirement below holds. A new view can
be added by registering one route under `/admin/*` or `/portal/*`
and writing a function component; chrome and guards apply
automatically.

## ADDED Requirements

### Requirement: Router mirrors the Vue tree's paths 1:1

`frontend-react/src/router.tsx` SHALL define every path that
`frontend/src/router/index.ts` defines, with the same path
strings (so bookmarks survive) and the same default redirects.

#### Scenario: Every admin path resolves

- **GIVEN** the React tree is mounted
- **WHEN** the operator navigates to any of
  `/admin/status`, `/admin/stats`, `/admin/nodes`,
  `/admin/inbounds`, `/admin/users`, `/admin/plans`,
  `/admin/provisioning-pools`, `/admin/orders`,
  `/admin/audit-log`, `/admin/settings`
- **THEN** each path SHALL resolve to a registered route element
  (placeholder is acceptable in P2; real views land in P4)
- **AND** no 404 SHALL be shown

#### Scenario: Every portal path resolves

- **WHEN** the operator navigates to any of
  `/portal/subscription`, `/portal/usage`, `/portal/plans`,
  `/portal/orders`, `/portal/profile`
- **THEN** each path SHALL resolve to a registered route element

#### Scenario: Default redirects match the Vue tree

- **WHEN** the operator navigates to `/admin` with no sub-path
- **THEN** the router SHALL redirect to `/admin/status`
- **AND** navigating to `/portal` SHALL redirect to
  `/portal/subscription`
- **AND** navigating to `/` SHALL redirect to `/admin`

#### Scenario: Unknown path renders NotFound

- **WHEN** the operator navigates to `/admin/this-does-not-exist`
- **THEN** the router SHALL render the `<NotFound>` element
- **AND** the URL SHALL remain at the original path (not redirect)

### Requirement: `<ProtectedRoute>` gates admin and portal areas

The router SHALL wrap every `/admin/*` route in
`<ProtectedRoute area="admin">` and every `/portal/*` route in
`<ProtectedRoute area="portal">`. Unauthenticated access SHALL
redirect to `/login?next=<original-fullpath>`.

#### Scenario: Anonymous user hitting `/admin/users` lands on login

- **GIVEN** the `adminAuth` Zustand store has no JWT
- **WHEN** the operator navigates to `/admin/users?filter=active`
- **THEN** the router SHALL redirect to
  `/login?next=%2Fadmin%2Fusers%3Ffilter%3Dactive` (URL-encoded
  fullpath)
- **AND** after a successful login the user SHALL land on
  `/admin/users?filter=active`

#### Scenario: Default entry paths skip the `next=` parameter

- **GIVEN** the `adminAuth` store has no JWT
- **WHEN** the operator navigates to `/admin` (default entry)
- **THEN** the router SHALL redirect to `/login` with no
  `next=` query (matches Vue tree's `defaultAuthEntryPaths`
  behavior)

#### Scenario: Portal session does not satisfy admin guard

- **GIVEN** the `portalAuth` store has a JWT but `adminAuth` does
  not
- **WHEN** the operator navigates to `/admin/status`
- **THEN** the router SHALL redirect to `/login?next=/admin/status`
- **AND** the portal session SHALL remain intact (not cleared)

### Requirement: `AdminLayout` exposes a sidebar with section grouping

`AdminLayout` SHALL render an AntD `<Layout>` with a `<Sider>`
containing a `<Menu>` whose items mirror the section/items
structure of the Vue `AdminLayout` (Overview / Nodes / Inbounds /
Users / Billing / Settings). The currently-active path SHALL
highlight in the sidebar.

#### Scenario: Sidebar groups match the Vue tree

- **GIVEN** AdminLayout is mounted
- **WHEN** the operator inspects the sidebar
- **THEN** the menu SHALL contain at least one item per section
  defined in the Vue `AdminLayout`'s `sections` computed
- **AND** the labels SHALL come from the same `nav.*` i18n keys

#### Scenario: Active item follows route

- **GIVEN** the operator is on `/admin/users`
- **WHEN** AdminLayout renders
- **THEN** the "Users" menu item SHALL have AntD's selected style
  applied
- **AND** when the operator navigates to `/admin/nodes`, the
  selected item SHALL swap to "Nodes" without remount

#### Scenario: Account dropdown is present in the top bar

- **GIVEN** AdminLayout is mounted with an authenticated admin
- **WHEN** the operator looks at the top-right of the header
- **THEN** the shared `AccountMenu` SHALL render
- **AND** clicking it SHALL open the same items it opens in the
  Vue tree (locale switcher, theme toggle, logout)

### Requirement: `PortalLayout` carries the same chrome but a portal sidebar

`PortalLayout` SHALL render an AntD `<Layout>` shell sized for
end users, with sidebar items for Subscription / Usage / Plans /
Orders / Profile.

#### Scenario: Portal sidebar items match the Vue tree

- **GIVEN** PortalLayout is mounted
- **WHEN** the operator inspects the sidebar
- **THEN** the menu SHALL contain exactly the five items above
- **AND** the labels SHALL come from `nav.*` keys

### Requirement: `AuthLayout` wraps unauthenticated pages

`AuthLayout` SHALL provide a centered card shell used by
`Login`, `OIDCCallback`, and `NotFound`. It SHALL display the
site branding from `useBranding()` so visitors see the right
site name/logo before authenticating.

#### Scenario: Auth pages render in a centered card

- **GIVEN** the operator navigates to `/login`
- **WHEN** the page renders
- **THEN** the visible chrome SHALL be `AuthLayout` (centered
  card, branding bar at top)
- **AND** the same chrome SHALL apply to `/oidc/callback` and any
  `<NotFound>` outside `/admin` or `/portal`

#### Scenario: Branding loads via `useBranding()` not a store init

- **GIVEN** AuthLayout is mounted before any auth flow runs
- **WHEN** AuthLayout reads the branding payload
- **THEN** the source SHALL be `useBranding()` (TanStack Query)
- **AND** the same hook SHALL serve every other consumer (no
  duplicate fetch)

### Requirement: AdminLayout collapses sidebar to a drawer on narrow viewports

The platform SHALL render the admin sidebar as a persistent left sider above the `md` breakpoint (~768px), and SHALL collapse to a hamburger-triggered AntD `<Drawer>` below it, matching the Vue tree's `md:hidden` behavior.

#### Scenario: Wide viewport shows persistent sider

- **GIVEN** `window.matchMedia('(min-width: 768px)')` returns
  `matches: true`
- **WHEN** AdminLayout mounts
- **THEN** the sider SHALL be visible as a persistent column on
  the left
- **AND** the mobile top bar (hamburger + brand) SHALL NOT be
  rendered

#### Scenario: Narrow viewport shows hamburger + drawer

- **GIVEN** `window.matchMedia('(min-width: 768px)')` returns
  `matches: false`
- **WHEN** AdminLayout mounts
- **THEN** the persistent sider SHALL be hidden
- **AND** the top bar SHALL contain a hamburger button + brand
- **AND** clicking the hamburger SHALL open an AntD `<Drawer>`
  whose body is the same `<Menu>` items as the wide-viewport
  sider

#### Scenario: Drawer closes on backdrop click and on item select

- **GIVEN** the mobile drawer is open
- **WHEN** the operator taps the backdrop, OR taps a menu item
- **THEN** the drawer SHALL close
- **AND** if a menu item was tapped, the router SHALL navigate
  to its `to` path

### Requirement: PortalLayout shows a bottom-nav on narrow viewports

The platform SHALL render a fixed-position bottom navigation bar below the `lg` breakpoint (~1024px), with the 5 portal sections as bottom tabs; above the breakpoint the bottom-nav SHALL be hidden and the standard sider SHALL be used (matching the Vue tree's `lg:hidden` behavior).

#### Scenario: Narrow viewport shows the bottom-nav

- **GIVEN** `window.matchMedia('(min-width: 1024px)')` returns
  `matches: false`
- **WHEN** PortalLayout mounts
- **THEN** a fixed-position bar SHALL be rendered at
  `bottom: 0; left: 0; right: 0`
- **AND** the bar SHALL contain 5 items mapping to
  `/portal/subscription`, `/portal/usage`, `/portal/plans`,
  `/portal/orders`, `/portal/profile`
- **AND** the currently-active route SHALL have its bar item
  styled as selected

#### Scenario: Wide viewport hides the bottom-nav

- **GIVEN** `window.matchMedia('(min-width: 1024px)')` returns
  `matches: true`
- **WHEN** PortalLayout mounts
- **THEN** no bottom-nav element SHALL be in the DOM
- **AND** the standard left sider SHALL be visible

### Requirement: Breakpoint constants live in `theme.ts`, not view code

The platform SHALL define `MD_BREAKPOINT = 768` and `LG_BREAKPOINT = 1024` in `src/theme.ts` as exported constants; layout components SHALL reference these constants and MUST NOT inline pixel values into their `matchMedia` calls.

#### Scenario: No layout file hardcodes breakpoint pixel values

- **WHEN** the operator greps `src/components/layout/` for
  `768` or `1024`
- **THEN** the only matches SHALL be imports of
  `MD_BREAKPOINT` / `LG_BREAKPOINT` from `@/theme`
- **AND** no `matchMedia('(min-width: 768px)')` SHALL be inlined

### Requirement: P2 deliverables typecheck and route-level smoke tests pass

The platform SHALL ship at P2 exit a working router config, three layout components (Admin / Portal / Auth), and a `<ProtectedRoute>` HOC, each MUST have at least a smoke test that mounts it and asserts the chrome shape (sidebar / header presence) or guard behavior.

#### Scenario: Layout smoke tests cover the three layouts

- **WHEN** the operator runs `npm run test -- components/layout`
- **THEN** vitest SHALL execute at least one `it(...)` for each
  of `AdminLayout`, `PortalLayout`, `AuthLayout`
- **AND** all tests SHALL pass

#### Scenario: ProtectedRoute redirect behavior is covered

- **WHEN** the operator runs `npm run test -- ProtectedRoute`
- **THEN** vitest SHALL execute at least these cases:
  unauthenticated admin redirect, default-entry redirect-without-next,
  portal-session-does-not-satisfy-admin
- **AND** all cases SHALL pass
