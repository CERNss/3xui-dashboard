## Context

`3xui-dashboard` is a greenfield central control panel for a fleet of vanilla
3x-ui (Xray) nodes. Today each 3x-ui panel manages exactly one server, exposes
far more surface than we need, and has no operator/end-user separation. We are
extracting the genuinely useful 3x-ui logic — remote-node transport, node
probing, subscription link generation — into our own Go backend and Vue SPA,
then layering net-new account, billing, and webhook capabilities on top.

Constraints:
- Remote nodes stay **vanilla 3x-ui**. We control them only through the
  documented `/panel/api/*` surface with a per-node Bearer API token.
- The dashboard runs **no Xray** itself — it is a pure controller.
- Backend Go 1.26 + Gin + GORM + PostgreSQL; frontend Vue 3 + TS + Vite +
  Tailwind + Pinia, embedded into the Go binary via `go:embed`.
- Reference upstream: `cern-3x-ui` (`web/runtime/remote.go`,
  `web/service/node.go`, `sub/`). Reference layout sibling: `cern-sub2api`.

> **Node API contract, data structures, and the full feature-extraction map
> (extract / drop / net-new) are documented in
> [`docs/3xui-node-reference.md`](../../../docs/3xui-node-reference.md).**
> That document is the authoritative reference for implementation; this design
> only summarizes the architecture-level decisions.

## Goals / Non-Goals

**Goals:**
- One backend that controls every node and serves both an admin console and a
  user portal from a single embedded binary.
- Faithfully re-implement node transport, probing, traffic snapshotting, and
  subscription generation extracted from 3x-ui.
- Fully separated admin vs. user auth (distinct JWT audiences, distinct routes).
- A client→user ownership model that 3x-ui lacks, enabling self-service.
- Pluggable, fire-and-forget webhook delivery for later alerting.

**Non-Goals:**
- Running or supervising a local Xray process.
- Telegram bot, geo-asset management, Reality cert tooling, the legacy 3x-ui UI.
- Modifying the 3x-ui nodes' code — they remain stock.
- A payment-gateway integration; billing is balance-based only in this change.
- High-availability / multi-replica dashboard (single instance assumed).

## Decisions

### 1. Project layout

Mirror `cern-sub2api`:

```
3xui-dashboard/
├── backend/   Go module: cmd/, internal/{config,handler,middleware,service,
│              repository,runtime,sub,model,job}, migrations/
├── frontend/  Vue 3 SPA: src/{views/admin,views/user,components,stores,
│              router,api,composables}
├── deploy/    docker-compose, Dockerfile, .env.example
└── openspec/
```

The Vue build output is embedded with `go:embed` and served as an SPA fallback.
Rationale: a single deployable binary, consistent with the sibling project; no
separate static host to operate.

### 2. Node API contract (extracted from `remote.go`)

All node calls go through one transport adapted from `web/runtime/remote.go`:
- Base URL = `scheme://address:port` + normalized base path.
- Auth: `Authorization: Bearer <node.api_token>`.
- Response envelope: `{ success: bool, msg: string, obj: <raw> }`.
- Endpoints used (per `web/controller/inbound.go`, `server.go`):
  - `GET  /panel/api/server/status` — probe (cpu/mem/xray/uptime).
  - `GET  /panel/api/inbounds/list` — inbounds + client stats.
  - `POST /panel/api/inbounds/add` `/update/:id` `/del/:id`.
  - `POST /panel/api/inbounds/addClient` `/updateClient/:clientId`
    `/:id/delClient/:clientId`.
  - `POST /panel/api/inbounds/:id/resetClientTraffic/:email`
    `/resetAllClientTraffics/:id` `/resetAllTraffics`.
  - `POST /panel/api/inbounds/onlines` `/lastOnline`.
- **Tag→remote-id resolution**: node-side ids are unstable, so we address
  inbounds by our stable `tag` and keep a per-node `tag→id` cache, refreshed
  from `/list` on a miss. `delClient`/`del` need the inbound id, so they
  resolve the tag first. This is lifted directly from `remote.go`.

Alternative considered: address inbounds by node-side id directly — rejected
because ids drift when inbounds are recreated on the node.

### 3. Runtime abstraction

Keep a thin `NodeRuntime` interface (subset of upstream `runtime.Runtime`):
`AddInbound/UpdateInbound/DelInbound`, `AddClient/UpdateClient/DelClient`,
`ResetClientTraffic/ResetInboundTraffics`, `FetchTrafficSnapshot`, `Probe`.
Only the **Remote** implementation exists (no Local). A `Manager` caches one
`Remote` per node id and invalidates it on node edit/delete.

