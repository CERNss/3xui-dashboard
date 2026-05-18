## 1. Project Scaffold & Infrastructure

- [x] 1.1 Create `backend/` Go module (Go 1.26) with `cmd/`, `internal/{config,handler,middleware,service,repository,runtime,sub,model,job}`, `migrations/`
- [x] 1.2 Add backend dependencies: Gin, GORM + Postgres driver, golang-jwt/v5, viper, robfig/cron, golang.org/x/sync, bcrypt, golang-migrate (anchored via `internal/depsanchor.go` until feature code imports each)
- [x] 1.3 Remove the old SQLite proxy scaffold (handlers/services/SQLite dependency)
- [x] 1.4 Implement Viper config loader reading `.env`: `DATABASE_URL`, `JWT_SECRET`, `ADMIN_USERNAME/PASSWORD`, `OIDC_*`, `SMTP_*`, `PUBLIC_REGISTRATION`, `EMAIL_DOMAIN_ALLOWLIST`; fail-fast on missing required keys
- [x] 1.5 Write `deploy/.env.example` documenting every config key
- [x] 1.6 Set up Gin server bootstrap, structured logging, graceful shutdown, and `go:embed` SPA serving with history-mode fallback
- [x] 1.7 Scaffold `frontend/` Vue 3 + TS + Vite + Tailwind + Pinia + vue-router app mirroring `cern-sub2api` layout (dual admin/portal route trees + two Pinia auth stores + two Axios instances per design.md)

## 2. Database & Migrations

- [x] 2.1 Define GORM models: `User`, `Node`, `ClientOwnership`, `TrafficSample`, `Plan`, `Order`, `BalanceLog`, `Webhook`, `WebhookDelivery`, `Setting`
- [x] 2.2 Write versioned SQL migrations for all tables incl. unique constraints (partial `users(LOWER(email))`, partial `users(oidc_subject)`, `users(sub_id)`, `client_ownerships(node_id,inbound_tag,client_email)`, `orders.idempotency_key`) and traffic-query indexes (node_id+taken_at, partial client_email+taken_at, partial inbound+taken_at)
- [x] 2.3 Implement startup migration runner (golang-migrate iofs over `migrations.FS`); DB connection has retry+backoff for docker-compose race; gated by DB_MIGRATE_ON_BOOT
- [x] 2.4 Implement `settings` key/value repository for runtime-mutable toggles (Get/Set/Delete/GetAll + typed bool/int/string helpers)

## 3. Node Runtime & Transport

- [x] 3.1 Port the SSRF-guarded HTTP transport (`internal/netsafe`) and Bearer-auth envelope client from upstream `web/runtime/remote.go`; allow-private context for admin-configured node destinations
- [x] 3.2 Implement `NodeRuntime` interface + `Remote` impl: base URL build, base-path normalization, envelope decode (`Envelope`, `EnvelopeError`, `DecodeObj` w/ ErrEmptyObj)
- [x] 3.3 Implement per-node `tag→remote-id` cache with refresh-on-miss from `/panel/api/inbounds/list` (concurrency-safe Replace/Set/Delete/Get/Snapshot)
- [x] 3.4 Implement `Manager` caching one `Remote` per node id with `InvalidateNode`; `ForEach` walks every enabled node and joins per-node errors
- [x] 3.5 Implement node API methods: inbound add/update/del/setEnable + form-encoded wireInbound, client add/update/delByEmail (Strategy A → re-push fallback on EnvelopeError), traffic resets (client/inbound/inbound-all/node-all), `FetchTrafficSnapshot` (list + onlines + lastOnline, best-effort), `Probe` over GET /server/status
- [x] 3.6 Port `sanitizeStreamSettingsForRemote` TLS cert path stripping — strips `certificateFile`/`keyFile` when inline `certificate`/`key` arrays are non-empty; passes malformed JSON through unchanged
- [x] 3.7 Unit-test transport: envelope decode (success + panel error + HTTP 401), tag resolution (populates from /list, refreshes on miss), idempotent DeleteInbound + DeleteClientByEmail on missing tag, basePath normalization (6 cases), sanitize (strip vs keep vs pass-through), netsafe IsPublic over 16 IPs incl. AWS metadata + CGNAT + ULA, allow-private context bypass

