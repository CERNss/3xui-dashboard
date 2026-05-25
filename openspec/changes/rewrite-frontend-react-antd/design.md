# Design: rewrite-frontend-react-antd

## Context

The current Vue 3 frontend (Vue Router + Pinia + vue-i18n + bespoke
Tailwind) renders the entire 3xui-dashboard SPA — 13 admin views,
5 portal views, 3 shared views, ~18.8K LOC in `frontend/src/`. The
backend embeds the build via `go:embed dist` from
`backend/internal/web/`.

The pre-launch greenfield window is closing. Per the project memory
on 2026-05-21, there are no real users and no production deploys
yet; the dashboard-user-panel batch (2026-05-19 → 2026-05-23) was
the largest UI push so far and already exposed the structural
limits described in the proposal. Doing the platform swap now means
zero migration cost; doing it after launch would mean training each
operator on a second visual language.

Stakeholders: solo author + the operator audience (admins running
3x-ui node fleets, end users of those fleets). No external
consumers depend on the frontend's DOM structure or class names.

## Goals / Non-Goals

**Goals:**

- Replace Vue 3 with React 18 + AntD 5 as the canonical client
  rendering platform, with zero loss of existing functionality.
- Preserve every backend contract (JWT, REST endpoints, OIDC flow,
  payment gateways, webhooks, subscription URLs).
- Preserve every i18n key (`admin.*`, `portal.*`, `nav.*`,
  `section.*`, `auth.*`, `app.*` — full list lives in
  `frontend/src/i18n/locales/{zh,en}.ts`) so locale strings travel
  unchanged.
- Preserve the `backend/internal/web/dist/` consumer path and
  `index.html` + `assets/` layout so `go:embed` works without code
  change.
- Establish a consistent widget contract enforced by AntD's
  component library, eliminating per-page hand-rolled drift.
- Reach feature parity in a parallel `frontend-react/` directory
  before cutover; the existing `frontend/` Vue tree stays runnable
  throughout the rewrite window.

**Non-Goals:**

- No new product features. Feature parity only; new work happens in
  follow-up changes after cutover.
- No backend code changes. If a missing-but-needed API surfaces
  mid-rewrite, that is a bug in the Vue tree and must be added to
  both stacks (or deferred until cutover).
- No design language refresh beyond AntD's default tokens. The
  bespoke Geist Sans / accent-teal / soft-card aesthetic is replaced
  by AntD defaults parameterized with the existing brand color.
- No vue/react bridge layer or per-route dual-mount. Cutover is a
  single commit that renames `frontend-react/` to `frontend/`.
- No upstream 3x-ui code is being extracted; this change is purely a
  local frontend platform swap.

## Decisions

### D1. Library lock-in

| Concern | Choice | Alternative rejected |
| --- | --- | --- |
| UI primitives | **AntD 5** | Material UI (MUI) — heavier theming overhead, less admin-panel-shaped. Mantine — smaller ecosystem, fewer ProTable equivalents. AntD Pro template — too much template noise; we want the components, not the scaffolding. |
| State (server) | **TanStack Query v5** | SWR — fewer mutation helpers. RTK Query — pulls in Redux, overkill for ~20 endpoints. Raw `useEffect` + `useState` — would re-introduce per-view loading/error duplication. |
| State (client) | **Zustand** | Redux Toolkit — overkill. React Context — works for theme/auth but no devtools, no selectors, easy to over-render. Pinia translates to Zustand most directly (both are stores-as-hooks). |
| Routing | **React Router v6** | TanStack Router — more powerful but immature for this size. Next.js — adds SSR overhead the embedded-SPA backend can't use. |
| i18n | **react-i18next** | FormatJS / react-intl — locale format is ICU; vue-i18n uses `{var}` interpolation, which react-i18next can consume directly. Lingui — small ecosystem. |
| Forms | **AntD `Form`** | react-hook-form — would force splitting form state from AntD's controlled inputs. AntD's `Form.useForm()` is good enough for every form in this app. |
| Date utility | **dayjs** | date-fns — works but AntD's `DatePicker` already imports `dayjs` adapter; sharing the lib avoids two date libs. |
| Build | **Vite + @vitejs/plugin-react** | Webpack — unnecessary. Next.js — SSR not wanted. Rsbuild — works but Vite is already the team's reflex. |
| Test (unit) | **Vitest + @testing-library/react** | Jest — Vitest reuses Vite config, faster cold start. RTL is the React equivalent of `@vue/test-utils`. |
| Test (e2e) | **Playwright (unchanged)** | Cypress — already on Playwright; selectors are the only change. |