Rationale: keeps the door open for a future local/agent runtime without
reworking call sites, while dropping the unused `local.go`.

### 4. Authentication — two isolated domains

- **Admin**: credentials from env (`ADMIN_USERNAME`, `ADMIN_PASSWORD`).
  Backend refuses to start if unset. Login at `POST /api/admin/auth/login`,
  constant-time compare, issues a JWT with `aud: "admin"`. No DB row.
- **User**: email/password (bcrypt) + standard OIDC. Login/register under
  `/api/user/auth/*`. JWT with `aud: "user"`.
- One signing secret, **audience-checked** per middleware. `requireAdmin`
  rejects any token whose `aud != admin`; `requireUser` the inverse. Routes are
  physically split: `/api/admin/*` vs `/api/user/*`.
- OIDC: Authorization Code + PKCE. Prefer the discovery document
  (`/.well-known/openid-configuration`) to resolve authorize/token/userinfo/JWKS;
  allow explicit endpoint overrides. ID token verified against JWKS with
  issuer/audience/expiry/clock-skew checks. `state` + `code_verifier` carried in
  short-lived signed cookies (pattern from `cern-sub2api/auth_oidc_oauth.go`).
- Identity linking: `users` has `oidc_subject` and `email`. First OIDC login
  creates a user; an OIDC email matching an existing account links instead of
  duplicating. `auth_method` is derived (`password` / `oidc` / `both`).

Alternative considered: a single shared auth domain with a role column —
rejected; the user explicitly wants admin fully outside the database and the
two surfaces hard-separated.

### 5. PostgreSQL schema

GORM models, versioned SQL migrations under `backend/migrations/` applied at
startup. Core tables:

- `users` — id, email (nullable, unique), password_hash (nullable),
  oidc_subject (nullable, unique), email_verified, status (active/suspended),
  balance numeric(18,4), timestamps.
- `nodes` — id, name, remark, scheme, address, port, base_path, api_token,
  enable, allow_private_address, status, last_heartbeat, latency_ms,
  xray_version, cpu_pct, mem_pct, uptime_secs, last_error, timestamps.
- `client_ownerships` — id, user_id, node_id, inbound_tag, client_email,
  client_uuid, sub_id, timestamps. Unique on (node_id, inbound_tag,
  client_email). The bridge 3x-ui lacks.
- `traffic_samples` — id, node_id, inbound_tag, client_email (nullable for
  inbound/node level), up, down, sampled_at. Time-bucketed source for charts.
- `plans` — id, name, price, traffic_gb, duration_days, ip_limit, enabled.
- `orders` — id, user_id, plan_id, amount, status (pending/completed/failed),
  idempotency_key (unique), provisioned_client info, timestamps.
- `balance_logs` — id, user_id, delta, reason, actor, created_at.
- `webhooks` — id, url, secret (nullable), events (text[]), enable,
  allow_private_address, timestamps.
- `webhook_deliveries` — id, webhook_id, event_id, event_type, payload,
  attempt, status, response_code, error, created_at, delivered_at.

`settings` (key/value) holds runtime-mutable toggles the admin edits in the UI
(public-registration flag, email-domain allowlist, sub remark model, traffic
thresholds). Pure deploy-time secrets (admin cred, OIDC, SMTP, JWT secret, DB
DSN) stay in `.env` and are not editable at runtime.

Counter-reset handling: `traffic_samples` stores **cumulative** node counters;
deltas are computed at query time. When a new sample is *lower* than the prior
one (node reset/restart) the delta is taken as the new absolute value, not a
negative — lifted from upstream traffic-writer logic.

### 6. Background jobs

A single in-process scheduler (`robfig/cron` or ticker goroutines) runs:
- **Probe loop** (~30 s): probe every enabled node, `UpdateHeartbeat`, append
  cpu/mem to an in-memory ring buffer per node id (from `node.go`'s
  `nodeMetrics`). Emits `node.online/offline/probe_failed` webhook events on
  status transitions.
- **Traffic loop** (~60 s): `FetchTrafficSnapshot` per node concurrently
  (bounded worker pool), persist `traffic_samples`, evaluate threshold/expiry
  rules → emit `client.*` webhook events (de-duplicated so an event fires once
  per crossing, not every tick).
- **Webhook dispatcher**: drains a delivery queue, retries with exponential
  backoff.

Fleet-wide reads (inbound list, traffic) fan out concurrently with
`golang.org/x/sync/errgroup` + a semaphore; per-node failures are collected and
returned alongside successes rather than failing the whole request.

