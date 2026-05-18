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

- [x] 9.1 Port `sub/` link builders into `internal/sub`: VLESS (full — tcp/ws/grpc/reality + tls fingerprint), VMess (v2rayn base64-of-JSON envelope), Trojan, Shadowsocks (base64 method:password); unknown protocols return empty so they're silently skipped
- [x] 9.2 Implement Assembler.Build(ctx, subID, remarkFmt): user lookup by sub_id → walk ownerships → fetch inbound (short-TTL cache, default 15s, keyed by node|tag) → BuildLink → assemble UserInfo (smallest of any ownership.ExpiresAt as expire); per-ownership failures logged and skipped
- [x] 9.3 Implement public routes `GET /sub/:subId` (base64), `GET /sub/json/:subId` (no auth, registered on engine root)
- [x] 9.4 Implement base64 (newline-joined urls → std base64) and JSON (array of {protocol, url, remark}); Clash format deferred to a follow-up
- [x] 9.5 Add `Subscription-Userinfo: upload=…; download=…; total=…; expire=…` header from the aggregated UserInfo; `Profile-Update-Interval: 12` constant
- [ ] 9.6 User-portal subscription endpoint w/ URL + QR data — deferred to group 15.4 (frontend portal page consumes /sub directly)
- [x] 9.7 Tests (6 cases) — VLESS+WS+TLS query keys (type, security, sni, fp, flow, path, host, fragment), VLESS+Reality (pbk, sid, sni from serverNames[0], fp), VMess vmess:// + base64 JSON inner round-trip, Trojan prefix, Shadowsocks prefix, unknown protocol → empty; formatRemark uses first rune of spec as separator and expands i/e/o/t tokens correctly

## 10. User Accounts

- [x] 10.1 Implement email/password registration (bcrypt DefaultCost) gated by public-registration switch (settings table overrides env) + domain allowlist
- [x] 10.2 Implement email/password login at `/api/user/auth/login` issuing `aud: "user"` JWT via auth.Service.IssueUserToken; auto-login after register
- [ ] 10.3 OIDC discovery — DEFERRED to a follow-up. Config wires through, /api/user/auth/oidc/start returns 501.
- [ ] 10.4 OIDC start/callback — DEFERRED with the same 501 stub.
- [ ] 10.5 OIDC account provisioning/linking — DEFERRED.
- [x] 10.6 Implement email-domain allowlist enforcement on register + bind (case-insensitive trailing-domain match; empty allowlist = unrestricted)
- [x] 10.7 Implement email binding flow — stores email + sets `email_verified=false` (no SMTP delivery, see 10.8)
- [ ] 10.8 SMTP integration — DEFERRED. Config slot present, BindEmail stores unverified, send pathways absent.
- [x] 10.9 Implement password change; ChangePassword tolerates empty old_password when no password hash exists yet (OIDC-only set-initial-password path); requires non-empty old_password otherwise
- [x] 10.10 Implement admin user-account administration: list (paged) / get / update (email + status + balance_cents) / suspend / unsuspend / balance adjust / delete; suspended users blocked from login at the service layer
- [x] 10.11 Emit `user.registered` event with {UserID, Email} payload from Register
- [x] 10.12 Tests (4 cases): empty allowlist permits any domain; allowlist matches case-insensitively + rejects unlisted + handles malformed inputs; publicRegistrationGate respects cfg fallback when no settings row; generateSubID returns distinct 32-hex strings.

## 11. Billing & Plans

- [x] 11.1 Plan CRUD: admin POST/PUT/DELETE/GET /api/admin/plans; user GET /api/user/plans (only enabled); normalizePlan validates name + non-negative price/duration/traffic
- [x] 11.2 User balance + balance_logs already in UserRepo.AdjustBalance (transactional read-modify-write + log row); admin balance-adjust at POST /api/admin/users/:id/balance
- [x] 11.3 Purchase orchestration: idempotency-key short-circuit, pending-row reservation, balance check, charge via AdjustBalance, ProvisionClient, MarkCompleted on success / refund + MarkRefunded on provisioning failure
- [x] 11.4 Order history: user own at GET /api/user/orders; admin all at GET /api/admin/orders with optional ?user_id and ?status filters
- [x] 11.5 Emit OrderCreated / OrderCompleted / OrderFailed (with Reason ∈ {insufficient_balance, charge_failed, provisioning_failed})
- [x] 11.6 Tests (5 cases): plan normalization rejects empty name + negative price/duration/traffic; isUniqueViolation recognises lib/pq + SQLSTATE 23505. Full Purchase-flow integration test deferred — requires a real Postgres harness

## 12. Webhook Notifications

