# Tasks: rewrite-frontend-react-antd

## 1. P0 — Scaffold `frontend-react/`

- [x] 1.1 Create `frontend-react/` directory at repo root, init `package.json` with name `3xui-dashboard-frontend-react`
- [x] 1.2 Add dependencies: `react`, `react-dom`, `react-router-dom`, `antd`, `@ant-design/icons`, `zustand`, `@tanstack/react-query`, `react-i18next`, `i18next`, `axios`, `dayjs`, `qrcode`, `js-yaml`, `@fontsource/geist-sans`, `@fontsource/geist-mono`
- [x] 1.3 Add devDependencies: `vite`, `@vitejs/plugin-react`, `typescript`, `@types/react`, `@types/react-dom`, `@types/node`, `@types/qrcode`, `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`, `eslint`, `eslint-plugin-react`, `eslint-plugin-react-hooks`, `@typescript-eslint/parser`, `@typescript-eslint/eslint-plugin`, `@playwright/test`
- [x] 1.4 Write `frontend-react/vite.config.ts` with React plugin, `build.outDir = '../backend/internal/web/dist'`, `emptyOutDir: true`, dev proxy `/api → http://localhost:8080`
- [x] 1.5 Write `frontend-react/tsconfig.json` mirroring the Vue tree's strict settings + `jsx: "react-jsx"`
- [x] 1.6 Write `frontend-react/.eslintrc.cjs` with react / react-hooks / typescript plugins
- [x] 1.7 Write `frontend-react/index.html` + `frontend-react/src/main.tsx` with `<ConfigProvider>` + `<QueryClientProvider>` + `<BrowserRouter>` wiring a hello-world placeholder
- [x] 1.8 Write `frontend-react/src/theme.ts` exporting `lightTheme` / `darkTheme` ThemeConfigs with `colorPrimary` (indigo `#4f46e5`) and `colorSuccess` (green `#10b981`) ported from the Vue tree's tailwind config per D5
- [x] 1.9 Add `dev`, `build`, `typecheck`, `test`, `lint`, `preview`, `e2e` scripts to `package.json` (mirror Vue tree)
- [x] 1.10 Add `make dev-frontend-react` and `make build-frontend-react` targets in root `Makefile` (Vue `make dev` / `make build` stay untouched)
- [x] 1.11 Verify `npm run build` produces `backend/internal/web/dist/index.html` + `assets/*.{js,css}` (then `git restore` the dist files — backend still consumes Vue build until cutover)

## 2. P1 — Port API layer

- [x] 2.1 Port `frontend/src/api/client/factory.ts` → `frontend-react/src/api/client/factory.ts` (axios setup unchanged)
- [x] 2.2 Port `frontend/src/api/client/admin.ts` and `client/portal.ts` (axios instances + interceptors)
- [x] 2.3 Port `frontend/src/api/branding.ts` (public endpoint)
- [x] 2.4 Port `frontend/src/api/admin/*.ts` (10 modules: auth, nodes, inbounds, users, plans, orders, provisioningPools, stats, webhooks, settings, audit, index)
- [x] 2.5 Port `frontend/src/api/portal/*.ts` (5 modules: auth, billing, profile, traffic, index)
- [x] 2.6 Verify axios calls compile and types align with backend by running `npm run typecheck` (no React hooks consume them yet)

## 3. P1 — Wrap API in TanStack Query hooks

- [x] 3.1 Create `frontend-react/src/hooks/queries/` folder mirroring api/{admin,portal} structure
- [x] 3.2 For each admin api module, write `useXxxList()`, `useXxxDetail()`, etc. wrapping `useQuery` with the 3-segment query key convention from design D3
- [x] 3.3 For each admin api mutation, write `useCreateXxx()`, `useUpdateXxx()`, `useDeleteXxx()` wrapping `useMutation` with `onSuccess: invalidate [area, resource]`
- [x] 3.4 Repeat 3.2 + 3.3 for every portal api module
- [x] 3.5 Add a single `QueryClient` factory in `src/lib/queryClient.ts` with the stale-time defaults from design D3 (30s for lists, 5min for branding/settings)

## 4. P1 — Port i18n + utils

