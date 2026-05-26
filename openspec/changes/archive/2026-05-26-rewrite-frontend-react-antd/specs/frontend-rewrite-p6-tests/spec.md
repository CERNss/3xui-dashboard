# frontend-rewrite-p6-tests

P6 milestone. Closes out the test surface: unit-spec parity for
every view, a shared render helper, e2e selector updates, and the
locale-parity guard that must pass before P7 cutover.

**Entry criteria.** P3–P5 are complete. Every view exists.

**Exit criteria.** Every requirement below holds. The full
`npm run test` + `npm run e2e` round-trip passes on the React
tree, and the locale-parity guard is wired into CI.

## ADDED Requirements

### Requirement: Test-count parity for every spec file

The React tree SHALL contain a `frontend-react/src/**/*.spec.tsx` for every `frontend/src/**/*.spec.ts` in the Vue tree at P6 entry time, mirroring the relative path, and each React spec SHALL contain at least as many `it(...)` blocks as the Vue spec. At rewrite-start time the Vue tree has 24 spec files broken down as: 4 shared components (`AccountMenu`, `ConfirmModal`, `EmptyState`, `Skeleton`), 1 layout (`AdminLayout`), 1 portal modal (`AlipayPayModal`), 1 composable (`useConfirm` — dropped per P1 rewrite, no React counterpart needed), 1 router (`router/index`), 13 admin views (every admin view file has a sibling spec), 3 portal views (`Plans`, `Profile`, plus any later additions), and `Login`.

#### Scenario: Parity audit script reports zero gaps

- **WHEN** the operator runs the parity audit
  (`node frontend-react/scripts/check-spec-parity.mjs`)
- **THEN** the script SHALL list every `.spec.ts` under the Vue
  tree and assert a sibling `.spec.tsx` under the React tree
- **AND** SHALL count `it(...)` blocks in each pair
- **AND** SHALL exit 0 only if the React spec's count is ≥ the
  Vue spec's count for every pair
- **AND** SHALL exclude `composables/useConfirm.spec.ts` from
  the parity check (the composable is intentionally removed by
  P1; no React equivalent exists)

#### Scenario: Audit fails when a spec is missing on the React side

- **GIVEN** a Vue spec exists with no React counterpart and is
  not on the documented exclusion list
- **WHEN** the parity audit runs
- **THEN** the script SHALL exit non-zero
- **AND** SHALL name the missing React path

#### Scenario: Router-level spec is ported, not skipped

- **GIVEN** the Vue tree has `src/router/index.spec.ts` covering
  unauthenticated admin / portal redirect behavior
- **WHEN** the React tree at P6 exit is inspected
- **THEN** an equivalent React spec SHALL exist (e.g.
  `src/router.spec.tsx` or under `components/ProtectedRoute`)
  exercising the same redirect cases — anonymous admin user
  hits `/admin/users`, default-entry without `next=`, portal
  session does not satisfy admin guard

### Requirement: A shared `renderWithProviders` helper exists

`frontend-react/src/test-utils/renderWithProviders.tsx` SHALL
export a `renderWithProviders(ui, opts?)` function that wraps the
rendered tree in a fresh `QueryClientProvider`,
`MemoryRouter` (with optional `initialEntries`), and the AntD
`ConfigProvider` from `src/theme.ts`. Every view spec SHALL use
this helper instead of constructing the providers inline.

#### Scenario: Helper wires every required provider

- **GIVEN** a component imports `useQuery`, `useTranslation`, and
  uses an AntD `<Table>`
- **WHEN** it is rendered via `renderWithProviders(<Component />)`
- **THEN** no provider-related error SHALL surface in the test
  log
- **AND** the component's data fetches SHALL be isolatable via
  the `vi.mock` of the underlying axios module

#### Scenario: Helper accepts an `initialPath`

- **GIVEN** a view that reads `useLocation()` to pick a default
  tab
- **WHEN** the test calls
  `renderWithProviders(<Overview/>, { initialPath: '/admin/stats' })`
- **THEN** the rendered tree SHALL behave as if the URL is
  `/admin/stats`

### Requirement: e2e smoke updates and passes

`e2e/smoke.spec.ts` SHALL keep its existing scope (login, navigate
to one admin page, assert content). Selectors SHALL update from
Vue-specific patterns to React DOM. Where stable selectors are
needed, components SHALL add `data-testid` attributes.

#### Scenario: e2e smoke passes against a React build

- **GIVEN** the backend is running with the React build embedded
- **WHEN** the operator runs `npm run e2e` (inside
  `frontend-react/`)
- **THEN** the smoke spec SHALL pass
- **AND** every selector used in the spec SHALL be either a
  `data-testid` attribute or a stable AntD class — no
  brittle nth-child selectors

### Requirement: Locale-parity guard is wired into CI

The CI pipeline SHALL run the locale-parity script introduced in P1 (`scripts/check-locale-parity.mjs`) on every PR, blocking merge if any key is dropped or renamed.

#### Scenario: CI step invokes the parity script

- **GIVEN** the repository's CI configuration (e.g.
  `.github/workflows/*.yml`)
- **WHEN** an inspector reads the React tree's job
- **THEN** the job SHALL include a step that runs
  `node frontend-react/scripts/check-locale-parity.mjs`
- **AND** the step SHALL be gated before merge (job failure ⇒
  PR cannot merge)

#### Scenario: Parity script detects a dropped key

- **GIVEN** a PR removes the key `admin.users.title` from the
  Vue tree's `zh.ts` but the React tree still references it
  (or vice versa)
- **WHEN** the CI step runs
- **THEN** the step SHALL exit non-zero
- **AND** SHALL name the dropped key in the failure output

### Requirement: Final P6 test sweep is green

By P6 exit, every script the rewrite cares about SHALL pass
cleanly on the React tree.

#### Scenario: Full test pipeline passes

- **WHEN** the operator runs the following commands in order
  inside `frontend-react/`:
  `npm run typecheck`, `npm run lint`, `npm run test`,
  `npm run e2e`, `node scripts/check-locale-parity.mjs`,
  `node scripts/check-spec-parity.mjs`
- **THEN** every command SHALL exit with code 0
- **AND** the React tree SHALL be considered "cutover-ready"