### D2. Parallel directory + single cutover

The repo grows a sibling `frontend-react/` at the root, structured
identically to `frontend/`. During the rewrite window:

- `make dev` continues to launch the Vue version (`frontend/`).
- `make dev-react` (new) launches the React version against the
  same backend dev server, on a different port to avoid conflicts.
- The backend's `go:embed` directive points at `frontend/dist/` and
  is not touched during the rewrite.

Cutover is one commit:

```
rm -rf frontend
mv frontend-react frontend
# update Makefile dev target (drop dev-react, point dev at the new tree)
# update README screenshots if they show Vue chrome
```

After cutover the `dev-react` target is removed; `dev` is the only
target again.

**Alternative considered.** File-by-file in-place replacement
(running Vue and React side-by-side under the same `frontend/`
directory via a vue+react Vite dual-plugin). Rejected because: (a)
both ecosystems' eslint configs fight, (b) test runners would need
to discriminate by extension, (c) we'd ship a permanent vue/react
bridge that we'd then need to remove later — extra net work for no
gain in a greenfield project.

**Alternative considered.** Delete Vue tree first, build React in
its place from empty. Rejected because that takes the rewrite from
"comparable-side-by-side" to "build-and-pray." Having the Vue tree
visible while writing the React equivalent of each view is the
fastest path.

### D3. Server-state contract

Every existing axios endpoint (21 modules under
`frontend/src/api/{admin,portal,client}/*`) is ported as-is into
`frontend-react/src/api/`. On top of axios we add a thin
TanStack Query layer:

```ts
// src/api/admin/nodes.ts (ported, axios unchanged)
export const nodesApi = {
  list: () => http.get<Node[]>('/api/admin/nodes').then(r => r.data),
  // ...
}

// src/hooks/queries/admin/nodes.ts (new layer)
export function useNodesList() {
  return useQuery({
    queryKey: ['admin', 'nodes', 'list'],
    queryFn: nodesApi.list,
  })
}

export function useUpdateNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: nodesApi.update,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'nodes'] }),
  })
}
```

**Query key convention.** Three-segment minimum:
`[area, resource, op, ...args]`. `area` ∈ `{admin, portal, public}`;
`resource` matches the API module name; `op` ∈ `{list, get,
detail, ...}`. Mutations invalidate by `[area, resource]` prefix.

**Stale time.** Default 30s for list views, 5min for branding /
settings (rarely change), 0 for one-shot fetches (login, OIDC
callback). Override per-hook.

**Error normalization.** A single `axios` interceptor (ported
from the Vue tree) converts non-2xx into a normalized
`{ code, message, details }` shape; TanStack Query consumes the
unwrapped error.

### D4. Client-state stores

Five Pinia stores → five Zustand stores, 1:1:

| Pinia store | Zustand replacement | Persisted |
| --- | --- | --- |
| `stores/adminAuth.ts` | `stores/adminAuth.ts` | JWT in localStorage |
| `stores/portalAuth.ts` | `stores/portalAuth.ts` | JWT in localStorage |
| `stores/app.ts` | `stores/app.ts` | none (locale-derived ephemeral state) |
| `stores/branding.ts` | `stores/branding.ts` | TanStack Query cache (see note) |
| `stores/theme.ts` | `stores/theme.ts` | preference in localStorage |

**Branding note.** Branding is fetched from
`/api/public/branding`. In the React tree it lives as a TanStack
Query — not a Zustand store — because it's server state. The
`useBranding()` hook returns the cached payload synchronously after
the first fetch.

**JWT persistence.** Use `zustand/middleware`'s `persist` with
`localStorage`. Same key names as the Vue tree
(`3xui.adminAuth`, `3xui.portalAuth`) so a user logged in under
Vue stays logged in after cutover.

### D5. AntD theme tokens

Brand palette ported from `frontend/tailwind.config.ts`:

```ts
// frontend-react/src/theme.ts
export const lightTheme: ThemeConfig = {
  token: {
    colorPrimary: '#4f46e5',    // primary-600 in Tailwind config
    colorSuccess: '#10b981',    // accent-600
    colorWarning: '#f59e0b',
    colorError:   '#ef4444',
    borderRadius: 10,           // matches current `rounded-xl`-ish feel
    fontFamily: '"Geist Sans", system-ui, sans-serif',
  },
  algorithm: theme.defaultAlgorithm,
}

export const darkTheme: ThemeConfig = {
  ...lightTheme,
  algorithm: theme.darkAlgorithm,
}
```

