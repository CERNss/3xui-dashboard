# layouts-and-chrome

The top-level React layout shells that wrap route content:
`AuthLayout`, `AdminLayout`, and `PortalLayout`.

## Purpose & Boundaries

Layouts own the chrome around route content: brand placement, navigation,
account menu, locale switcher, theme toggle placement, and responsive shell
behavior. Page content is owned by `admin-views` and portal view specs.

## Files

```
frontend/src/components/layout/
  AuthLayout.tsx    - wraps /login, /oidc/callback, and auth-context 404s
  AdminLayout.tsx   - wraps /admin/* routes with sidebar/topbar shell
  PortalLayout.tsx  - wraps /portal/* routes with side or bottom navigation
  nav.tsx           - shared nav item definitions
```

## Requirements

### Requirement: AuthLayout Wraps Pre-Login Pages

The system SHALL render pre-login and OIDC callback pages inside
`AuthLayout`.

#### Scenario: Auth route mounts

- **WHEN** the user opens `/login` or `/oidc/callback`
- **THEN** the route SHALL render inside `AuthLayout`
- **AND** the child route content SHALL be rendered through the layout's children or `Outlet`.

#### Scenario: Brand content is server-driven

- **WHEN** branding data is available from the public branding query
- **THEN** AuthLayout SHALL use the configured title, icon URL, description/subtitle, and footer
- **AND** fall back to built-in product copy when branding is absent.

#### Scenario: Locale switcher is available before login

- **WHEN** AuthLayout renders
- **THEN** a `LocaleSwitcher` SHALL be available in the auth chrome
- **AND** no admin or portal account menu SHALL be shown.

### Requirement: AdminLayout Provides Responsive Admin Chrome

The system SHALL render every `/admin/*` route inside `AdminLayout`.

#### Scenario: Wide admin navigation

- **WHEN** the viewport is at least the configured medium breakpoint
- **THEN** AdminLayout SHALL render a left `Sider`
- **AND** nav sections SHALL come from `adminSections(t)` in `nav.tsx`
- **AND** active route state SHALL be derived from the current pathname.

#### Scenario: Narrow admin navigation

- **WHEN** the viewport is narrower than the medium breakpoint
- **THEN** AdminLayout SHALL hide the wide sider
- **AND** expose a menu button that opens a left `Drawer` containing the same sidebar navigation.

#### Scenario: Admin topbar

- **WHEN** AdminLayout renders
- **THEN** the topbar SHALL show the active nav label, welcome copy, notifications affordance, locale switcher, and account menu
- **AND** logout SHALL clear the admin auth store and navigate to `/login`.

#### Scenario: Sidebar footer actions

- **WHEN** the admin sidebar renders
- **THEN** it SHALL provide theme toggle and collapse/expand actions
- **AND** collapsed mode SHALL preserve icon-only navigation with tooltips.

### Requirement: PortalLayout Provides Portal Navigation

The system SHALL render every `/portal/*` route inside `PortalLayout`.

#### Scenario: Wide portal navigation

- **WHEN** the viewport is at least the configured large breakpoint
- **THEN** PortalLayout SHALL render a left `Sider` containing portal nav items from `portalItems(t)`
- **AND** the active item SHALL follow the current pathname.

#### Scenario: Narrow portal navigation

- **WHEN** the viewport is narrower than the large breakpoint
- **THEN** PortalLayout SHALL render a fixed bottom navigation bar
- **AND** content padding SHALL leave room for the bottom nav and safe-area inset.

#### Scenario: Portal topbar

- **WHEN** PortalLayout renders
- **THEN** the header SHALL show the current user email or brand title, a locale switcher, and an account menu
- **AND** logout SHALL clear the portal auth store and navigate to `/login`.

### Requirement: Layouts Use React Router Outlet

The router SHALL compose layouts through React Router elements and `Outlet`.

#### Scenario: Admin route tree

- **WHEN** the user navigates to `/admin/nodes`
- **THEN** the rendered hierarchy SHALL include `ProtectedRoute area="admin"`, `AdminLayout`, and the `Nodes` view.

#### Scenario: Portal route tree

- **WHEN** the user navigates to `/portal/subscription`
- **THEN** the rendered hierarchy SHALL include `ProtectedRoute area="portal"`, `PortalLayout`, and the `Subscription` view.

## Out of Scope

- Per-page layout variants.
- Brand management API details, which are owned by `settings`.