- [x] 4.1 Copy `frontend/src/i18n/locales/zh.ts` → `frontend-react/src/i18n/locales/zh.ts`, change `export default` to named `export const zh`
- [x] 4.2 Copy `en.ts` the same way
- [x] 4.3 Write `frontend-react/src/i18n/index.ts` initializing `i18next` with `{ resources: { zh: { translation: zh }, en: { translation: en } }, returnNull: false, keySeparator: '.', interpolation: { prefix: '{', suffix: '}' } }`
- [x] 4.4 Write a locale-parity script `frontend-react/scripts/check-locale-parity.mjs` that flattens both locale objects from Vue + React trees and asserts the key set diff is empty (CI guard)
- [x] 4.5 Port `frontend/src/utils/format.ts` → `frontend-react/src/utils/format.ts` verbatim (no Vue-specific imports)
- [x] 4.6 Delete `useConfirm` composable from the React tree's TODO list — use `Modal.confirm` directly

## 5. P1 — Port stores to Zustand

- [x] 5.1 Port `stores/adminAuth.ts` to Zustand with `persist` middleware, localStorage key `3xui.adminAuth` (matches Vue tree per spec scenario "Storage keys are stable across cutover")
- [x] 5.2 Port `stores/portalAuth.ts` to Zustand with `persist`, localStorage key `3xui.portalAuth`
- [x] 5.3 Port `stores/app.ts` (locale ephemeral state, no persist)
- [x] 5.4 Port `stores/theme.ts` to Zustand with `persist`, localStorage key matches Vue tree; expose `mode: 'light' | 'dark' | 'system'`
- [x] 5.5 Replace `stores/branding.ts` Pinia store with a `useBranding()` TanStack Query hook (server state, not client state per design D4)

## 6. P1 — Port shared components

- [x] 6.1 Replace `components/common/ConfirmModal.vue` callers with AntD `Modal.confirm` — no React component needed
- [x] 6.2 Replace `EmptyState.vue` with a thin wrapper around AntD `Empty` that accepts `icon`, `title`, `description`, `actionLabel`, `onAction` props
- [x] 6.3 Replace `Skeleton.vue` with a wrapper around AntD `Skeleton` that supports the existing `variant="kpi"` + `rows` prop shape used by Status/Stats panels
- [x] 6.4 Port `AccountMenu.vue` to React using AntD `Dropdown` + `Menu` items
- [x] 6.5 Port `LocaleSwitcher.vue` to React using AntD `Segmented` with locale tokens
- [x] 6.6 Add a new shared `PageHeader` component (title + subtitle + trailing actions slot) — fulfills the spec scenario "Page header is a single shared component"
- [x] 6.7 Add a shared `RefreshButton` wrapping AntD `<Button icon={<ReloadOutlined />}>` — fulfills "Refresh button identical on every page"
- [x] 6.8 Add a shared `ResponsiveListTable` wrapper that swaps AntD `<Table>` (desktop) ↔ AntD `<List>` + card render-prop (mobile) at the `MD_BREAKPOINT` boundary defined in `src/theme.ts` per design D13. Sets `data-component="responsive-list-table"` on its root for tests

## 7. P2 — Layouts + routing

- [x] 7.1 Write `frontend-react/src/components/layout/AdminLayout.tsx` using AntD `<Layout>` + `<Sider>` + `<Header>` + `<Content>`; sidebar `<Menu>` mirrors the section/items structure from the Vue AdminLayout; below the `MD_BREAKPOINT` swap the persistent sider for a hamburger + `<Drawer>` (per D13)
- [x] 7.2 Write `PortalLayout.tsx` similarly; below `LG_BREAKPOINT` render a fixed-position bottom-nav `<Menu mode="horizontal">` with 5 portal sections (per D13)
- [x] 7.3 Write `AuthLayout.tsx` (centered card shell used by Login + OIDCCallback)
- [x] 7.4 Write `<ProtectedRoute area="admin" | "portal">` HOC that reads the appropriate Zustand auth store and redirects to `/login?next=...` when unauthenticated
- [x] 7.5 Write `frontend-react/src/router.tsx` mirroring `frontend/src/router/index.ts` paths 1:1; `/admin` redirects to `/admin/status`, `/portal` redirects to `/portal/subscription`
- [x] 7.6 Verify navigation between paths preserves the active sidebar highlight (`useLocation` + `<Menu selectedKeys>`)