## 4. Admin Auth

- [x] 4.1 Implement admin credential check against env values with constant-time comparison (`auth.Service.CheckAdminCredentials`, subtle.ConstantTimeCompare on both username and password)
- [x] 4.2 Implement `POST /api/admin/auth/login` issuing a JWT with `aud: "admin"`; HS256-only signing; returns {token, expires_at, username}
- [x] 4.3 Implement `requireAdmin` middleware (rejects non-`admin` audience with 403, invalid/expired/missing with 401); verified claims attached to gin.Context under ContextKey
- [x] 4.4 Implement `requireUser` middleware and wire route groups `/api/admin/*` vs `/api/user/*`; admin /auth/login is the only public admin endpoint
- [x] 4.5 Test: user token rejected on admin routes and admin on user routes (both → 403); expired/missing/malformed token → 401; bad signature surfaced as ErrInvalidToken; non-HS256 alg rejected; missing admin env aborts startup via config_test

## 5. Node Management

- [x] 5.0 (extra) Implement `internal/service/event.Bus` — typed pub/sub with exact / wildcard-suffix / star subscribers; webhooks attach in group 12
- [x] 5.1 Implement node CRUD service: Create/Update/Delete/SetEnabled with normalize+validate (trim, lowercase scheme, basePath leading+trailing slash, port 1-65535); InvalidateNode on runtime.Manager on every mutation; MetricsStore.Drop on delete
- [x] 5.2 Implement node probe — wraps runtime.Probe, returns ProbeResult{NodeID, PriorStatus, Status, Err}; applies heartbeat patch (last_seen_at, cpu_pct, mem_pct, xray_version, uptime_s, status) on success and sets offline on failure
- [x] 5.3 Implement MetricsStore — per-node ring buffer (default cap 720 = 6h at 30s), Append/Drop/Raw window query/Bucketed (uniform time buckets with bucket-averaged CPU+Mem, sorted)
- [x] 5.4 Implement admin node endpoints under /api/admin/nodes: GET list, POST create (201), GET :id, PUT :id, DELETE :id (204), POST :id/enable, POST :id/disable, POST :id/probe (502 on transport failure), GET :id/metrics?from=&to=&bucket= (raw if bucket missing)
- [x] 5.5 Implement periodic probe job — scheduler.Add("probe", "@every 30s", probeJob.RunOnce); errgroup with concurrency cap (default 8) + per-call timeout (default 12 s); walks ListEnabled() so disabled nodes are skipped
- [x] 5.6 Emit `node.online` (offline→online), `node.offline` (online→offline), `node.probe_failed` (every failure regardless of prior state) via event.Bus with typed payloads

## 6. Inbound Management

- [x] 6.1 Implement per-node inbound List / Get via runtime.Remote.ListInbounds + GetInbound
- [x] 6.2 Implement fleet-wide concurrent aggregation (ListAll) — errgroup w/ configurable concurrency cap (default 8), per-node error collected into FleetResult.NodeErrors[nodeID]→msg, healthy rows preserved
- [x] 6.3 Implement Add / Update (with ErrTagNotFound → Add fallback) / UpdateStrict / Delete (idempotent on missing tag) / SetEnable via runtime.Remote
- [x] 6.4 Implement admin endpoints under /api/admin/inbounds: GET (fleet), GET /nodes/:nodeID, POST /nodes/:nodeID, GET/PUT/DELETE /nodes/:nodeID/:tag, POST :tag/enable, :tag/disable; 502 on upstream error, 404 on missing tag/node, 409 on disabled node
- [x] 6.5 Test partial-fleet failure surfaces healthy results + per-node errors (one panel 500s, the other lists; result has 1 inbound + NodeErrors[brokenID]); fleet happy path returns both nodes' inbounds

## 7. Client Provisioning

