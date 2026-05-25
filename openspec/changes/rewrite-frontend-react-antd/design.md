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
- Preserve every backend contract unless a milestone spec explicitly
  revises that contract during the pre-launch window. P5 intentionally
  revises the account/OIDC/email-verification backend contract; JWT,
  payment gateways, webhooks, and subscription URLs remain parity
  contracts.
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
- Backend code and database changes are allowed when a milestone spec
  explicitly calls them out. The project has not launched, so there is
  no production compatibility constraint. Unspecified backend
  contracts still remain parity-only.
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

### D10. Charting strategy: keep inline SVG, no chart library

OpsMonitor.vue currently renders four chart shapes — donut, line,
bars, dots — entirely as hand-written inline SVG (single
`<svg viewBox="0 0 120 48">` paths). No chart library is in the
Vue tree's `package.json`.

**Decision:** carry the inline-SVG approach into React. Each
chart becomes a tiny presentational component (e.g.
`<DonutGauge value pct />`, `<TrendLine points />`,
`<BarsPanel rows />`, `<DotsGrid grid />`) under
`components/charts/`. Each ~30-80 LOC, no dependencies, no theme
collisions.

**Alternative rejected: `@ant-design/charts`.** Adds ~150KB
gzipped and a second theme system (`@antv/g2plot`) on top of
AntD's token system. The current charts are small, decorative,
and don't need interactivity (no tooltips, no zoom). Not worth
the dependency.

**Alternative rejected: `recharts`.** Same complaint — heavy for
~4 chart shapes used in one view.

**Open path forward.** If OpsMonitor grows interactive tooling
(zoom-to-range, multi-series overlays, alert annotations), we
revisit — at that point `@ant-design/charts` becomes worth the
weight. Until then, inline SVG keeps the bundle lean and theme
control 100% local.

### D11. `<KeepAlive>` and `<Transition>` translation

The Vue tree's Overview.vue uses `<KeepAlive>` to preserve panel
state when switching tabs, and `<Transition mode="out-in">` for
the fade between panels. Users.vue uses `<Transition name="fade">`
in two places for revealing batch-action bars and slide-fade for
toast notifications. Plans.vue uses `<Transition name="fade">`
for purchase feedback.

**Decision on KeepAlive:** mount panels on first activation, keep
them mounted, hide inactive ones via `display: none`. This is
what the React Overview I sketched in P0 already does (lazy-add
to a `mounted` Set, render with `v-show`-equivalent). No third-
party library needed.

**Decision on Transition:** use AntD's built-in fade/slide
helpers (`motion` props on `Modal`, `Drawer`, `Tabs` — they
animate by default) for the common cases. For the custom places
(Users batch-action bar slide-in, toast slide-fade), use a
4-line CSS transition with a `data-show` attribute. No
`framer-motion`, no `react-transition-group`.

**Alternative rejected: `react-keep-alive` / `react-activation`.**
Both work but pull in tree-walking hacks that fight React 18's
strict mode. The `display: none` approach is dumb and works.

**Alternative rejected: `framer-motion`.** Overkill for fade in /
out animations. Adds ~50KB. Reach for it only if we ever need
choreographed multi-element animations, which the current UI
doesn't.

### D12. Vue tree freeze policy

During the rewrite window, the Vue tree is on a strict diet:

- **Allowed:** bugfixes, security patches, dependency security
  bumps.
- **Forbidden:** new views, new features, new endpoints from the
  backend that the Vue tree wires up.
- **Allowed but with cost:** if a new feature is urgent enough to
  ship before cutover, the same PR ports it to the React tree.
  No Vue-only features merge.

Enforcement is social — one author, one main branch, this policy
written in CLAUDE.md as a reminder. If this policy breaks down,
the cost is real: every Vue-only feature becomes catch-up work
in the React tree later, and cutover slides.

**Why this matters.** The dashboard-user-panel batch landed 8
days of feature work in late May; if a similar batch landed
during a 3-week React rewrite, the React tree would never catch
up without re-doing the same UX work twice. Better to slow
feature velocity for 3 weeks than to ship 5 weeks late.

### D13. Mobile responsive strategy

The Vue tree has real mobile chrome that the rewrite must preserve:

- **AdminLayout** uses `md:hidden` to swap the persistent sidebar
  for a hamburger + overlay drawer at viewports under ~768px.
  The mobile top bar appears with brand + hamburger; tapping the
  hamburger opens the same nav as a drawer.
- **PortalLayout** uses `lg:hidden` to render a fixed
  bottom-navigation bar at viewports under ~1024px, with the 5
  portal sections as bottom tabs. Above that breakpoint the bar
  is hidden and the standard sider is used.
- Multiple admin list views (Users, Nodes, ProvisioningPools)
  switch between desktop `<Table>` and mobile card layout via
  the `<ResponsiveListTable>` primitive introduced in
  `refactor-admin-portal-ui-primitives`.