## 8. P3 — Auth surface

- [x] 8.1 Port `Login.vue` → `Login.tsx` using AntD `Form`, supports password login + OIDC button; reads `next=` query param and routes there on success; preserve the post-failure `cooldownTimer` (anti-spam delay between failed attempts) — translate `setInterval` to a `useEffect` countdown
- [x] 8.2 Port `OIDCCallback.vue` → `OIDCCallback.tsx` (handles `code=&state=`, calls `/api/user/auth/oidc/callback`, stores JWT, navigates to portal)
- [x] 8.3 Port `NotFound.vue` → `NotFound.tsx` (AntD `Result` with `status="404"`)
- [ ] 8.4 Smoke test `make dev-frontend-react` end-to-end: open `/login`, enter admin creds, land on `/admin/status`

## 9. P4 — Admin views (light tier)

- [x] 9.1 Port `admin/Plans.vue` (360 LOC) → `Plans.tsx` using AntD `Table` + `Modal` form for create/edit
- [x] 9.2 Port `admin/Orders.vue` (232) → `Orders.tsx` (read-only `Table` + filters)
- [x] 9.3 Port `admin/AuditLog.vue` (227) → `AuditLog.tsx` (read-only `Table` with severity tag, pagination); preserve the 300ms search-input debounce (use `useDeferredValue` or a manual `setTimeout` in `useEffect`)
- [x] 9.4 Port `admin/ProvisioningPools.vue` (454) → `ProvisioningPools.tsx`

## 10. P4 — Admin views (medium tier)

- [x] 10.1 Port `admin/Overview.vue` + `Status.vue` + `Stats.vue` (750 LOC combined) → `Overview.tsx` with internal `Tabs`; tab state driven by route path; refresh button delegates to active panel via ref or query refetch; KeepAlive→`mounted Set + display:none` (per D11), Transition→AntD Tabs built-in motion (per D11)
- [x] 10.2 Port `admin/Nodes.vue` (543) → `Nodes.tsx` with `Table` + create/edit `Drawer` + status badge
- [x] 10.3 Port `admin/Webhooks.vue` (504) → `Webhooks.tsx` — replace `useConfirm` callsite with `Modal.confirm`; accept an `embedded?: boolean` prop so the same component can serve both `/admin/webhooks` (full chrome) and `/admin/settings?tab=notifications` (no header)
- [x] 10.4 Port `admin/OpsMonitor.vue` (658) → `OpsMonitor.tsx` at `/admin/ops-monitor` — KPI cards, per-node metric trend, four analysis panels (bars / line / stack / dots); charts go to `src/components/charts/` as inline-SVG components (DonutGauge / TrendLine / BarsPanel / DotsGrid) per D10; partial-failure handling preserves the Vue tree's `metricError` semantics

## 11. P4 — Admin views (heavy tier)