- [x] 7.1 Implement client create/update/delete via runtime.Remote AddClient/UpdateClient/DeleteClientByEmail; buildWireClient picks identifier by protocol (VLESS/VMess→UUID, Trojan/Shadowsocks→random hex password, unknown→UUID safe default); ExpiryTime in ms (0 = non-expiring), TotalGB in bytes
- [x] 7.2 Implement ListOnInbound — every 3x-ui client on the inbound annotated with its ClientOwnership row (or nil for unmapped); LinkToUser / UnlinkUser admin endpoints for unmapped clients
- [x] 7.3 Implement ProvisionClient(userID, nodeID, inboundTag, PlanParams{PlanID, DurationDays, TrafficLimitBytes}) — Add on first call, Update on subsequent (with re-add fallback on ErrClientNotFound); expiry computed as max(now, existing.ExpiresAt) + duration so re-provision *extends*; Upsert on client_ownerships
- [x] 7.4 Implement admin client endpoints under /api/admin/clients: GET /nodes/:nodeID/inbounds/:tag (list+annotation), POST .../provision, DELETE .../clients/:email, POST .../clients/:email/link, POST .../clients/:email/unlink; 404 on missing user/plan/node/tag/client, 409 on disabled node, 502 on upstream
- [x] 7.5 Tests: computeExpiry handles first-provision (now+30d), zero-duration (non-expiring), extend-from-future (existing+30d), expired-existing (now+30d); buildWireClient assigns UUID to VLESS, password to Trojan/SS, UUID id as unknown-protocol default; TotalGB stored as bytes; ms-since-epoch ExpiryTime

## 8. Traffic Statistics

- [x] 8.1 Implement periodic traffic-collection job (@every 60s): errgroup with concurrency cap calls FetchTrafficSnapshot per node, persists inbound-level + client-level rows in one InsertBatch per node
- [x] 8.2 Implement cumulative→delta computation (SumDeltas, BucketDeltas): cur<prev treated as counter reset → delta = cur (not cur-prev); never produces negative deltas
- [x] 8.3 Implement UsageForOwnership / UsageForUser aggregated queries: per-ownership totals (up, down, total, limit, expires_at, last_sample_at) over [from, to]
- [x] 8.4 Implement HistoryForOwnership / HistoryForInbound returning BucketPoint{bucket_start_unix, up, down} sorted ascending
- [x] 8.5 Implement traffic reset endpoints: client (POST .../inbound/:tag/client/:email), inbound (resetTraffic + resetAllClientTraffics), node (resetAllTraffics) — all 204 on success, 502 on upstream
- [x] 8.6 Implement threshold/expiry evaluation in collectOne: client.over_limit (up+down ≥ Total>0), client.expired (ExpiryTime in past); in-memory dedup keyed by event|node|inbound|email with 6h re-emit window
- [x] 8.7 Implement user-facing own-traffic endpoint at /api/user/traffic — user_id taken from JWT subject, not the URL, so users cannot query others; default window = last 7 days
- [x] 8.8 Tests (4 cases): monotonic series sums correctly; counter reset produces no negative delta and the post-reset value becomes the new baseline; empty/single-sample inputs produce zero; BucketDeltas groups + sums correctly across two buckets

## 9. Subscription

- [ ] 9.1 Port `sub/` link builders (`subService`, `subJsonService`, `subClashService`, `links`) into `internal/sub`
- [ ] 9.2 Implement central subscription assembly: resolve `sub_id` → ownerships → fetch inbound configs (short-TTL cache) → build links
- [ ] 9.3 Implement public routes `GET /sub/:subId`, `/sub/json/:subId`, `/sub/clash/:subId` (no auth)
- [ ] 9.4 Implement base64 / JSON / Clash output formats
- [ ] 9.5 Add `Subscription-Userinfo` + update-interval headers from aggregated traffic
- [ ] 9.6 Implement user-portal subscription endpoint returning URL + QR data
- [ ] 9.7 Add per-protocol link test vectors; test unknown `sub_id` → 404 and multi-node aggregation

## 10. User Accounts

- [ ] 10.1 Implement email/password registration (bcrypt) gated by public-registration switch + domain allowlist
- [ ] 10.2 Implement email/password login at `/api/user/auth/login` issuing `aud: "user"` JWT
- [ ] 10.3 Implement OIDC discovery resolution (`.well-known/openid-configuration`) with explicit-endpoint overrides
- [ ] 10.4 Implement OIDC start (`state` + PKCE verifier in signed cookies) and callback (code exchange, JWKS ID-token verification, issuer/aud/exp/clock-skew checks)
- [ ] 10.5 Implement OIDC account provisioning + linking by `oidc_subject` / verified email
- [ ] 10.6 Implement email-domain allowlist enforcement across register / bind / OIDC email
- [ ] 10.7 Implement email-address binding flow (verified when SMTP on, `unverified` when off)
- [ ] 10.8 Implement optional SMTP integration (config slot, default disabled, graceful degrade, non-fatal send failures)
- [ ] 10.9 Implement password change (incl. set-initial-password for OIDC-only accounts)
- [ ] 10.10 Implement admin user-account administration: list / edit / suspend / delete; suspended tokens rejected
- [ ] 10.11 Emit `user.registered` event
- [ ] 10.12 Test: registration disabled blocks signup but not login; disallowed domain rejected; OIDC state mismatch rejected

