# frontend-platform-react Specification

## Purpose
TBD - created by archiving change rewrite-frontend-react-antd. Update Purpose after archive.
## Requirements
### Requirement: Canonical client-side library stack

The frontend SHALL be implemented exclusively on the following
libraries; introducing an alternative library that overlaps with
any of these roles requires a separate change proposal.

| Role | Library |
|---|---|
| UI rendering | React 18 |
| UI primitives + design tokens | Ant Design 5 (`antd`) |
| Iconography | `@ant-design/icons` |
| Routing | React Router v6 (`react-router-dom`) |
| Client state | Zustand |
| Server state | TanStack Query v5 (`@tanstack/react-query`) |
| HTTP client | axios |
| i18n | react-i18next + i18next |
| Date utility | dayjs |
| Build | Vite + `@vitejs/plugin-react` |
| Unit tests | Vitest + `@testing-library/react` |
| E2E tests | Playwright |

#### Scenario: package.json names every canonical library

- **WHEN** an inspector reads `frontend/package.json` (or
  `frontend-react/package.json` during the rewrite window)
- **THEN** every library in the table above SHALL appear in
  `dependencies` or `devDependencies` with a non-empty version
  range
- **AND** `vue`, `vue-router`, `vue-i18n`, `pinia`, and
  `@vitejs/plugin-vue` SHALL NOT appear in either section after
  cutover

#### Scenario: PR introducing a competing library is rejected

- **WHEN** a PR adds a library that overlaps an existing role
  (e.g. adding Redux while Zustand is the canonical client-state
  store, or adding SWR while TanStack Query is canonical)
- **THEN** the PR SHALL be blocked pending an OpenSpec change
  proposal that updates this table

### Requirement: Build output is `dist/` with `index.html` + `assets/`

The build SHALL emit a directory consumable by the backend's
`go:embed` directive at `backend/internal/web/embed.go`. The Go
side does not change as part of this rewrite, so the output
layout MUST remain identical to the current Vue build.

#### Scenario: Production build produces the expected layout

- **GIVEN** `npm run build` is invoked in the frontend tree
- **WHEN** the build completes successfully
- **THEN** `backend/internal/web/dist/index.html` SHALL exist
- **AND** `backend/internal/web/dist/assets/` SHALL contain at
  least one hashed JS bundle and one hashed CSS bundle
- **AND** any other top-level paths the existing Vue build emits
  (e.g. `dist/favicon.ico` if present) SHALL also be produced

#### Scenario: Backend serves SPA fallback unchanged

- **GIVEN** the React build output is present at the expected path
- **WHEN** the backend receives a GET for any non-`/api/*` route
  that does not match an asset
- **THEN** the backend SHALL serve `dist/index.html` verbatim
- **AND** no backend code change SHALL be required to support
  this — the existing SPA fallback handler covers React Router as
  cleanly as it covered vue-router

### Requirement: Every i18n key is preserved 1:1

The locale files in the React tree SHALL contain exactly the same
flattened key set as the Vue tree's
`frontend/src/i18n/locales/{zh,en}.ts` at the moment immediately
before cutover. New keys MAY be added during the rewrite; existing
keys MUST NOT be removed or renamed.

#### Scenario: Locale key parity script passes

- **GIVEN** the Vue tree and React tree both have their
  `zh.ts`/`en.ts` locale files
- **WHEN** the locale-parity script flattens both objects' key sets
  and diffs them
- **THEN** every key present in the Vue tree SHALL be present in
  the React tree

#### Scenario: Missing key fails loudly in development

- **GIVEN** `react-i18next` is configured with `returnNull: false`
  and `keySeparator: '.'`
- **WHEN** a component calls `t('admin.nonexistent.key')` in
  development mode
- **THEN** the call SHALL surface the missing key in the console
  rather than silently rendering an empty string

#### Scenario: vue-i18n interpolation syntax works unchanged

- **GIVEN** a locale string uses `{var}` interpolation (e.g.
  `admin.stats.kpiSubtitle.todayDelta = '今日: {value}'`)
