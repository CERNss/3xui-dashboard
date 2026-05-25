# frontend-rewrite-p1-cross-cutting

P1 milestone. Ports the framework-agnostic horizontal layers — API
client, i18n, format utilities, Zustand stores, shared widgets —
so the rest of the rewrite (layouts, auth, views) can lean on them
without each view re-implementing infrastructure.

**Entry criteria.** P0 (`frontend-rewrite-p0-scaffold`) is
complete. `frontend-react/` builds and a hello-world page is up.

**Exit criteria.** Every requirement below holds. A new view file
can be written by importing from `@/api/...`, `@/hooks/queries/...`,
`@/stores/...`, `@/components/common/...`, and `@/utils/format`
without needing to touch any infra file.

## ADDED Requirements

### Requirement: Every Vue API module is mirrored 1:1 in the React tree

`frontend-react/src/api/` SHALL contain a TypeScript module for
every module under `frontend/src/api/`, with the same exported
type names and function signatures so consumers don't relearn the
API surface.

#### Scenario: Module-by-module file parity

- **WHEN** the operator lists `frontend/src/api/` and
  `frontend-react/src/api/` recursively
- **THEN** every `.ts` file path under the Vue tree (e.g.
  `admin/nodes.ts`, `portal/billing.ts`, `client/factory.ts`)
  SHALL exist under the React tree at the matching relative path

#### Scenario: Exported symbols are identical

- **GIVEN** an arbitrary api module pair (e.g. Vue
  `api/admin/users.ts` and React `api/admin/users.ts`)
- **WHEN** TypeScript resolves their exports
- **THEN** every exported `const`, `type`, and `interface` SHALL
  share the same name and shape across the two trees
- **AND** every exported function SHALL accept the same arguments
  and return the same promise type

#### Scenario: Axios interceptor behavior carries over

- **GIVEN** the React `api/client/admin.ts` axios instance is in
  use
- **WHEN** the backend returns 401 to a privileged call
- **THEN** the same interceptor logic as the Vue tree SHALL fire
  (clear auth store, redirect to `/login?next=...`)

### Requirement: Every endpoint has a TanStack Query hook

The platform SHALL provide a TanStack Query hook under `hooks/queries/` for every axios function exported under `api/admin/*` and `api/portal/*`. Read endpoints get a `useXxx*()` `useQuery` hook; write endpoints get a `useXxx*()` `useMutation` hook with correct invalidation.

#### Scenario: List endpoint exposes a query hook

- **GIVEN** `api/admin/nodes.ts` exports `nodesApi.list()`
- **WHEN** the operator imports
  `useNodesList` from `@/hooks/queries/admin/nodes`
- **THEN** the import SHALL resolve to a hook returning the
  TanStack Query result type `UseQueryResult<Node[], Error>`
- **AND** the hook's `queryKey` SHALL be of the form
  `['admin', 'nodes', 'list']`

#### Scenario: Mutation invalidates its resource family

- **GIVEN** `useUpdateNode()` is invoked and the mutation
  resolves successfully
- **WHEN** the mutation's `onSuccess` callback runs
- **THEN** the `QueryClient` SHALL invalidate every cached query
  whose key starts with `['admin', 'nodes']`
- **AND** a currently-rendered `useNodesList()` SHALL refetch

### Requirement: i18n locale files migrate with zero key drift

`frontend-react/src/i18n/locales/{zh,en}.ts` SHALL contain
exactly the same flattened key set as the corresponding Vue
files, with the same string values, and SHALL be loaded into
`i18next` with `returnNull: false` and `keySeparator: '.'`.

#### Scenario: Locale-parity script reports zero diff

- **GIVEN** the operator runs
  `node frontend-react/scripts/check-locale-parity.mjs`
- **WHEN** the script flattens both trees' locale objects and
  diffs their key sets
- **THEN** the script SHALL exit with code 0
- **AND** the script's stdout SHALL contain `OK` or equivalent
- **AND** if any key is missing on either side, the script SHALL
  exit non-zero with the missing key names listed

#### Scenario: `{var}` interpolation works in the React tree

- **GIVEN** `i18next` has loaded the zh locale
- **WHEN** a component calls
  `t('admin.stats.kpiSubtitle.todayDelta', { value: '2.51 GB' })`
- **THEN** the returned string SHALL be `今日: 2.51 GB`

#### Scenario: Missing key surfaces in development

- **GIVEN** the React tree is running in development mode
- **WHEN** a component calls `t('admin.nonexistent.key')`
- **THEN** the dev console SHALL log a missing-key warning
  identifying the key