**Decision on chrome.** AntD's `<Layout.Sider breakpoint="md"
collapsedWidth="0">` + `<Drawer>` combo replaces the current
`md:hidden` overlay pattern in AdminLayout. PortalLayout's
bottom-nav has no direct AntD primitive — we render it as a
fixed-position `<Menu mode="horizontal">` under `<lg`.

**Decision on list views.** A shared `<ResponsiveListTable>`
wrapper (under `components/common/`) accepts `columns` and an
optional `mobileCard` render prop. Above the breakpoint it
renders AntD `<Table>`; below it renders AntD `<List>` with the
card render prop. Every admin list view (Users / Nodes /
Inbounds / Webhooks / Plans / Orders / ProvisioningPools) uses
this wrapper, never raw `<Table>` directly. This solves the
"same component drifts" problem at the same time as solving
mobile.

**Breakpoint constants.** Match the Tailwind defaults the Vue
tree uses: `md = 768px`, `lg = 1024px`. Codified in
`src/theme.ts` as constants so views never hardcode pixel
values.

**Test coverage.** Each layout's `.spec.tsx` SHALL test both
breakpoints (using `window.matchMedia` mocks).
`<ResponsiveListTable>` SHALL have direct unit tests covering
the breakpoint swap.

### D14. Accessibility and performance budget

The Vue tree never set explicit a11y or performance targets;
"it works" was the bar. The rewrite is the right window to lock
in measurable budgets so future drift fails CI rather than
sliding silently.

**Accessibility budget.** AntD components ship with reasonable
ARIA defaults, but custom code (PageHeader, RefreshButton,
ResponsiveListTable, the inline-SVG charts in OpsMonitor)
needs explicit attention. The bar:

- Every `<svg role="img">` inline chart MUST have an
  `aria-label` derived from the chart title + summary stat
  (e.g. "节点流量趋势，过去 24 小时，峰值 4.2 GB/s").
- All AntD `<Drawer>` / `<Modal>` invocations MUST set
  `title` (AntD handles `aria-labelledby` from it).
- All interactive elements MUST be keyboard-reachable: tab
  order matches visual order, focus ring visible, ESC closes
  drawers/modals (AntD default).
- The mobile drawer (D13) MUST trap focus while open and
  return focus to the hamburger on close.
- A CI job runs `@axe-core/playwright` against the e2e smoke's
  three pages (login, /admin/status, /portal/subscription) and
  fails on serious or critical violations. WCAG AA targeting.

**Performance budget.** Numbers below are measured on the
production build served by the Go binary on a cold Chrome
cache, loopback localhost:

- Initial HTML response < 50ms (it's go:embed, this is cheap).
- Total JS gzipped < 500KB across all chunks. Vendor split
  per D9 (`vendor-react`, `vendor-antd`, `vendor-query`,
  `vendor-i18n`, `vendor-qrcode`, `vendor-misc`) so first paint
  fetches < 200KB of critical JS.
- Time to Interactive on the Login route < 1.5s on
  Mid-Tier-Mobile (CPU 4×, Network Slow 4G) Lighthouse profile.
- LCP on `/admin/status` (Overview) < 2.5s on same profile.
- A CI job runs Lighthouse against a built bundle and fails if
  any of the four metrics regresses by > 10% versus the
  committed baseline. Baseline updates require explicit PR
  approval of the regression.

**Why both.** a11y and perf are the two things that always
slide first when there's deadline pressure, and the easiest to
catch automatically. Once these gates are in CI, future PRs
either stay within budget or have to justify the regression
out loud.

**Open implementation choice.** Lighthouse CI is the canonical
fit but it adds a Chrome-headless step. Alternative is
`size-limit` (bundle-only, much cheaper) plus a manual
Lighthouse run gated by release. Default to size-limit for
bundle bytes, Lighthouse-CI optional. Decide before P6 starts.

### D15. API error → UI behavior taxonomy

The Vue tree's `src/utils/format.ts::formatError` already
codifies most of the categories below. The rewrite lifts this
into a contract so every view handles errors the same way,
no exceptions.

**Categories, source of truth, recommended UI.**

| Category | Detection | Recommended UI | Recovery |
|---|---|---|---|
| Network unreachable | `axios.isAxiosError && (code === 'ERR_NETWORK' || !response)` | AntD `notification.error` with backend-troubleshooting hint | Retry button issuing the same mutation |
| 400 validation | `response.status === 400`, body has field errors | Inline AntD `Form.Item` errors on the offending fields | User edits and re-submits |
| 401 auth expired | `response.status === 401` | Axios interceptor clears the auth store and routes to `/login?next=<current>` (D6) | Re-login |
| 403 permission denied | `response.status === 403` | Page-level `<Result status="403">` if the view itself is forbidden; AntD `message.error` if it's a single action | None within the view — operator must escalate |
| 404 resource gone | `response.status === 404` | Page-level `<Result status="404">` if loading the resource; `message.error` if it's a deletion of an already-gone resource (idempotent success) | Navigate back; for already-gone deletion, invalidate the list so the row disappears |
| 409 conflict | `response.status === 409` | Inline form error on the conflicting field (e.g. "name already exists") | User picks a different value |
| 422 validation range | `response.status === 422` | Inline form error like 400 | User edits |
| 429 rate limit | `response.status === 429` | AntD `notification.warning` with countdown if `Retry-After` is set, plain message otherwise | Auto-retry once after the delay, then surface to user |
| 500 server | `response.status === 500` | AntD `notification.error` with the backend message body verbatim + "view dashboard logs for detail" suffix | Manual retry; encourage operator to file an issue |
| 502 / 503 / 504 upstream | status in {502, 503, 504} | AntD `notification.error` with "上游节点不可达" template + the failing node name if available | Retry button; suggest checking the specific node's panel |

**Implementation rule.** Every TanStack Query `useQuery` /
`useMutation` SHALL pass its `onError` through a shared
`useErrorHandler()` hook that dispatches to the above table.
Views MUST NOT inline `try { … } catch (e) { setError(...) }`
with raw axios errors — the shared hook is the only renderer.

**Inline vs notification rule.** Mutations triggered by a form
submission render errors inline (`Form.Item` status). Mutations
triggered by a button click outside a form (delete row, toggle
status, batch action) render via `notification`. Queries that
fail at page load render as a page-level `<Result>` if the
whole page can't function; as a panel-level empty-state if only
one section depends on the data.

**Why this matters.** The Vue tree's pattern of "every view
puts `error.value = formatError(e)` and renders an inline red
banner" is uniform but ignores AntD's affordances (toast,
notification, Result, Form.Item). Lifting to the taxonomy lets
each error category use the right affordance without view
authors having to think about it every time.

**Carry-over of the formatError catalog.** The Chinese fallback
strings inside `formatError` (e.g. "连不上后端 — 检查 dashboard
服务是否在跑、本机网络是否通") move into i18n keys under
`errors.network.*` / `errors.http4xx.*` / `errors.http5xx.*`
so they participate in the locale system. Operators can read
these in English too.

### D16. Testing strategy

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

### D17. Account identity contract can evolve during P5

P5 is allowed to change the backend account identity model because
the service has not launched and there are no compatibility
constraints. The older single-provider `users.oidc_subject` contract
is replaced by:

- `users.email` as the only local login identifier.
- `users.password_hash` as a non-nullable credential. OIDC-created
  users must choose a local password during account completion.
- `users.display_name` as optional profile metadata; it is editable
  and not unique.
- `oidc_providers` for configured OIDC providers.
- `user_oidc_identities` for `(provider_key, subject)` bindings,
  including the provider-returned verified email for audit/display.

OIDC callback resolution is explicit:

1. Existing `(provider_key, subject)` binding issues a portal token.
2. Provider verified email matching an existing local account returns
   a pending completion token; binding that account requires the
   account password.
3. Creating a new local user from a pending OIDC callback requires a
   user-entered login email, email verification code/token, display
   name, and password. The local login email may differ from the
   provider email.

OIDC unlink is intentionally out of scope for P5. The Profile page
shows linked providers and can start link flows for unlinked
providers, but it does not remove identities.

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

- **[Trade-off resolved]** Tailwind is dropped at cutover. With
  the full-AntD design language choice (D1) and the mobile
  responsive strategy leaning on `<ResponsiveListTable>` +
  `<Drawer>` + `<Flex>` (D13), there's no Tailwind class that
  AntD's primitives can't replace. The ~12KB gzip saving plus
  one less config file plus zero "two-styling-system" confusion
  is worth the ~half-day of `<div class="...">` → `<Flex>` /
  `<Space>` translation effort in P4. No Tailwind dependency in
  `frontend-react/package.json` from P0 onward.

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

**Rollback strategy.** Until cutover, rollback is trivial for the
frontend — the Vue tree is untouched. P5 may add backend/database
account-identity changes before launch; those migrations are not a
production data-rollback concern because no deployed operator/user
data exists yet. After cutover, rollback means `git revert` the
cutover commit plus any still-unwanted pre-launch account-identity
migrations.

## Open Questions

- **OQ-1.** Do we keep the existing Playwright e2e selectors
  stable by adding `data-testid` to React components, or rewrite
  selectors to use AntD's role-based semantics? Recommendation:
  `data-testid` for parity in the first pass, role-based as a
  follow-up cleanup if/when accessibility audit happens.

## Resolved Questions

- **~~OQ-Tailwind~~** (resolved 2026-05-25): drop at cutover.
  See the Tailwind trade-off note above.
- **~~OQ-Stripe-timing~~** (resolved 2026-05-25): Stripe is
  already wired in the Vue tree (`portal/Plans.vue` calls
  `purchaseViaPayment('stripe', ...)` and `api/portal/billing.ts`
  knows the `'stripe'` `PaymentMethod`). Cutover order is no
  longer Stripe-blocked; P5 ports the existing Stripe flow as-is.