- [x] 11.1 Port `admin/Users.vue` (1300) → `Users.tsx` with `Table` + `rowSelection` for batch ops + filter chips + status toggles; preserve `autoRefreshTimer` (admin-side auto-refresh) via TanStack Query `refetchInterval`; preserve flash/toast timeout (translate `setTimeout` to a controlled `useEffect` cleanup)
- [x] 11.2 Port `admin/Inbounds.vue` (1282) → `Inbounds.tsx` (list view, links to editor); preserve the QR generation path (`qrcode.toDataURL` → AntD QR via the same `qrcode` lib or via AntD's `<QRCode>` component)
- [x] 11.3 Port `admin/InboundEditorModal.vue` (1178) → `InboundEditor.tsx` as a `Drawer`; split the 6 protocols (vless / vmess / trojan / shadowsocks / hysteria / wireguard) into separate files under `src/views/admin/inbound-editor/protocols/`, one component per protocol with its full field set
- [x] 11.4 Port `admin/Settings.vue` (1565) → `Settings.tsx` with `Tabs` for the **8** sections (general / subscription / alerts / dataCollection / securityAuth / userDefaults / messages / notifications); each tab is a separate file under `src/views/admin/settings/`. `DataCollectionSettings` was already split in the Vue tree — port it. `NotificationsSettings` is a thin wrapper around `<Webhooks embedded />` so the form code lives in one place. Include the favicon file upload (`<input type="file" accept="image/...">` + FormData POST)

## 12. P5 — Portal views

- [x] 12.1 Port `portal/Subscription.vue` (289) → `Subscription.tsx` (sub URL display, QR via `qrcode` library, copy buttons)
- [x] 12.2 Port `portal/Dashboard.vue` (249) → `Usage.tsx` (traffic stats, progress bars)
- [x] 12.3 Port `portal/Plans.vue` (305) → `Plans.tsx` (purchase flow)
- [x] 12.4 Port `portal/Orders.vue` (243) → `Orders.tsx`
- [ ] 12.5 Port `portal/Profile.vue` (392) → `Profile.tsx` (email change, password change, OIDC linking)
- [x] 12.6 Port `components/portal/AlipayPayModal.vue` → `AlipayPayModal.tsx` — QR generation via the same `qrcode` lib (or AntD `<QRCode>`), payment-status polling at 3-second interval (matches Vue tree), countdown timer, both polling and countdown cleanup on modal close
- [x] 12.7 Replace `useConfirm` callsites in `portal/Plans.tsx` and `portal/Subscription.tsx` with `Modal.confirm` (mirrors the admin-side change)

## 13. P6 — Tests

- [ ] 13.1 Port the 13 admin view specs (Status, Stats, Overview, Plans, ProvisioningPools, Nodes, Users, Webhooks, Settings, Orders, InboundEditorModal, OpsMonitor, AuditLog, settings/DataCollectionSettings — actually 14 once the sub-folder spec is counted)
- [ ] 13.2 Port the 3 portal view specs (`Plans`, `Profile`, plus any other present at P6 entry)
- [ ] 13.3 Port `views/Login.spec.ts` → `Login.spec.tsx`
- [ ] 13.4 Port the 4 shared-component specs (`AccountMenu`, `EmptyState`, `Skeleton`, plus the `AlipayPayModal` portal-modal spec). Drop `ConfirmModal.spec.ts` — component no longer exists on the React side (per P1, replaced by `Modal.confirm`)
- [ ] 13.5 Port `components/layout/AdminLayout.spec.ts` → `AdminLayout.spec.tsx`
- [ ] 13.6 Port `router/index.spec.ts` → equivalent React spec covering ProtectedRoute redirect cases (anonymous-admin / default-entry-no-next / portal-session-does-not-satisfy-admin)
- [ ] 13.7 Drop `composables/useConfirm.spec.ts` — composable removed per P1; add to the parity script's exclusion list (no React counterpart expected)
- [ ] 13.8 Wrap each spec's `render` call in `QueryClientProvider` + `MemoryRouter` helper (extract to `src/test-utils/renderWithProviders.tsx`)
- [ ] 13.9 Use `vi.mock('@/api/...')` for axios mocking (same pattern as Vue tree)
- [ ] 13.10 Update `e2e/smoke.spec.ts` selectors to match React DOM output; add `data-testid` attributes where stable selectors are needed
- [ ] 13.11 Verify test-count parity: every Vue `.spec.ts` (except the documented exclusion) has a React `.spec.tsx` with ≥ as many `it(...)` blocks
- [ ] 13.12 `npm run typecheck` and `npm run test` both pass in `frontend-react/`

## 14. P7 — Cutover

- [ ] 14.1 Final feature-parity audit: manually walk every route in both Vue and React versions side-by-side, file any gaps as blockers
- [ ] 14.2 Run locale-parity script (task 4.4) — diff must be empty
- [ ] 14.3 Run test suite + e2e smoke in `frontend-react/` — all green
- [ ] 14.4 Commit 1 (the cutover): `rm -rf frontend && mv frontend-react frontend`; message: "🔥 cutover: Vue → React/AntD"
- [ ] 14.5 Commit 2 (sweep): update root `Makefile` (drop `dev-react`, point `dev` and `build` at the new tree), update `README.md` (any Vue references / screenshots), update `docs/` and `deploy/` build references, update CI workflow if any
- [ ] 14.6 Build and run the backend binary; verify `index.html` loads the React app, an admin login round-trip works, and `dist/assets/` paths resolve
- [ ] 14.7 Archive this OpenSpec change via `openspec archive rewrite-frontend-react-antd`
