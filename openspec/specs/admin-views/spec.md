# admin-views

The four admin-side route views — Status, Nodes, Inbounds, Settings —
plus the full 5-tab inbound editor modal. Lives in
`frontend/src/views/admin/`.

## Purpose & boundaries

These are the visual surfaces an operator actually clicks through.
The chrome they live inside is `layouts-and-chrome::AdminLayout`; the
tokens they consume are `design-system`; the data they fetch is
defined by the various backend modules (`node-management`,
`inbound-management`, `client-provisioning`, `traffic-statistics`,
`settings`).

## Views inventory

| Path | View file | Purpose |
|---|---|---|
| `/admin/status` | `Status.vue` | Fleet KPI strip + node health table. Landing page. |
| `/admin/nodes` | `Nodes.vue` | Node CRUD: add / edit / disable / delete / probe. |
| `/admin/inbounds` | `Inbounds.vue` | Cross-node inbound list, Sub2API-density table, traffic progress bars, per-row labeled actions. |
| `/admin/inbounds` (modal) | `InboundEditorModal.vue` | Full 3x-ui-equivalent 5-tab inbound editor (基础配置 / 协议 / Stream / Sniffing / 高级配置), supports 8 transmissions × 3 securities + raw JSON escape hatches. |
| `/admin/settings` | `Settings.vue` | The known-keys editor for the `settings` module's six toggles. |

## Requirements

### Requirement: Status View Renders Fleet KPI Strip + Node Health

The system SHALL provide `/admin/status` as the admin entry landing
page, showing a 4-card KPI strip + a node health table.

#### Scenario: KPI strip composition

- **WHEN** Status.vue renders
- **THEN** the page SHALL show four KPI cards in a responsive grid (`grid-cols-1 md:grid-cols-2 lg:grid-cols-4`):
  - **节点**: count + breakdown pills (`X 在线 · Y 离线 · Z 未知`)
  - **入站**: count + "across X 节点" subtext
  - **客户端**: count + "已 provisioned" subtext
  - **总流量**: formatted bytes + `↑` upload + `↓` download split
- **AND** every card SHALL follow the Xboard pattern: tiny label top-left, accent icon tile top-right, big number, delta subtitle
- **AND** every card SHALL use the same `bg-accent-50 / text-accent-600` icon tile (single accent, semantics via the icon glyph)

#### Scenario: Node health table

- **WHEN** Status.vue renders and at least one node exists
- **THEN** the page SHALL show a hairline-bordered card with header "节点健康" + "管理 →" link to `/admin/nodes`
- **AND** the table columns SHALL be: 名称, 状态, CPU / Mem, Xray, Last Seen
- **AND** status SHALL be a rounded-full pill with green/red/gray accents and `{{ nodeStatusLabel(status) }}` (中文「在线/离线/未知」)
- **AND** when no nodes exist, the table SHALL render `EmptyState` with title "还没有节点" + CTA "去添加节点"

### Requirement: Nodes View Provides CRUD + Probe

The system SHALL provide `/admin/nodes` for the node lifecycle UI.

#### Scenario: Node list table

- **WHEN** Nodes.vue renders
- **THEN** the table columns SHALL be: ID, 名称, 连接, 状态, CPU / Mem, Xray, Last Seen, 操作
- **AND** the 操作 column SHALL show three icon buttons per row: 探测 (magnifying-glass), 启用/禁用 (toggle), 删除 (trash)
- **AND** disabled nodes SHALL render with `opacity-60`

#### Scenario: Add node modal

- **WHEN** the user clicks "添加节点"
- **THEN** a modal SHALL open with fields: 名称, Scheme (https/http), Host, Port, Base path, API Token, 启用 checkbox
- **AND** the primary CTA SHALL read "创建" (verb-first), with "取消" as the secondary
- **AND** the modal SHALL close on outdoor click + Escape

#### Scenario: Delete confirmation

- **WHEN** the user clicks the delete button on a row
- **THEN** the system SHALL show a native `confirm()` dialog reading "确认删除节点 \"X\"？\n所有附属的 client_ownerships 会被级联删除。"
- **AND** the delete SHALL fire only after the user confirms

### Requirement: Inbounds View Renders Sub2API-Density Table

The system SHALL provide `/admin/inbounds` for the fleet-wide inbound
list, with a high-density multi-line table.

#### Scenario: KPI strip above the table

- **WHEN** Inbounds.vue renders
- **THEN** the page SHALL show a 6-card KPI strip: 上传, 下载, 总用量, 累计, 入站, 客户端
- **AND** the layout SHALL be `grid-cols-2 md:grid-cols-3 lg:grid-cols-6`

#### Scenario: Toolbar above table

- **WHEN** Inbounds.vue renders
- **THEN** the toolbar SHALL contain:
  - A 320px search input with prefix icon, placeholder "搜索 备注 / 协议 / 端口 / 节点 / tag"
  - A protocol filter chip row with options: 全部, vless, vmess, trojan, shadowsocks
- **AND** the active chip SHALL render `bg-ink-900 text-white` (light) / `bg-accent-600` (dark)

#### Scenario: Table row density (Sub2API pattern)

