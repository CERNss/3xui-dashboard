# frontend-platform-react Specification

## Purpose

Defines the supported React SPA platform: library stack, app providers,
state/query boundaries, routing, build output, and shared UI primitives. This
module is the current frontend contract; migration-era parity requirements are
not part of the active design.

## Requirements

### Requirement: Canonical Client-Side Stack

The frontend SHALL be implemented under `frontend/` using the following
libraries. Introducing another library that overlaps one of these roles
requires an OpenSpec change.

| Role | Library |
|---|---|
| UI rendering | React 18 |
| UI primitives and theme tokens | Ant Design 5 (`antd`) |
| Iconography | `@ant-design/icons` |
| Routing | React Router v6 (`react-router-dom`) |
| Client state | Zustand |
| Server state | TanStack Query v5 (`@tanstack/react-query`) |
| HTTP client | axios |
| i18n | `react-i18next` + `i18next` |
| Date utility | dayjs |
| Build | Vite + `@vitejs/plugin-react` |
| Unit/component tests | Vitest + Testing Library |
| E2E tests | Playwright |

#### Scenario: Package manifest names the supported stack

- **WHEN** an inspector reads `frontend/package.json`
- **THEN** every library in the table above SHALL appear in `dependencies` or `devDependencies`
- **AND** package dependencies SHALL align with the single supported frontend stack in this spec.

#### Scenario: Competing client-state library is proposed

- **WHEN** a PR adds Redux, SWR, another router, or another UI component system
- **THEN** the PR SHALL include an OpenSpec change that updates this platform contract.

### Requirement: App Root Owns Providers

`frontend/src/main.tsx` SHALL mount a single React root and install the
global providers that every view depends on.

#### Scenario: Root provider order

- **WHEN** the SPA boots
- **THEN** `main.tsx` SHALL render `ConfigProvider`, AntD `App`, `QueryClientProvider`, `BrowserRouter`, and `AppRouter`
- **AND** theme selection SHALL come from `useThemeStore().resolvedTheme`
- **AND** `frontend/src/i18n/index.ts` SHALL be imported before route views render.

### Requirement: Build Output Embeds Into The Go Binary

The frontend build SHALL emit assets into the backend web embed directory.

#### Scenario: Production build layout

- **GIVEN** `npm run build` is invoked in `frontend/`
- **WHEN** the build completes
- **THEN** `backend/internal/web/dist/index.html` SHALL exist
- **AND** `backend/internal/web/dist/assets/` SHALL contain hashed JS and CSS assets
- **AND** the backend's existing SPA fallback SHALL serve `index.html` for non-API browser routes.

#### Scenario: Development server proxies backend paths

- **WHEN** the Vite dev server runs
- **THEN** `/api`, `/sub`, and `/uploads` requests SHALL proxy to `http://localhost:8080`
- **AND** the dev server SHALL default to port `5174`.

### Requirement: Routing Uses React Router Guards

`frontend/src/router.tsx` SHALL be the canonical route map.

#### Scenario: Route topology

- **WHEN** routes are inspected
- **THEN** `/login` and `/oidc/callback` SHALL render inside `AuthLayout`
- **AND** `/admin/*` SHALL render inside `ProtectedRoute area="admin"` and `AdminLayout`
- **AND** `/portal/*` SHALL render inside `ProtectedRoute area="portal"` and `PortalLayout`
- **AND** `/` SHALL redirect to `/admin`.

#### Scenario: Admin default route

- **WHEN** an authenticated admin opens `/admin`
- **THEN** the router SHALL redirect to `/admin/status`.

#### Scenario: Portal default route

- **WHEN** an authenticated portal user opens `/portal`
- **THEN** the router SHALL redirect to `/portal/subscription`.

### Requirement: Server State Goes Through TanStack Query

HTTP reads and writes SHALL be wrapped by query/mutation hooks. View
components SHOULD consume hooks instead of calling axios clients directly.

#### Scenario: View opens a list page

- **GIVEN** a view needs backend data
- **WHEN** the view is implemented
- **THEN** it SHALL consume a `useXxx...` query hook under `frontend/src/hooks/queries/`
- **AND** the query key SHALL identify the area (`admin`, `portal`, or `public`) and resource.

#### Scenario: Mutation succeeds

- **WHEN** a mutation updates a resource
- **THEN** its success handler SHALL invalidate or refresh the affected query keys so visible lists refetch.

### Requirement: Client State Goes Through Zustand

Session, locale, and theme state SHALL live in Zustand stores under
`frontend/src/stores/`.

#### Scenario: Admin JWT survives reload

- **GIVEN** an admin has logged in successfully
- **WHEN** the browser reloads
- **THEN** `useAdminAuthStore` SHALL rehydrate the admin token from localStorage
- **AND** guarded admin routes SHALL remain accessible until the token expires or is cleared.

#### Scenario: Portal JWT survives reload

- **GIVEN** a portal user has authenticated through OIDC or user login API
- **WHEN** the browser reloads
- **THEN** `usePortalAuthStore` SHALL rehydrate the portal token and user identity.

#### Scenario: Legacy token keys are read

- **WHEN** auth stores initialize
- **THEN** they MAY read legacy token keys defined in `frontend/src/stores/storage.ts`
- **AND** the canonical persisted store keys SHALL remain `3xui.adminAuth` and `3xui.portalAuth`.

### Requirement: Shared Components Carry Repeated UI Patterns

Repeated page primitives SHALL live under `frontend/src/components/common/`
or `frontend/src/components/layout/`.

#### Scenario: Page header repeats

- **WHEN** a top-level page needs title, subtitle, and actions
- **THEN** it SHALL use `PageHeader` instead of copying equivalent header markup.

#### Scenario: Refresh action repeats

- **WHEN** a page exposes manual reload
- **THEN** it SHALL use `RefreshButton` or an AntD button with the same behavior and iconography.

#### Scenario: Responsive table/list repeats

- **WHEN** a resource list needs table-on-wide and list-on-narrow behavior
- **THEN** it SHOULD use `ResponsiveListTable` rather than maintaining independent duplicated layouts.

### Requirement: Only React Sources Are Supported

The supported frontend source tree is `frontend/src` and uses `.ts`/`.tsx`.

#### Scenario: Source tree is inspected

- **WHEN** `frontend/src` is scanned
- **THEN** page, component, store, and route implementations SHALL be React/TypeScript files
- **AND** no second frontend source tree SHALL be required to build or run the app.
