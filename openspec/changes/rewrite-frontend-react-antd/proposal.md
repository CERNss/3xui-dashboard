# rewrite-frontend-react-antd

## Why

The Vue 3 + bespoke-Tailwind frontend has hit two structural limits:

- **Component drift.** Same widget (refresh button, page header, status
  badge, KPI card) is re-implemented per page with subtly different
  Tailwind class strings. The `refactor-admin-portal-ui-primitives`
  change tried to extract primitives, but each new admin surface keeps
  growing more one-off copies — the user explicitly flagged
  "same component looks different across pages" as a quality bug
  during the Status+Stats → Overview merge on 2026-05-24.
- **Form / table velocity.** The 4 heavy admin views (Settings 1565,
  Users 1300, Inbounds 1282, InboundEditorModal 1178) are ~58% of the
  source LOC and consist almost entirely of hand-written form + table
  layouts. AntD's `Form`, `Table`, `Modal`, `Drawer` collapse these by
  3-5× while solving validation, pagination, selection, and i18n
  number/date formatting in a single library.

Pre-launch greenfield (no real users, no production deploys) is the
right window for a full platform swap. After launch the cost grows
quickly: every operator dashboard ships AntD-shaped admin UX, and
matching that visual vocabulary on a Tailwind-rolled stack is a tax
we're paying for no upside.

Reference upstream (3x-ui original codebase) is irrelevant here — the
frontend was already a clean-room re-implementation, not an extraction.
Nothing is being dropped from upstream; this change touches only the
local Vue tree.

## What Changes

This is a **wholesale frontend tech-stack swap**, not a feature change.
The product surface (every admin / portal page, every i18n string,
every API contract, every backend endpoint) is held constant. Only the
client-side rendering platform changes.

### New tech stack

- **React 18** + **TypeScript** (Vite + `@vitejs/plugin-react`)
- **Ant Design 5** as the UI primitive library (full design language,
  not just components). Tailwind retained only for spacing / grid
  patches via `tailwindcss` postcss plugin if needed; intent is to lean
  on AntD `Flex` / `Row` / `Col` / `Space` first.
- **React Router v6** (path-based, no hash router)
- **Zustand** for client state (auth, theme, branding, locale)
- **TanStack Query v5** for all server-state fetches/mutations
  (every existing axios endpoint gets a `useXxx()` hook wrapper)
- **react-i18next** for i18n — locale files migrate 1:1, key names
  preserved