### Requirement: `utils/format.ts` ports verbatim

`frontend-react/src/utils/format.ts` SHALL contain the same
exports as `frontend/src/utils/format.ts`, with identical
signatures and identical output for the same inputs (golden
test).

#### Scenario: `formatError` produces the same message

- **GIVEN** an axios error with `response.data.error.message =
  "node offline"`
- **WHEN** the React `formatError(err, 'fallback')` is called
- **THEN** it SHALL return `"node offline"`
- **AND** the Vue version called with the same input SHALL return
  the same string

#### Scenario: `formatBytes` produces the same human-readable size

- **GIVEN** an input of `1610612736` (1.5 GiB)
- **WHEN** the React `formatBytes()` is called
- **THEN** it SHALL return `"1.50 GiB"`
- **AND** the Vue version SHALL return the same string

### Requirement: Five Zustand stores match the Pinia stores 1:1

`frontend-react/src/stores/` SHALL contain Zustand stores named
`adminAuth`, `portalAuth`, `app`, `theme`, with the same state
shape, action names, and persistence keys as the corresponding
Pinia stores. `branding` SHALL be implemented as a TanStack Query
hook (`useBranding()`) rather than a store.

#### Scenario: Auth store rehydrates from existing localStorage key

- **GIVEN** the Vue tree has previously logged in as admin and
  written the JWT to `localStorage` key `3xui.adminAuth`
- **WHEN** the React tree's `useAdminAuthStore()` initializes
- **THEN** the store SHALL read the existing `localStorage` value
- **AND** the user SHALL be considered authenticated without a
  fresh login

#### Scenario: `useBranding()` returns the branding payload

- **GIVEN** the backend serves `/api/public/branding` with a JSON
  payload
- **WHEN** any component calls `useBranding()`
- **THEN** the hook SHALL return the same payload shape the Vue
  `useBrandingStore` exposed (e.g. `siteName`, `repoUrl`)
- **AND** the value SHALL be cached so subsequent calls within
  five minutes do NOT trigger another HTTP request

#### Scenario: Theme store toggles light/dark without reload

- **GIVEN** the theme store is currently `light`
- **WHEN** an action toggles it to `dark`
- **THEN** the new value SHALL persist to `localStorage`
- **AND** the AntD `<ConfigProvider>` SHALL re-evaluate its
  `algorithm` so the page repaints without a full remount

### Requirement: Shared component primitives exist and are documented

`frontend-react/src/components/common/` SHALL export the
following primitives, each backed by AntD or a thin wrapper
around it: `ConfirmModal` (or operator note that
`Modal.confirm` is used directly), `EmptyState`, `Skeleton`,
`AccountMenu`, `LocaleSwitcher`, `PageHeader`, `RefreshButton`.

#### Scenario: `RefreshButton` is the single source of truth

- **GIVEN** any future view that needs a refresh affordance
- **WHEN** the developer imports a refresh control
- **THEN** the only available import path SHALL be
  `@/components/common/RefreshButton`
- **AND** there SHALL NOT be a hand-rolled AntD `<Button icon=...>`
  inside any view that duplicates RefreshButton's role

#### Scenario: `PageHeader` carries title, subtitle, actions

- **GIVEN** a view needs a top-of-page header
- **WHEN** the developer uses `<PageHeader title=... subtitle=...
  actions={...} />`
- **THEN** the rendered header SHALL place the title and subtitle
  on the left, the actions slot on the right
- **AND** every page in the React tree that has a header SHALL
  use this component (no inline `<h1>` + `<p>` duplicates)

#### Scenario: `EmptyState` accepts the same props as the Vue version

- **GIVEN** the Vue `EmptyState` accepts `icon`, `title`,
  `description`, `actionLabel`, and emits an `action` event
- **WHEN** the React `EmptyState` is rendered
- **THEN** it SHALL accept the same prop names
- **AND** it SHALL accept an `onAction` callback that fires when
  the action button is clicked

### Requirement: P1 deliverables compile, lint, and test clean

The platform SHALL typecheck, lint, and test cleanly by P1 exit, with at least one unit test (or test stub) covering each shared component primitive added under `frontend-react/src/api/`, `hooks/queries/`, `i18n/`, `stores/`, `utils/`, and `components/common/`.

#### Scenario: All P1 scripts pass

- **WHEN** the operator runs `npm run typecheck && npm run lint
  && npm run test` inside `frontend-react/`
- **THEN** all three commands SHALL exit with code 0
- **AND** at least one test SHALL exercise each of the seven
  shared components (or document the Modal.confirm exception)