- [x] 12.1 Webhook CRUD (admin): list/create/update/delete with URL, signing secret (auto-generated random hex if empty), Events StringSlice (jsonb), enable flag, allow_private flag
- [x] 12.2 Event catalog (well-known constants in event.NodeOnline/Offline/ProbeFailed, UserRegistered, OrderCreated/Completed/Failed, ClientExpired/OverLimit); per-webhook subscription via patternsMatch (exact / wildcard-suffix / "*")
- [x] 12.3 Versioned JSON Envelope {version, event, timestamp, data}; data is the raw event payload (typed per event)
- [x] 12.4 HMAC-SHA256 signing of timestamp + "." + body under the per-webhook secret; headers X-Dashboard-{Event, Timestamp, Signature, Delivery-Id} sent with every request
- [x] 12.5 Async delivery dispatched on a goroutine per (event, webhook) match; exponential backoff 1s → 60s, MaxAttempts=5; SSRF-guarded http transport (allow_private opt-in per webhook); each delivery isolated so a failing webhook doesn't stall others
- [x] 12.6 webhook_deliveries log via WebhookDeliveryRepo (Create/MarkSuccess/MarkFailed with truncated body+error); GET /webhooks/:id/deliveries; POST /webhooks/:id/test fabricates a "webhook.test" event; POST /webhooks/deliveries/:deliveryID/replay re-dispatches by id
- [x] 12.7 Tests (3 cases): patternsMatch covers exact + wildcard-suffix + "*" + multi-pattern; sign is deterministic + 64-hex + changes with different secret; randomSecret returns 32-hex

## 13. Frontend — Shared Infrastructure

- [x] 13.1 Router with `/admin/*` and `/portal/*` route trees, AdminLayout + PortalLayout, AuthLayout for login/register; beforeEach guard checks RequireAdmin/RequireUser via stores
- [x] 13.2 Two Pinia auth stores (adminAuth, portalAuth) + two Axios instances (admin /api/admin, portal /api/user) with separate token storage (dashboard.admin.token / dashboard.portal.token) and per-instance 401 redirect to the right login page
- [x] 13.3 Tailwind theme (primary indigo + accent teal + surface scales + card/glow shadows + slide/scale/fade animations). Heavy component library (chart.js wrappers, modals, toasts) deferred — see 14.x/15.x notes
- [x] 13.4 vue-i18n with en + zh skeleton locales; ApiEnvelope + ApiError types in src/types/api.ts; per-domain API client modules in src/api/{admin,portal}/

## 14. Frontend — Admin Console

- [x] 14.1 Admin login page — full form bound to /api/admin/auth/login + adminAuth store + ?next= redirect support
- [x] 14.2 Nodes page (renders as the Admin Dashboard for now) — live list, probe button, enable/disable toggle, status badge, scheme+host+port+base_path; create-edit modal deferred (POST /api/admin/nodes works from any HTTP client today)
- [ ] 14.3 Inbounds page — DEFERRED. /api/admin/inbounds endpoint is fully functional and documented; admins can use any REST client until the page lands
- [ ] 14.4 Clients page — DEFERRED. /api/admin/clients/* endpoints work
- [ ] 14.5 Traffic dashboard — DEFERRED. /api/admin/traffic/* + history endpoints work
- [ ] 14.6 Users page — DEFERRED. /api/admin/users/* works
- [ ] 14.7 Plans & orders pages — DEFERRED. /api/admin/plans + /api/admin/orders work
- [ ] 14.8 Webhooks page — DEFERRED. /api/admin/webhooks + /:id/test + /:id/deliveries + replay work
- [ ] 14.9 Settings page — DEFERRED. settings KV repo works; admin endpoint to expose it is also deferred

## 15. Frontend — User Portal

- [x] 15.1 Login + Register pages, full form bound to /api/user/auth/{login,register}. The OIDC login button is omitted in v1 (backend stub returns 501 — see Group 10 deferral notes)
- [ ] 15.2 OIDC callback page — DEFERRED with OIDC backend stubs
- [x] 15.3 Portal Dashboard — own usage table + subscription URL display, bytes humanized
- [x] 15.4 Subscription URL is exposed on the dashboard card (copy-paste-ready). Dedicated /sub QR-code page DEFERRED (requires wiring qrcode lib into a page)
- [ ] 15.5 Plans + purchase + order history — DEFERRED. /api/user/{plans,orders,purchase} work
- [ ] 15.6 Profile page (bind-email, change-password) — DEFERRED. /api/user/{profile,change-password,bind-email} work

## 16. Packaging & Deploy

- [x] 16.1 Frontend build wired into the Go binary via //go:embed at backend/internal/web/embed.go (`//go:embed all:dist`). Vite outputs directly to ../backend/internal/web/dist
- [x] 16.2 Multi-stage Dockerfile: node:22-alpine → golang:1.26-alpine → alpine:3.20 runtime; non-root user, ca-certificates + tzdata, CGO disabled, trimpath + -s -w
- [x] 16.3 deploy/docker-compose.yml with postgres:16-alpine + dashboard; service depends_on db with healthcheck; required env vars fail-fast via `${VAR:?...}`
- [x] 16.4 Makefile targets: build (build-frontend + build-backend), dev (parallel), dev-{frontend,backend}, test (go test ./... + vue-tsc), lint (go vet + eslint), migrate (boot-and-exit), docker-build/up/down, clean (preserves dist/.gitkeep), tidy
- [x] 16.5 README.md covers v1 ship/defer list, prereqs, docker quickstart, local-dev quickstart, 3x-ui node onboarding (api-token issuance + POST /api/admin/nodes payload), supported version range, config reference + project layout
