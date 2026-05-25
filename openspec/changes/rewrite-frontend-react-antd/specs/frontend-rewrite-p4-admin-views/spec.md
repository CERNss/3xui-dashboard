# frontend-rewrite-p4-admin-views

P4 milestone. Ports the 13 admin views from the Vue tree to React,
each backed by AntD's `Table` / `Form` / `Drawer` / `Modal`
primitives and the TanStack Query hooks from P1. Tiered by size:
light (1d each), medium (1.5d each), heavy (2–3d each).

**Entry criteria.** P3 (`frontend-rewrite-p3-auth-surface`) is
complete. Login → AdminLayout → placeholder Overview round-trip
works.

**Exit criteria.** Every requirement below holds. Every admin
route in the React tree shows a working view at parity with the
Vue tree's behavior. `make dev-frontend-react` is a fully
demoable admin SPA.

## ADDED Requirements

### Requirement: Every admin route is backed by a real view

The React tree SHALL replace every P2 placeholder element with a
real view component for the following routes:
`/admin/status` and `/admin/stats` (both rendered by Overview
with different default tabs), `/admin/nodes`, `/admin/inbounds`,
`/admin/users`, `/admin/plans`, `/admin/provisioning-pools`,
`/admin/orders`, `/admin/audit-log`, `/admin/ops-monitor`,
`/admin/settings`.

#### Scenario: Each route resolves to a non-placeholder component

- **GIVEN** an authenticated admin
- **WHEN** the operator navigates to any admin route above
- **THEN** the rendered component SHALL NOT be a placeholder
- **AND** the page SHALL fetch its data via the corresponding
  TanStack Query hook from P1
- **AND** the page SHALL render the result (table, KPI strip,
  form, depending on the view)

### Requirement: List views go through `<ResponsiveListTable>`, not raw `<Table>`

Every admin list view SHALL render through the shared
`<ResponsiveListTable>` wrapper (introduced in P1) which renders
AntD `<Table>` above the `md` breakpoint and AntD `<List>` with
card render-prop below it. The wrapper preserves the columns,
sort, and filter affordances the Vue tree exposed. Hand-rolled
`<Table>` or `<table>` inside view files is forbidden.

#### Scenario: Nodes list uses ResponsiveListTable

- **GIVEN** the operator is on `/admin/nodes` at desktop width
- **WHEN** the page renders
- **THEN** the DOM SHALL contain an AntD `<Table>` rendered
  through `<ResponsiveListTable>` (recognizable by its wrapper
  `data-component="responsive-list-table"` attribute)
- **AND** every column from the Vue Nodes view SHALL be present
  (name, status, cpu/mem, xray version, last seen)
- **AND** the row count SHALL match the underlying API response

#### Scenario: Nodes list collapses to cards on mobile

- **GIVEN** `window.matchMedia('(min-width: 768px)').matches`
  returns `false`
- **WHEN** the operator opens `/admin/nodes`
- **THEN** `<ResponsiveListTable>` SHALL render AntD `<List>`
  with one card per row instead of `<Table>`
- **AND** each card SHALL contain the same data as the desktop
  row (name, status, cpu/mem, xray version, last seen) — no
  data SHALL be hidden by the responsive swap

#### Scenario: Users supports row selection for batch operations

- **GIVEN** the operator is on `/admin/users`
- **WHEN** the operator selects multiple rows via the table's
  selection column
- **THEN** the batch-action buttons (delete / suspend / unsuspend)
  SHALL become enabled