## 11. Billing & Plans

- [ ] 11.1 Implement plan CRUD (admin) with enable flag
- [ ] 11.2 Implement user balance + `balance_logs`; admin balance adjustment endpoint
- [ ] 11.3 Implement plan purchase: idempotency-key dedupe, balance transaction, call `ProvisionClient`, refund-on-failure
- [ ] 11.4 Implement order history endpoints (user own / admin all with filters)
- [ ] 11.5 Emit `order.created/completed/failed` events
- [ ] 11.6 Test: insufficient balance rejected; provisioning failure refunds; duplicate idempotency key returns original order

## 12. Webhook Notifications

- [ ] 12.1 Implement webhook CRUD (admin) with URL validation, signing secret, event subscription, enable flag, allow-private flag
- [ ] 12.2 Implement the event catalog + per-webhook subscription matching (incl. wildcard)
- [ ] 12.3 Implement versioned JSON payload envelope with self-describing `data`
- [ ] 12.4 Implement HMAC signing + timestamp headers
- [ ] 12.5 Implement async delivery queue + dispatcher with exponential-backoff retry, timeouts, SSRF-guarded transport, per-webhook isolation
- [ ] 12.6 Implement `webhook_deliveries` log, delivery-history endpoint, test-event send, failed-delivery replay
- [ ] 12.7 Test: only subscribed events delivered; failing webhook does not stall others

## 13. Frontend — Shared Infrastructure

- [ ] 13.1 Set up router with `/admin/*` and `/portal/*` route trees, layouts, and group-aware auth guards
- [ ] 13.2 Set up two Pinia auth stores + two Axios instances with separate token storage and 401 handling
- [ ] 13.3 Build shared component library (tables, modals, forms, charts via chart.js, toasts) and Tailwind theme
- [ ] 13.4 Set up i18n scaffolding and API type definitions

## 14. Frontend — Admin Console

- [ ] 14.1 Admin login page
- [ ] 14.2 Nodes page: list with live status, create/edit, enable/disable, probe, cpu/mem history charts
- [ ] 14.3 Inbounds page: per-node + fleet view, create/edit/delete
- [ ] 14.4 Clients page: list/search, create/edit/delete, link to user
- [ ] 14.5 Traffic dashboard: node/inbound/client usage + history charts + resets
- [ ] 14.6 Users page: list/edit/suspend/delete, balance adjustment
- [ ] 14.7 Plans & orders pages
- [ ] 14.8 Webhooks page: config, event subscription, delivery log, test/replay
- [ ] 14.9 Settings page: public-registration toggle, email-domain allowlist, sub remark model, traffic thresholds

## 15. Frontend — User Portal

- [ ] 15.1 User login / register pages (register hidden when public registration off) + OIDC login button
- [ ] 15.2 OIDC callback handling page
- [ ] 15.3 Dashboard: own traffic usage, percentage, days remaining
- [ ] 15.4 Subscription page: copyable URL + QR code, empty state
- [ ] 15.5 Plans page + purchase flow + order history
- [ ] 15.6 Profile page: email binding, password change/set, account info

## 16. Packaging & Deploy

- [ ] 16.1 Frontend build wired into the Go binary via `go:embed`
- [ ] 16.2 Multi-stage `Dockerfile` (frontend build → Go build → slim runtime)
- [ ] 16.3 `docker-compose.yml` with PostgreSQL + the dashboard service
- [ ] 16.4 Root `Makefile` / `Makefile` targets: dev, build, lint, test, migrate
- [ ] 16.5 README: 3x-ui node onboarding (API token issuance), supported 3x-ui version range, config reference
