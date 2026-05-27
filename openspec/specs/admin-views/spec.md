# admin-views

Admin route views under `frontend/src/views/admin/`. These are the operator
surfaces mounted by `layouts-and-chrome::AdminLayout` and guarded by admin
authentication.

## Purpose & Boundaries

This spec covers the visual and interaction contract for `/admin/*` pages. The
backend behavior they call is owned by module specs such as `node-management`,
`inbound-management`, `client-provisioning`, `traffic-statistics`,
`billing-and-plans`, `webhook-notifications`, and `settings`.

## View Inventory

| Path | View file | Purpose |
|---|---|---|
| `/admin/status` | `Overview.tsx` | Overview shell with Status and Stats tabs. |
| `/admin/stats` | redirect | Redirects to `/admin/status?tab=stats`. |
| `/admin/ops-monitor` | `OpsMonitor.tsx` | Operational health and traffic trend analysis. |
| `/admin/nodes` | `Nodes.tsx` + `nodes/NodeDrawer.tsx` | Node CRUD, enable/disable, delete, probe. |
| `/admin/inbounds` | `Inbounds.tsx` + `InboundEditor.tsx` | Fleet inbound list and inbound editor drawer. |
| `/admin/users` | `Users.tsx` | Portal user search, filters, moderation, and account updates. |
| `/admin/plans` | `Plans.tsx` | Plan CRUD and publication controls. |
| `/admin/provisioning-pools` | `ProvisioningPools.tsx` | Plan-to-node/inbound provisioning pool management. |
| `/admin/orders` | `Orders.tsx` | Order history, payment status, and operator inspection. |
| `/admin/audit-log` | `AuditLog.tsx` | Admin action audit trail. |
| `/admin/webhooks` | `Webhooks.tsx` | Customer webhook subscription CRUD and delivery status. |
| `/admin/settings` | `Settings.tsx` + `settings/*` | Server-driven settings grouped by workflow. |

## Requirements

### Requirement: Overview Provides Status And Stats Tabs

The system SHALL provide `/admin/status` as the admin landing page, with a
tabbed Overview view.

#### Scenario: Status tab renders fleet health

- **WHEN** `Overview.tsx` renders with the `status` tab active
- **THEN** it SHALL mount `StatusPanel`
- **AND** show fleet KPIs and node health content sourced from admin query hooks.

#### Scenario: Stats tab renders traffic statistics

- **WHEN** the URL is `/admin/status?tab=stats`
- **THEN** the active tab SHALL be `stats`
- **AND** `StatsPanel` SHALL render traffic-oriented KPIs and charts.

#### Scenario: Refresh targets the active tab

- **WHEN** the operator clicks the page refresh action
- **THEN** the active panel's `reload()` handle SHALL be invoked
- **AND** the inactive tab SHALL NOT be forced to remount.

### Requirement: Ops Monitor Shows Operational Trends

The system SHALL provide `/admin/ops-monitor` for higher-density operational
observability.

#### Scenario: Ops monitor loads

- **WHEN** the operator opens `/admin/ops-monitor`
- **THEN** the page SHALL render KPI cards and trend panels for node health and traffic data
- **AND** failures SHALL be surfaced inline through the page error state rather than silently ignored.

### Requirement: Nodes View Provides Node Lifecycle Operations

The system SHALL provide `/admin/nodes` for registering and maintaining remote
3x-ui nodes.

#### Scenario: Node table/list

- **WHEN** `Nodes.tsx` renders
- **THEN** it SHALL show each node's identity, connection target, status, resource health, last-seen data, and row actions
- **AND** the list SHALL provide search/filter behavior appropriate for repeated operator use.

#### Scenario: Node drawer

- **WHEN** the operator creates or edits a node
- **THEN** `NodeDrawer.tsx` SHALL collect name, scheme, host, port, base path, API token, and enabled state
- **AND** save through the admin node mutation hook.

#### Scenario: Destructive action confirms intent

- **WHEN** the operator deletes a node
- **THEN** the UI SHALL ask for confirmation before invoking the delete mutation.

### Requirement: Inbounds View Provides Fleet Inbound Management

The system SHALL provide `/admin/inbounds` for fleet-wide inbound inspection and
editing.

#### Scenario: Inbound list

- **WHEN** `Inbounds.tsx` renders
- **THEN** it SHALL show node, remark/tag/port, protocol, client counts, usage, and row actions
- **AND** it SHALL provide search/filter controls for common operator lookup workflows.

#### Scenario: Inbound editor drawer

- **WHEN** the operator creates or edits an inbound
- **THEN** `InboundEditor.tsx` SHALL open as an AntD drawer containing an AntD form
- **AND** it SHALL include Basic, Protocol, Stream, Sniffing, and Advanced JSON sections where applicable.

#### Scenario: Protocol coverage

- **WHEN** the editor protocol is changed
- **THEN** it SHALL render protocol-specific fields for `vless`, `vmess`, `trojan`, `shadowsocks`, `wireguard`, and `hysteria`
- **AND** WireGuard and Hysteria SHALL suppress Stream/Sniffing sections when those settings do not apply.

#### Scenario: Stream coverage

- **WHEN** a stream-enabled protocol is edited
- **THEN** the editor SHALL support transmissions `tcp`, `ws`, `grpc`, `httpupgrade`, `h2`, `xhttp`, `kcp`, and `quic`
- **AND** security modes `none`, `tls`, and `reality` with conditional fields.

#### Scenario: Wire format composition

- **WHEN** the operator submits the editor
- **THEN** `inbound-editor/model.ts` SHALL compose `settings`, `streamSettings`, and `sniffing` as JSON strings matching the backend/runtime 3x-ui wire contract.

### Requirement: User And Billing Views Cover Portal Operations

The system SHALL provide admin pages for user, plan, provisioning-pool, and
order operations.

#### Scenario: Users view

- **WHEN** `/admin/users` renders
- **THEN** operators SHALL be able to search/filter users, inspect balance and OIDC linkage, and perform moderation actions through admin APIs.

#### Scenario: Plans view

- **WHEN** `/admin/plans` renders
- **THEN** operators SHALL be able to create, edit, enable/disable, and inspect plans that portal users can purchase.

#### Scenario: Provisioning pools view

- **WHEN** `/admin/provisioning-pools` renders
- **THEN** operators SHALL be able to bind plans to node/inbound targets used by client provisioning.

#### Scenario: Orders view

- **WHEN** `/admin/orders` renders
- **THEN** operators SHALL be able to inspect orders, payment method/status, and provisioning outcome.

### Requirement: Audit, Webhook, And Settings Views Are First-Class Admin Pages

The system SHALL provide operational pages for audit, webhook, and settings
workflows.

#### Scenario: Audit log view

- **WHEN** `/admin/audit-log` renders
- **THEN** it SHALL display admin action history with filtering/search affordances.

#### Scenario: Webhooks view

- **WHEN** `/admin/webhooks` renders
- **THEN** operators SHALL be able to register, edit, enable/disable, delete, and inspect webhook subscriptions.

#### Scenario: Settings view

- **WHEN** `/admin/settings` renders
- **THEN** `Settings.tsx` SHALL present server-provided descriptors in workflow tabs
- **AND** each setting row SHALL support save and reset-to-default where applicable.

## Out of Scope

- Backend endpoint semantics, which are owned by the corresponding backend module specs.
- Real-time push updates; admin pages may use manual refresh and query invalidation.