- **AND** invoking a batch action SHALL operate on exactly the
  selected user IDs (matches Vue tree's `batch.*` behavior)

### Requirement: Forms uniformly use AntD `<Form>`

Every create / edit dialog SHALL use AntD's `<Form>` with
`Form.useForm()`. Validators SHALL preserve the Vue tree's
field-level constraints (required fields, port ranges, email
shape, password complexity).

#### Scenario: Plan create form rejects invalid input

- **GIVEN** the operator opens the "New Plan" dialog on
  `/admin/plans`
- **WHEN** the operator submits with an empty name or non-numeric
  price
- **THEN** the form SHALL display per-field errors
- **AND** SHALL NOT issue a POST

#### Scenario: Node edit dialog preserves Vue tree's validators

- **GIVEN** the operator opens the edit dialog for a node
- **WHEN** the operator enters a port outside 1–65535
- **THEN** the field SHALL show a validation error
- **AND** the operator SHALL be unable to submit until the field
  is corrected

### Requirement: Overview (`/admin/status` + `/admin/stats`) renders both panels under tabs

`Overview.tsx` SHALL render the same two-tab layout the Vue
tree's `Overview.vue` introduced on 2026-05-24: a shared header
with title/subtitle that swap per active tab, a single shared
refresh button, and tab content for Status (fleet health) and
Stats (operational KPI). The route path SHALL drive the default
tab.

#### Scenario: `/admin/status` defaults to the status tab

- **WHEN** the operator navigates to `/admin/status`
- **THEN** the Status tab SHALL be active
- **AND** the page SHALL show the KPI strip (nodes / inbounds /
  clients / traffic) plus the node health table

#### Scenario: `/admin/stats` defaults to the stats tab

- **WHEN** the operator navigates to `/admin/stats`
- **THEN** the Stats tab SHALL be active
- **AND** the page SHALL show the KPI strip (month new users /
  total users / month upload / month download) plus the traffic
  rankings + system log strip

#### Scenario: Refresh button refetches only the active tab

- **GIVEN** the Status tab is active and both panels have loaded
- **WHEN** the operator clicks the shared refresh button
- **THEN** only the Status panel's queries SHALL refetch
- **AND** the Stats panel's data SHALL remain in cache

### Requirement: Settings is tabbed and matches the Vue 8-section layout

`Settings.tsx` SHALL render AntD `<Tabs>` with the same eight
sections as the Vue tree (general / subscription / alerts /
dataCollection / securityAuth / userDefaults / messages /
notifications). Each tab's body SHALL live in a separate file
under `src/views/admin/settings/` so individual sections can be
reviewed/edited independently.

#### Scenario: Eight tabs present and match Vue labels

- **WHEN** the operator navigates to `/admin/settings`
- **THEN** the tabs SHALL be in the same order as Vue:
  `general`, `subscription`, `alerts`, `dataCollection`,
  `securityAuth`, `userDefaults`, `messages`, `notifications`
- **AND** the labels SHALL come from the same `admin.settings.*Tab`
  i18n keys

#### Scenario: Tab state survives via `?tab=` query

- **GIVEN** the operator is on `/admin/settings?tab=messages`
- **WHEN** the page mounts
- **THEN** the active tab SHALL be `messages`
- **AND** when the operator clicks `notifications`, the URL
  SHALL update to `/admin/settings?tab=notifications` (matching
  Vue tree's pattern)

#### Scenario: Notifications tab embeds the Webhooks view

- **GIVEN** the operator opens `/admin/settings?tab=notifications`
- **WHEN** the tab body renders
- **THEN** it SHALL render the same component used by the
  top-level `/admin/webhooks` route (`<Webhooks />`)
- **AND** the embedded instance SHALL receive an `embedded`
  prop (or equivalent context) that hides the redundant
  `PageHeader` so the Settings tab's own header is the only
  one visible
- **AND** mutations issued from inside the embedded Webhooks
  view SHALL invalidate the same query keys as the standalone
  `/admin/webhooks` route, so both surfaces stay in sync

### Requirement: Inbounds + InboundEditor split into list + drawer with 6 protocol sub-forms

The React tree SHALL implement `Inbounds.tsx` as the list view (`<Table>`) and `InboundEditor.tsx` as an AntD `<Drawer>` containing an AntD `<Form>` with six protocol-specific sub-forms — vless, vmess, trojan, shadowsocks, hysteria, wireguard — matching the protocol surface in the Vue tree's `InboundEditorModal.vue` (1178 LOC). Each protocol's sub-form lives in its own file under `src/views/admin/inbound-editor/protocols/`.

#### Scenario: Drawer opens with full inbound payload

- **GIVEN** the operator clicks "Edit" on an inbound row
- **WHEN** the drawer opens
- **THEN** the form SHALL be populated with the inbound's current
  values (id, tag, port, protocol, settings, stream settings,
  client list)
- **AND** every editable field present in the Vue editor SHALL
  be present here

#### Scenario: vless sub-form covers flow, decryption, clients

- **GIVEN** the operator opens the editor on a vless inbound
- **WHEN** the drawer renders
- **THEN** the protocol sub-form SHALL include `decryption`
  (defaulting to `none`), the client list with `id`, `flow`,
  `email`, `expiryTime`, `enable`, and the stream-settings
  controls
- **AND** every field present in the Vue tree's vless branch
  SHALL be present

#### Scenario: vmess sub-form covers client list + alterId

- **GIVEN** the operator opens the editor on a vmess inbound
- **WHEN** the drawer renders
- **THEN** the protocol sub-form SHALL include the client list
  with `id`, `email`, `expiryTime`, `enable`, and any
  vmess-specific stream controls present in the Vue tree

#### Scenario: trojan sub-form covers password list

- **WHEN** the operator opens the editor on a trojan inbound
- **THEN** the protocol sub-form SHALL render the password
  list with `password`, `email`, `expiryTime`, `enable` per row

#### Scenario: shadowsocks sub-form covers method + password

- **WHEN** the operator opens the editor on a shadowsocks inbound
- **THEN** the protocol sub-form SHALL render the method
  selector, the global password, and the per-client field set
  the Vue tree exposes

#### Scenario: hysteria sub-form covers obfuscation + auth

- **WHEN** the operator opens the editor on a hysteria inbound
- **THEN** the protocol sub-form SHALL render hysteria-specific
  fields (obfs, auth string, up/down mbps) per the Vue tree

#### Scenario: wireguard sub-form covers peer table + secret keys

- **WHEN** the operator opens the editor on a wireguard inbound
- **THEN** the protocol sub-form SHALL render the peer table and
  the secret-key controls the Vue tree exposes

#### Scenario: Drawer save persists via the mutation hook

- **GIVEN** the operator edits a field and clicks Save
- **WHEN** the form submits
- **THEN** it SHALL invoke the corresponding mutation hook
  (`useUpdateInbound`)
- **AND** on success the drawer SHALL close
- **AND** the parent list SHALL refetch via the P1 invalidation
  contract

### Requirement: OpsMonitor is a heavy admin view with inline-SVG charts

The React tree SHALL ship `OpsMonitor.tsx` at `/admin/ops-monitor`, mirroring the Vue tree's `OpsMonitor.vue` (658 LOC) — KPI cards, per-node metric trend lines, and the four analysis panels (bars / line / stack / dots). Charts SHALL be inline-SVG presentational components under `src/components/charts/`, not a third-party chart library (per design D10).

#### Scenario: OpsMonitor mounts and fetches metric series

- **GIVEN** the operator navigates to `/admin/ops-monitor`
- **WHEN** the page renders
- **THEN** the page SHALL fetch the per-enabled-node metric
  series via the TanStack Query hook wrapping the Vue tree's
  metric fetch
- **AND** SHALL render a KPI strip and the resource-trend SVG

#### Scenario: Chart components live under `components/charts/`

- **WHEN** the operator inspects `frontend-react/src/components/charts/`
- **THEN** the directory SHALL contain at least these
  presentational components: `DonutGauge`, `TrendLine`,
  `BarsPanel`, `DotsGrid`
- **AND** none of them SHALL import `@ant-design/charts`,
  `recharts`, or any other charting library

#### Scenario: Metric fan-out preserves partial-failure handling

- **GIVEN** the OpsMonitor fetches per-node metrics across N
  nodes and 1 node times out
- **WHEN** the page renders
- **THEN** the N-1 healthy series SHALL render normally
- **AND** the failing node SHALL show its error inline (same
  behavior as the Vue tree's `metricError` ref)

### Requirement: Settings is data-driven, not hand-rolled field-by-field

Each Settings tab body SHALL render its fields from
`SettingItem[]` fetched from `/api/admin/settings` (the existing
backend endpoint), not by hand-writing per-field JSX. A shared
`SettingRow` component SHALL render the input control for one
SettingItem; tab bodies are essentially filtered lists of
SettingItems plus tab-specific composite widgets (favicon
upload, message-template preview, OIDC provider list).

This matters because adding a new operator-tunable setting on
the backend (a new row in `app_settings`) MUST become a new
visible field in the UI without touching React view code — only
the SettingItem catalog grows.

#### Scenario: Tab body is filtered SettingItems plus composites

- **GIVEN** the operator opens `/admin/settings?tab=general`
- **WHEN** the tab body renders
- **THEN** it SHALL fetch `SettingItem[]` from the settings API
  via `useSettingsList()`
- **AND** SHALL filter to items whose `group` (or equivalent
  taxonomy field) matches `general`
- **AND** SHALL render each filtered item through `<SettingRow>`
- **AND** composite UI (e.g. favicon file picker, brand color
  picker) SHALL be additional siblings, not replacements for
  the SettingRow list

#### Scenario: Drafts buffer matches the Vue tree's per-key edits

- **GIVEN** a SettingRow holds the current saved value
- **WHEN** the operator edits the field
- **THEN** the edit SHALL accumulate in a `drafts` map keyed by
  setting `key`, not mutate the loaded item directly
- **AND** the save button on that row SHALL be enabled only
  while `drafts[key]` differs from the loaded value
- **AND** clicking save SHALL invoke a `useUpdateSetting()`
  mutation that PUTs the new value and invalidates the settings
  query

#### Scenario: A new backend setting key appears without code change

- **GIVEN** the backend adds a new row to `app_settings` (say
  `ops_collect_max_jitter_seconds`) with a value in the
  `dataCollection` group
- **WHEN** the operator loads `/admin/settings?tab=dataCollection`
- **THEN** the new field SHALL appear in the React UI without
  any React code change (the SettingItem catalog is the
  contract; the UI is its renderer)
- **AND** the help-text path SHALL be derivable from the
  setting `key` (Vue tree uses a `helpPaths` map; the React
  tree SHOULD use the same map or migrate to a backend-provided
  help-key field)

### Requirement: Settings tabs split into `settings/` sub-folder files

`Settings.tsx` SHALL drive the AntD `<Tabs>` shell and route each tab body to a file under `src/views/admin/settings/`. The Vue tree has already started this split (`settings/DataCollectionSettings.vue` exists); the React tree SHALL complete it for all eight tabs.

#### Scenario: Each tab body is its own file

- **WHEN** the operator lists `frontend-react/src/views/admin/settings/`
- **THEN** the directory SHALL contain one `*.tsx` per tab —
  `GeneralSettings`, `SubscriptionSettings`, `AlertsSettings`,
  `DataCollectionSettings`, `SecurityAuthSettings`,
  `UserDefaultsSettings`, `MessagesSettings`,
  `NotificationsSettings`
- **AND** `Settings.tsx` itself SHALL contain only the tab
  shell + active-tab dispatch, no field-level form code
- **AND** `NotificationsSettings.tsx` SHALL re-export or thinly
  wrap the `<Webhooks embedded />` component rather than
  duplicating webhook-form code

### Requirement: Every view uses the shared `PageHeader` and `RefreshButton`

No admin view SHALL hand-roll its own header bar or refresh
button. The two shared primitives from P1 (`PageHeader`,
`RefreshButton`) SHALL be the only sources.

#### Scenario: Audit of admin views

- **GIVEN** the React tree at P4 exit
- **WHEN** the operator greps `frontend-react/src/views/admin/`
  for `<h1` or `Button.*Reload` outside the shared components
- **THEN** the only matches SHALL be inside
  `components/common/PageHeader.tsx` and
  `components/common/RefreshButton.tsx` themselves
- **AND** no view file SHALL appear in the match list

### Requirement: Each admin view owns a documented i18n prefix

Every React admin view SHALL consume i18n keys exclusively from its documented prefix (plus shared `common.*` / `nav.*` / `app.*` keys). The prefix-to-view mapping below is the parity contract: the React tree's view SHALL use every key currently present under that prefix in the Vue tree's `zh.ts` / `en.ts`, no additions, no removals.

| View | Owned prefix | Notes |
|---|---|---|
| Overview (status default tab) | `admin.status.*` + `admin.stats.*` | Aggregates both Status and Stats key sets; tab labels also pull from `nav.status` / `nav.stats` |
| Nodes | `admin.nodes.*` | Includes `admin.nodes.status.{online,offline,unknown}` |
| Inbounds | `admin.inbounds.*` | List view; share confirm strings with InboundEditor |
| InboundEditor (drawer) | `admin.inboundEditor.*` | Per-protocol field labels |
| Users | `admin.users.*` | Includes batch op strings under `admin.users.batch.*` |
| Plans | `admin.plans.*` | Both list + create/edit modal keys |
| Orders | `admin.orders.*` | KPI strip under `admin.orders.kpi*`; status labels under `admin.orders.status.*` |
| ProvisioningPools | `admin.provisioningPools.*` | |
| AuditLog | `admin.auditLog.*` | Severity labels under `admin.auditLog.severity.*`, filter labels under `admin.auditLog.filter*` |
| OpsMonitor | `admin.opsMonitor.*` | Chart labels + telemetry hints |
| Webhooks | `admin.webhooks.*` | Reused by both `/admin/webhooks` and Settings → notifications |
| Settings | `admin.settings.*` | Tab labels under `admin.settings.{tab}Tab`, descriptions under `admin.settings.{tab}Desc`, group labels under `admin.settings.group*` |

#### Scenario: View only references its owned prefix

- **GIVEN** the React Nodes view source
- **WHEN** grepped for `t(['"]`
- **THEN** every matched key SHALL start with `admin.nodes.`,
  `common.`, `nav.`, or `app.`
- **AND** no `admin.users.*` or `admin.plans.*` key SHALL leak
  into the Nodes view (cross-view borrowing is the most common
  source of silent breakage at locale refactors)

#### Scenario: Parity script catches a dropped owned key

- **GIVEN** the Vue tree has `admin.nodes.lastSeen` and the
  React tree's Nodes view never references it (e.g. the column
  was renamed during the port)
- **WHEN** the locale-parity script runs (P1 task 4.4)
- **THEN** the key SHALL still be present in
  `frontend-react/src/i18n/locales/zh.ts`
- **AND** a follow-up "unused-key" lint MAY flag it for
  deletion in a separate cleanup change — but NOT during the
  rewrite, because preserving keys is the rewrite's contract

### Requirement: P4 view specs pass with parity to Vue tree

Every view ported in P4 SHALL ship a `.spec.tsx` whose `it(...)`
count meets or exceeds the corresponding Vue `.spec.ts`.

#### Scenario: P4 test suite passes

- **WHEN** the operator runs `npm run test -- src/views/admin`
- **THEN** all specs SHALL pass
- **AND** for each Vue spec under `frontend/src/views/admin/`,
  the React equivalent SHALL contain at least as many `it(...)`
  blocks