The `theme` store decides which one is active; `<ConfigProvider>`
at the app root applies it. AntD CSS variables are enabled
(`cssVar: true`) so theme switches happen without re-renders.

**Geist Sans / Mono fonts** are retained (already in `package.json`
under `@fontsource/geist-*`). Imported once at app root.

### D6. Routing + auth guard

React Router v6 with one `<Routes>` tree:

```tsx
<Routes>
  <Route path="/login" element={<Login />} />
  <Route path="/oidc/callback" element={<OIDCCallback />} />
  <Route path="/admin" element={<ProtectedRoute area="admin"><AdminLayout /></ProtectedRoute>}>
    <Route index element={<Navigate to="/admin/status" replace />} />
    <Route path="status" element={<Overview defaultTab="status" />} />
    <Route path="stats" element={<Overview defaultTab="stats" />} />
    {/* ... */}
  </Route>
  <Route path="/portal" element={<ProtectedRoute area="portal"><PortalLayout /></ProtectedRoute>}>
    {/* ... */}
  </Route>
  <Route path="*" element={<NotFound />} />
</Routes>
```

`<ProtectedRoute area="admin">` reads the appropriate Zustand auth
store and either renders `children` or redirects to `/login`
with a `next=` query param. This is the React analog of the Vue
tree's `authGuard` navigation guard.

### D7. AntD vs Tailwind boundary

AntD owns: every form input, table, modal, drawer, button, menu,
dropdown, tag, alert, message, notification, popover, tooltip,
skeleton, empty state, badge, statistic, descriptions, card, list,
tabs, segmented, switch, radio, checkbox.

Tailwind utility classes are used **only** for layout patches that
AntD's `<Flex>` / `<Row>` / `<Col>` / `<Space>` don't cover
cleanly — e.g. a one-off `mt-2` or `gap-3` where adding a `<Space>`
would be heavier than the inline class. If a Tailwind class
appears more than 3 times for the same purpose, it gets promoted to
either an AntD theme token or a shared component.

We start with Tailwind installed (postcss layer only) and revisit
removing it entirely after P4 lands. If the heavy-view rewrites
don't need it, it gets dropped at cutover.

### D8. i18n key preservation

`react-i18next` consumes vue-i18n's existing message format
unchanged: both use `{var}` interpolation, both support nested
keys, both support pluralization via the same `n` parameter. The
locale files `frontend/src/i18n/locales/{zh,en}.ts` are copied
verbatim into `frontend-react/src/i18n/locales/`.

Module exports change shape slightly:

```ts
// Vue (current)
export default { admin: { ... }, portal: { ... } }

// React (i18next)
export const zh = { admin: { ... }, portal: { ... } }  // same content
```

A one-time codemod script (or just sed) handles the export-name
change.

**Validation.** A diff of the `zh.ts` / `en.ts` key set before and
after the rewrite must be empty. Add a CI guard: a script walks
both files and asserts the flattened key list is unchanged.

### D9. Build output contract

`frontend-react/vite.config.ts` configures:

```ts
build: {
  outDir: '../backend/internal/web/dist',
  emptyOutDir: true,
}
```

— same as the current Vue config. Backend's `go:embed dist` is in
`backend/internal/web/embed.go` and continues to pick up the same
files. Index file is `dist/index.html`; assets land under
`dist/assets/`. The backend's SPA fallback (any non-API route →
serve `index.html`) is unchanged.

### D10. Testing strategy

**Unit (Vitest + RTL).** Each view gets a `.spec.tsx` next to it,
ported 1:1 from the corresponding `.spec.ts` in the Vue tree.
- Mock axios via `vi.mock('@/api/...')` (same pattern as today).
- Mock TanStack Query by wrapping the component under test in a
  fresh `QueryClientProvider`.
- Mock React Router by wrapping in `<MemoryRouter>`.

**E2E (Playwright).** `e2e/smoke.spec.ts` keeps its scope (login →
nav to one admin page → assert content). Selectors update from
Vue's `[data-test=...]` (if any) to AntD's stable class names or
`data-testid` attributes added to React components.

**Coverage gate.** Test count parity is the bar: every spec file in
`frontend/src/views/**/*.spec.ts` has a corresponding
`frontend-react/src/views/**/*.spec.tsx` with ≥ as many test cases.

## Risks / Trade-offs