- **WHEN** the React tree's `t('admin.stats.kpiSubtitle.todayDelta',
  { value: '2.51 GB' })` is invoked
- **THEN** the rendered string SHALL be `今日: 2.51 GB`

### Requirement: Server state goes through TanStack Query

Every HTTP fetch or mutation against the backend SHALL be issued
through a TanStack Query hook (`useQuery` or `useMutation`).
Components SHALL NOT call axios endpoints directly via
`useEffect`/`useState`.

#### Scenario: Component opens a list view

- **GIVEN** a view needs a list of resources from the backend
- **WHEN** the view is implemented
- **THEN** it SHALL consume a `useXxxList()` hook that wraps
  `useQuery`
- **AND** the hook SHALL define a `queryKey` of the form
  `[area, resource, op, ...args]` where `area ∈ {admin, portal,
  public}`

#### Scenario: Mutation invalidates the corresponding list

- **GIVEN** a mutation hook (e.g. `useUpdateNode()`) is invoked and
  succeeds
- **WHEN** the mutation's `onSuccess` runs
- **THEN** it SHALL invalidate every cached query whose key
  starts with the mutation's `[area, resource]` prefix
- **AND** any list view currently rendered SHALL refetch
  automatically

### Requirement: Client state goes through Zustand

The platform SHALL store all client-side session state (auth tokens, theme preference, app-locale) in Zustand stores under `src/stores/`, and components MUST NOT duplicate this state in local `useState`.

#### Scenario: JWT survives reload

- **GIVEN** a user has authenticated via `/admin/login` or
  `/portal/login` and the JWT is in the corresponding Zustand
  store
- **WHEN** the browser is reloaded
- **THEN** the store SHALL rehydrate from `localStorage` using
  `zustand/middleware`'s `persist` middleware
- **AND** the user SHALL remain authenticated without a second
  login

#### Scenario: Storage keys are stable across the Vue→React cutover

- **GIVEN** a user was logged in under the Vue tree before
  cutover, with a JWT stored at a specific `localStorage` key
- **WHEN** the cutover commit is deployed and the user opens the
  app
- **THEN** the React tree's Zustand store SHALL read the same
  `localStorage` key the Vue tree wrote
- **AND** the user SHALL NOT be forced to log in again

### Requirement: AntD theme is parameterized from the existing brand palette

The React tree SHALL configure a single `<ConfigProvider theme={...}>`
at the app root that carries forward the existing brand identity:
the indigo `primary` and teal `accent` from the Vue tree's
Tailwind config become AntD's `colorPrimary` and `colorSuccess`
tokens, respectively. The same `theme` Zustand store decides
between light and dark algorithms.

#### Scenario: Primary buttons render in the brand color

- **GIVEN** the app is mounted with the default light theme
- **WHEN** an AntD `<Button type="primary">` is rendered
- **THEN** its background color SHALL be the brand primary
  (`#6366f1` indigo, from the Vue tree's `primary-500` token) —
  not AntD's default blue

#### Scenario: Theme toggle swaps light/dark without remount

- **GIVEN** the `theme` Zustand store is currently `light`
- **WHEN** the user toggles to `dark`
- **THEN** `<ConfigProvider>` SHALL re-render with
  `theme.darkAlgorithm`
- **AND** the swap SHALL apply via AntD CSS variables — no
  component remount, no flash of unstyled content

### Requirement: Route guards enforce admin/portal area separation

The router SHALL gate every `/admin/*` route behind admin
authentication and every `/portal/*` route behind portal
authentication. Unauthenticated access SHALL redirect to
`/login` with a `next=` query param carrying the original path.

#### Scenario: Anonymous user opens an admin URL

- **GIVEN** there is no admin JWT in the `adminAuth` Zustand store
- **WHEN** the user navigates to `/admin/users`
- **THEN** the router SHALL redirect to `/login?next=/admin/users`
- **AND** after successful login the user SHALL land on
  `/admin/users` (not the admin home)

#### Scenario: Portal user attempts to enter the admin area

- **GIVEN** the user is authenticated as a portal user but not as
  an admin
- **WHEN** the user navigates directly to `/admin/status`
- **THEN** the router SHALL redirect to `/login?next=/admin/status`
- **AND** the portal session SHALL NOT be cleared

#### Scenario: Default landing on `/admin`

- **WHEN** an authenticated admin navigates to `/admin` with no
  sub-path
- **THEN** the router SHALL redirect to `/admin/status` (the
  Overview page, default tab `status`)

### Requirement: Shared widgets render identically across pages

The platform SHALL ensure that the same functional widget renders
identically wherever it appears. This is enforced by sourcing every
common interaction from AntD's component library (or a single
shared wrapper around it), not by hand-rolling per-page variants.

#### Scenario: Refresh button identical on every page

- **GIVEN** any admin page that exposes a refresh affordance
- **WHEN** the page renders
- **THEN** the refresh control SHALL be an AntD `<Button>` (or a
  single shared wrapper component) — never a hand-rolled
  inline-class button
- **AND** its label/icon/size SHALL match every other refresh
  button in the app

#### Scenario: Page header is a single shared component

- **GIVEN** multiple top-level pages need a "title + subtitle +
  trailing actions" header
- **WHEN** the page is implemented
- **THEN** it SHALL use the shared `PageHeader` wrapper (around
  AntD's primitives) — copying the JSX into the page is forbidden

### Requirement: No Vue tree at cutover

After the cutover commit, the repository SHALL contain exactly one
frontend tree — the React tree, located at `frontend/`. The Vue
tree SHALL be deleted, not archived under a sibling directory.

#### Scenario: Post-cutover repository has only React sources

- **GIVEN** the cutover commit has been merged
- **WHEN** a tree-walk lists files under `frontend/src/`
- **THEN** no `*.vue` files SHALL be present
- **AND** no Pinia store SHALL be present
- **AND** no `vite-plugin-vue` reference SHALL appear in any
  config file

#### Scenario: Backend embed continues to work

- **GIVEN** the cutover commit has been merged and the backend is
  rebuilt
- **WHEN** the binary serves the SPA
- **THEN** `index.html` SHALL load the React app
- **AND** no Go code change SHALL have been required for the
  cutover