### 7. Subscription generation — central, not proxied

Extract `sub/subService.go`, `subJsonService.go`, `subClashService.go`,
`links.go` into `backend/internal/sub`. The dashboard generates subscription
output itself from inbound configs it pulls from nodes — it does **not** proxy
the nodes' `/sub` endpoints. This is what lets one `sub_id` aggregate clients
across multiple nodes into a single subscription.

- Public route `GET /sub/:subId` (+ `/json/:subId`, `/clash/:subId`), no auth.
- Resolve `sub_id` → `client_ownerships` rows → for each, fetch the owning
  node's inbound config (short-TTL cache to avoid hammering nodes), build links,
  concatenate.
- Emit `Subscription-Userinfo` and update-interval headers from aggregated
  traffic.

Trade-off: generating links centrally means we must keep our link-builder in
sync with Xray protocol changes. Accepted — it is the only way to aggregate
multi-node subscriptions, and the builder is self-contained.

### 8. Provisioning — one shared operation

A single `ProvisionClient(userID, nodeID, inboundTag, planParams)` service is
the only path that creates/extends a node-side client. Admin "create client"
and user "purchase plan" both call it. It: creates/extends the client via the
node API, upserts `client_ownerships`, returns client + `sub_id`. Purchase
wraps it in a balance transaction with refund-on-failure.

### 9. Frontend — one SPA, two route trees

A single Vue app with two top-level route groups: `/admin/*` (admin console)
and `/portal/*` (user portal), each with its own layout, Pinia auth store, and
Axios instance (separate token storage keys). A guard redirects to the correct
login page by group. Rationale: one build, one embed, shared component library;
the separation is logical (audience + routes) not a second bundle.

### 10. Configuration

`.env` via Viper: `DATABASE_URL`, `JWT_SECRET`, `ADMIN_USERNAME`,
`ADMIN_PASSWORD`, `OIDC_*`, `SMTP_*` (default `SMTP_ENABLED=false`),
`PUBLIC_REGISTRATION` default, `EMAIL_DOMAIN_ALLOWLIST`. An `.env.example`
documents every key. SMTP absent ⇒ email verification skipped, emails stored
`unverified`, no core flow blocked.

## Risks / Trade-offs

- **3x-ui API drift** → Pin a supported 3x-ui version range; document it; add a
  per-node probe that records `xray_version`; isolate all API knowledge in the
  `runtime` package so a bump touches one place.
- **Node-side inbound recreation breaks id cache** → Always address by `tag`;
  refresh the `tag→id` cache from `/list` on miss; treat delete of a missing
  tag as idempotent success.
- **Subscription link builder diverges from Xray** → Extract upstream code
  verbatim first, add a test vector suite per protocol, version the module.
- **Traffic counter resets corrupt history** → Detect decreasing cumulative
  samples and treat as new absolute; never store negative deltas.
- **A slow/hostile node stalls fleet operations** → Per-call timeouts, bounded
  concurrency, per-node error isolation, SSRF-guarded dialer.
- **Webhook endpoint abuse / SSRF** → Guarded dialer, `allow_private_address`
  opt-in per webhook, HMAC signing, bounded retries.
- **Admin credential only in env** → Rotation requires a restart; documented as
  acceptable for a single-operator panel.
- **Single dashboard instance** → In-memory metric ring buffers and the job
  scheduler assume one process; horizontal scaling is a non-goal here.

## Migration Plan

Greenfield — no production data to migrate.
1. Scaffold backend module + frontend app per the layout above; drop the old
   SQLite proxy scaffold.
2. Stand up PostgreSQL; run startup migrations to create all tables.
3. Implement capabilities roughly in dependency order: admin-auth →
   node-management → inbound-management → client-provisioning →
   traffic-statistics → subscription → user-accounts → billing-and-plans →
   webhook-notifications.
4. Onboard nodes by registering each vanilla 3x-ui instance with its API token;
   verify probe + inbound list before relying on it.
Rollback: the dashboard is additive and read-mostly against nodes; stopping it
leaves the 3x-ui nodes fully functional on their own.

## Open Questions

- Exact supported 3x-ui version range — to be pinned against the `cern-3x-ui`
  revision in this repo before implementation.
- Retention/compaction policy for `traffic_samples` (raw rows vs. rolled-up
  buckets) — default to time-bucketed downsampling, exact windows TBD.
- Whether the user portal should expose self-service node/region selection at
  purchase time, or admin pre-assigns the node — assume admin-assigned for now.
