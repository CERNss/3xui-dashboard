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
`/admin/status` (Overview), `/admin/stats` (Overview default-tab
stats), `/admin/nodes`, `/admin/inbounds`, `/admin/users`,
`/admin/plans`, `/admin/provisioning-pools`, `/admin/orders`,
`/admin/audit-log`, `/admin/settings`.

#### Scenario: Each route resolves to a non-placeholder component

- **GIVEN** an authenticated admin
- **WHEN** the operator navigates to any admin route above
- **THEN** the rendered component SHALL NOT be a placeholder
- **AND** the page SHALL fetch its data via the corresponding
  TanStack Query hook from P1
- **AND** the page SHALL render the result (table, KPI strip,
  form, depending on the view)

### Requirement: Tables uniformly use AntD `<Table>`

Every list view SHALL use AntD's `<Table>` for tabular data, with
the same columns and sort/filter affordances the Vue tree
exposed. Hand-rolled `<table>` elements in JSX are forbidden in
admin views.

#### Scenario: Nodes list uses AntD Table

- **GIVEN** the operator is on `/admin/nodes`
- **WHEN** the page renders
- **THEN** the DOM SHALL contain an AntD `<Table>` (recognizable
  by its CSS class)
- **AND** every column from the Vue Nodes view SHALL be present
  (name, status, cpu/mem, xray version, last seen)
- **AND** the row count SHALL match the underlying API response

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

### Requirement: Settings is tabbed and matches the Vue 7-section layout

`Settings.tsx` SHALL render AntD `<Tabs>` with the same seven
sections as the Vue tree (general / subscription / alerts /
securityAuth / userDefaults / messages / notifications). Each
tab's body SHALL live in a separate file under
`src/views/admin/settings/` so individual sections can be
reviewed/edited independently.

#### Scenario: Seven tabs present and match Vue labels

- **WHEN** the operator navigates to `/admin/settings`
- **THEN** the tabs SHALL be in the same order as Vue:
  `general`, `subscription`, `alerts`, `securityAuth`,
  `userDefaults`, `messages`, `notifications`
- **AND** the labels SHALL come from the same `admin.settings.*Tab`
  i18n keys

#### Scenario: Tab state survives via `?tab=` query

- **GIVEN** the operator is on `/admin/settings?tab=messages`
- **WHEN** the page mounts
- **THEN** the active tab SHALL be `messages`
- **AND** when the operator clicks `notifications`, the URL
  SHALL update to `/admin/settings?tab=notifications` (matching
  Vue tree's pattern)

### Requirement: Inbounds + InboundEditorModal split into list + drawer

`Inbounds.tsx` SHALL render the list view (table) and
`InboundEditor.tsx` SHALL be implemented as an AntD `<Drawer>`
with `<Form>` and protocol-specific sub-forms (vless / vmess /
trojan / etc.). The Vue tree's `InboundEditorModal.vue` (1178
LOC) is reduced to one drawer file plus per-protocol form
components.

#### Scenario: Drawer opens with full inbound payload

- **GIVEN** the operator clicks "Edit" on an inbound row
- **WHEN** the drawer opens
- **THEN** the form SHALL be populated with the inbound's current
  values (id, tag, port, protocol, settings, stream settings,
  client list)
- **AND** every editable field present in the Vue editor SHALL
  be present here

#### Scenario: Drawer save persists via the mutation hook

- **GIVEN** the operator edits a field and clicks Save
- **WHEN** the form submits
- **THEN** it SHALL invoke the corresponding mutation hook
  (`useUpdateInbound`)
- **AND** on success the drawer SHALL close
- **AND** the parent list SHALL refetch via the P1 invalidation
  contract

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

### Requirement: P4 view specs pass with parity to Vue tree

Every view ported in P4 SHALL ship a `.spec.tsx` whose `it(...)`
count meets or exceeds the corresponding Vue `.spec.ts`.

#### Scenario: P4 test suite passes

- **WHEN** the operator runs `npm run test -- src/views/admin`
- **THEN** all specs SHALL pass
- **AND** for each Vue spec under `frontend/src/views/admin/`,
  the React equivalent SHALL contain at least as many `it(...)`
  blocks