- **[Risk]** AntD's default visual language is meaningfully
  different from the current Tailwind look (sharper corners, less
  whitespace, different focus rings). Operators familiar with the
  current UI will see a major visual change at cutover.
  **→ Mitigation:** The user explicitly chose "full AntD design
  language" over "AntD components with current visual" in the
  planning question. Brand color tokens (primary / accent) port
  over so the navbar and primary buttons keep the same identity.
  Document the visual change in the release notes.

- **[Risk]** TanStack Query's cache lifetime defaults differ from
  the Vue tree's per-view `onMounted(reload)` pattern. Without
  tuning, some lists might appear stale (e.g. user just created a
  plan but it's not in the list yet).
  **→ Mitigation:** Mutations explicitly invalidate by query key
  prefix (D3). For lists that mutate from another tab, add
  `refetchOnWindowFocus: true`. Audit each heavy view (Users,
  Inbounds, Settings) during P4 to ensure cache invalidation paths
  exist for every action.

- **[Risk]** Form parity for the 4 heavy views. The Vue tree has
  hand-rolled validation; AntD `Form` rules are powerful but
  different in shape. A wrong port could silently lose a validator
  (e.g. password complexity, port range).
  **→ Mitigation:** Tests for the heavy views explicitly assert
  every validator's negative path. Where possible, validators move
  to a shared `src/validators/` module that both Vue (during
  transition) and React can call — but only if cheap; otherwise
  re-implement and rely on test coverage.

- **[Risk]** i18n key drift. A typo during the locale-file move
  could silently render `undefined` strings.
  **→ Mitigation:** CI script (D8) compares flattened key sets
  before/after. react-i18next is configured with
  `returnNull: false` and `keySeparator: '.'` so missing keys throw
  in dev mode.

- **[Risk]** Cutover commit is huge (delete Vue, rename React tree
  — touches every frontend file in the repo). Code review and git
  blame both suffer.
  **→ Mitigation:** Two commits at cutover. First: `rm -rf frontend`
  and `mv frontend-react frontend` in one commit, message is "🔥
  cutover: Vue → React/AntD". Second: Makefile / README / docker
  build updates. Reviewer reads the second; the first is mechanical.

- **[Trade-off]** Tailwind retention. Keeping it postcss-loaded
  costs ~12KB gzip on the bundle and one more config file, but
  saves "rewrite all spacing patches as `<Space>` props" work. We
  start with it kept and decide at the end of P4 whether to remove.

- **[Trade-off]** No SSR. AntD + React Router supports SSR but the
  backend `go:embed`s a pre-built bundle; SSR would mean a Node
  runtime in production. Not worth it for an admin dashboard.

## Migration Plan

1. **P0 (Scaffold)** lands `frontend-react/` with hello-world,
   AntD theme, vite proxy, type/lint config. CI runs the new tree's
   typecheck + test commands but does not yet block on them.
2. **P1 (Cross-cutting)** ports the API layer, i18n, stores, shared
   components. By the end of P1 a developer can write a new view
   without touching infra.
3. **P2 (Layout + routing)** lands the 3 layouts and route guard.
4. **P3 (Auth surface)** lands login flow end-to-end — at this
   point the React tree is independently demoable via `make
   dev-react`.
5. **P4 (Admin) + P5 (Portal)** run in parallel where possible.
   Each view rewrite is its own PR with its own spec port. Tests
   gate each PR.
6. **P6 (Tests)** is mostly continuous during P3–P5; this milestone
   is the residual cleanup + the test-count parity audit.
7. **P7 (Cutover)** is the single big commit + Makefile/README
   sweep. After this, only the React tree exists.

**Rollback strategy.** Until cutover, rollback is trivial — the
Vue tree is untouched. After cutover, rollback means
`git revert` the cutover commit; since no backend or database
schema is modified, that's a clean revert. Post-revert, the Vue
tree is restored and operators are back where they started.

## Open Questions

- **OQ-1.** Do we keep Tailwind at cutover or remove it? Decision
  deferred until P4 lands and we can measure how often Tailwind
  utility classes were actually needed.
- **OQ-2.** Does the cutover happen before or after the next planned
  product push (payment-gateway-stripe completion)? Recommendation:
  after stripe lands in the Vue tree, then port stripe-specific
  flows during P5. Otherwise we'd be doing stripe twice.
- **OQ-3.** Do we keep the existing Playwright e2e selectors stable
  by adding `data-testid` to React components, or rewrite selectors
  to use AntD's role-based semantics? Recommendation: `data-testid`
  for parity, role-based as a follow-up cleanup.