- **axios** retained (unchanged)
- **dayjs** for date formatting (AntD's expected adapter)
- **Vitest** + **@testing-library/react** for unit tests
- **Playwright** retained for e2e; only selectors update

### Migration strategy

- **Parallel rewrite.** New `frontend-react/` directory at the repo
  root, built in isolation. The existing `frontend/` Vue tree keeps
  working throughout — operators can still run `make dev` against it
  for any hotfix during the rewrite window.
- **Single cutover at the end.** When `frontend-react/` reaches
  feature parity and tests pass, replace `frontend/` in one commit:
  `rm -rf frontend && mv frontend-react frontend`. No vue/react bridge
  layer, no per-route dual-mount.
- **No backwards-compat shims.** Per the pre-launch greenfield
  principle, the Vue tree is deleted at cutover, not archived as a
  fallback.

### What stays unchanged

- Every backend endpoint, request/response shape, JWT format, OIDC
  flow, payment gateway, webhook contract.
- Every i18n key (`admin.*`, `portal.*`, `nav.*`, `section.*`,
  `auth.*`, `app.*` …). String values may be tightened during the
  rewrite, but keys are 1:1.
- The `backend/internal/web/dist/` consumer path. Backend's
  `go:embed` directive does not change; the new Vite build output
  must land at the same path with an `index.html` + `assets/` layout.
- The Playwright e2e at `e2e/smoke.spec.ts`. Only DOM selectors
  update.

### Out of scope

- No new product features. If a feature is missing from the Vue
  tree, it stays missing — it is added in a separate change after
  cutover.
- No backend changes whatsoever.
- No design refresh beyond what AntD's default token set imposes.
  Brand color palette (accent / primary) is ported from the existing
  Tailwind config to AntD `theme.token` so navbars and primary
  buttons stay on-brand.

## Capabilities

### New Capabilities

The change introduces one **platform-level** capability that
survives archival as the live spec going forward, plus eight
**milestone-level** capabilities that exist only for the duration
of the rewrite (entry/exit contracts per phase). The milestone
specs are NOT archived into `openspec/specs/` at completion; only
`frontend-platform-react` lives on.

- `frontend-platform-react`: The new React + AntD frontend
  platform contract — which libraries are canonical, how server
  state / client state are partitioned, how AntD theme tokens map
  from the existing brand palette, how i18n keys are preserved,
  and the build output contract that backend `go:embed` depends
  on. Lives on after archival.
- `frontend-rewrite-p0-scaffold`: P0 acceptance — parallel
  `frontend-react/` directory exists, dev/build pipelines work on
  port 5174, hello-world page renders with brand chrome, zero
  impact on the Vue tree.
- `frontend-rewrite-p1-cross-cutting`: P1 acceptance — 21 axios
  API modules ported 1:1, TanStack Query hook per endpoint,
  locale-parity guard, five Zustand stores with stable
  localStorage keys, seven shared component primitives.
- `frontend-rewrite-p2-layout-routing`: P2 acceptance — three
  layouts (AdminLayout / PortalLayout / AuthLayout) wired to
  AntD `<Layout>`, React Router mirrors Vue paths 1:1,
  ProtectedRoute enforces admin/portal area separation.
- `frontend-rewrite-p3-auth-surface`: P3 acceptance — Login
  (password + OIDC), OIDCCallback, NotFound; end-to-end auth
  round-trip works in the React tree alone.
- `frontend-rewrite-p4-admin-views`: P4 acceptance — 13 admin
  views replace placeholders; AntD `<Table>` / `<Form>` /
  `<Drawer>` uniformly used; Overview tabs work, Settings has 7
  sub-tabs, Inbounds editor is a drawer.
- `frontend-rewrite-p5-portal-views`: P5 acceptance — 5 portal
  views + AlipayPayModal; Subscription QR matches URL, purchase
  flow lands an order, Profile handles email/password/OIDC link.
- `frontend-rewrite-p6-tests`: P6 acceptance — test-count parity
  per spec file, shared `renderWithProviders` helper, e2e smoke
  passes, locale-parity guard wired into CI.
- `frontend-rewrite-p7-cutover`: P7 acceptance — Vue tree deleted,
  React tree renamed to `frontend/`, Makefile/README/build
  scripts swept, backend embed unchanged, OpenSpec change
  archived (milestone specs dropped, platform spec retained).

### Modified Capabilities

None. Every product-level capability (`admin-views`,
`layouts-and-chrome`, `design-system`, `theme-system`, `i18n`,
`unified-login`, `subscription`, etc.) keeps the same requirements;
only the implementation stack changes. Once `frontend-platform-react`
lands, the existing spec files in those capabilities will continue to
describe behavior accurately — they are platform-agnostic about
Vue-vs-React.

## Impact

### Affected code

- **Deleted at cutover**: entire `frontend/src/` (21 views, 9
  components, 5 stores, 21 API modules, 2 locale files, ~18.8K LOC).
- **Added during the change**: parallel `frontend-react/` tree
  containing the React equivalents. Final cutover renames it to
  `frontend/`.

### Affected systems

- **Backend (`backend/internal/web/`)**: zero code changes. The
  `go:embed dist` directive continues to point at the same path. CI
  build script must invoke the new `frontend-react/` build during the
  rewrite window and `frontend/` post-cutover.
- **Makefile / docker build / README**: paths and commands updated
  twice — once when `frontend-react/` is introduced (so it can be
  built and served), once at cutover (when it becomes `frontend/`).
- **CI**: vitest + playwright commands run against the new tree.

### Affected dependencies

- **Removed (Vue stack)**: `vue`, `vue-router`, `vue-i18n`, `pinia`,
  `@vueuse/core`, `@vitejs/plugin-vue`, `@vue/test-utils`,
  `eslint-plugin-vue`, `vue-tsc`. Tailwind may be retained as a
  postcss utility layer or removed entirely depending on how cleanly
  AntD covers the spacing/grid needs.
- **Added (React stack)**: `react`, `react-dom`, `react-router-dom`,
  `antd`, `@ant-design/icons`, `zustand`, `@tanstack/react-query`,
  `react-i18next`, `i18next`, `dayjs`, `@vitejs/plugin-react`,
  `@testing-library/react`, `@testing-library/jest-dom`,
  `eslint-plugin-react`, `eslint-plugin-react-hooks`. `axios` and
  `qrcode` and `js-yaml` are retained as-is.

### Operator-visible impact

- **During the rewrite window**: zero. The Vue version continues to
  run. The new `frontend-react/` is not wired to the backend until
  cutover.
- **At cutover**: every page is visually different (AntD design
  language replaces the bespoke Tailwind look). URLs and bookmarks
  preserved. Login session preserved (JWT contract unchanged).
- **Post-cutover**: faster table/form iteration; consistent widget
  styling enforced by AntD's component contract; smaller maintenance
  surface for the next product push.