- **WHEN** the table renders a row
- **THEN** each row SHALL be `py-4` minimum row height
- **AND** cells SHALL be MULTI-LINE where useful:
  - **节点**: chip with status dot + "node #ID" subtext below
  - **备注 · Tag · 端口**: 备注 (main, ink) + tag (mono, dim) + `:port` (mono, dim) on three lines
  - **协议**: protocol pill on top, transport + security pills below (vertical stack)
  - **客户端**: total count big + "● 在线 X · ○ 离线 Y" breakdown line
  - **流量 · 用量**: bytes + `/ limit` + Marzban-style gradient progress bar (green <60%, amber <85%, red ≥85%) + per-direction `↑ up / ↓ down` line
- **AND** the 操作 column SHALL show three ALWAYS-VISIBLE labeled mini-buttons: 编辑 / 重置 / 删除

#### Scenario: Row expansion

- **WHEN** the user clicks a row
- **THEN** an expanded panel SHALL appear below with the inbound's client list + an "添加客户端" CTA
- **AND** the chevron in the first column SHALL rotate 90° to indicate state

#### Scenario: Add inbound CTA opens 5-tab modal

- **WHEN** the user clicks "添加入站"
- **THEN** `InboundEditorModal.vue` SHALL open in `create` mode
- **AND** the modal SHALL provide 5 tabs: 基础配置 / 协议 / Stream / Sniffing / 高级配置 (see next requirement)

### Requirement: Inbound Editor Modal Covers 3x-UI Parity

The system SHALL provide a 5-tab inbound editor that matches the
visual structure of the upstream 3x-ui panel's add-inbound modal,
supporting all 8 transmissions × 3 securities.

#### Scenario: Tab "基础配置"

- **WHEN** the user opens the modal
- **THEN** the first tab SHALL provide fields for: enable toggle, remark, protocol (vless/vmess/trojan/shadowsocks), listen address, port, total GB limit, trafficReset (daily/none), expiry datetime

#### Scenario: Tab "协议"

- **WHEN** the user is on the 协议 tab
- **THEN** the form SHALL show protocol-specific fields:
  - VLESS: clients (id + flow), decryption=none
  - VMess: clients (id + alterId)
  - Trojan: clients (password)
  - Shadowsocks: method (chacha20-poly1305 / aes-256-gcm / 2022-blake3-*) + clients (password)

#### Scenario: Tab "Stream"

- **WHEN** the user is on the Stream tab
- **THEN** the form SHALL accept network ∈ {tcp, ws, grpc, httpupgrade, h2, xhttp, kcp, quic}
- **AND** the form SHALL conditionally show transport-specific fields for the chosen network (e.g. ws path + host, grpc serviceName)
- **AND** the form SHALL accept security ∈ {none, tls, reality} with conditional sub-fields:
  - tls: server name, alpn, cert mode
  - reality: server name, short ids, private/public key pair, dest, spider X

#### Scenario: Tab "Sniffing"

- **WHEN** the user is on the Sniffing tab
- **THEN** the form SHALL accept: enabled toggle, dest override (http/tls/quic/fakedns), metadata only, route only, domains override

#### Scenario: Tab "高级配置"

- **WHEN** the user is on the 高级配置 tab
- **THEN** the form SHALL show three textareas labeled "settings", "streamSettings", "sniffing" — each with a "override raw" checkbox
- **AND** when "override raw" is checked, the corresponding textarea content SHALL replace the per-tab structured form output verbatim — escape hatch for fields not yet in the structured form

#### Scenario: composeBody emits proper wire format

- **WHEN** the user submits the modal
- **THEN** `composeBody()` SHALL produce a request body matching the wire format expected by `runtime-3xui-client::AddInbound` — `settings`, `streamSettings`, `sniffing` as stringified JSON (NOT JSON objects)
- **AND** the API call SHALL be `POST /api/admin/inbounds/nodes/:nodeID` in create mode or `PUT .../:tag` in edit mode

### Requirement: Settings View Lists Known Keys

The system SHALL provide `/admin/settings` for the six known settings
keys defined by the `settings` module.

#### Scenario: Settings list

- **WHEN** Settings.vue renders
- **THEN** the page SHALL fetch `GET /api/admin/settings` and render one row per known key
- **AND** each row SHALL show: label, type, type-appropriate input (toggle for bool / number input for int / text for string), Save button per-row
- **AND** an indicator SHALL distinguish DB-override rows from env-default rows ("已覆盖" badge or similar)

#### Scenario: Save flash

- **WHEN** the user clicks Save on a row
- **THEN** the system SHALL call `PUT /api/admin/settings/<key>` with the new value
- **AND** show a brief "已保存" flash on success
- **AND** surface backend errors verbatim via `formatError` on failure

#### Scenario: Reset-to-default

- **WHEN** a row has a DB override and the user clicks "恢复默认"
- **THEN** the system SHALL call `DELETE /api/admin/settings/<key>`
- **AND** re-fetch the list to show the env-default value

## Out of scope

- Mobile-responsive table collapse (admin views are desktop-first).
- Bulk operations (multi-select + bulk delete/reset) — single-row only
  for now.
- Real-time push updates (auto-refresh is manual via the "刷新" button
  on each page).
- Charts on the Status page (KPI strip is the only data visualization
  in v1; per-node CPU/mem time series are exposed by the backend but
  not yet rendered).
