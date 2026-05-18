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

- [ ] 3.1 Port the SSRF-guarded HTTP transport and Bearer-auth envelope client from upstream `web/runtime/remote.go`
- [ ] 3.2 Implement `NodeRuntime` interface + `Remote` impl: base URL build, base-path normalization, envelope decode
- [ ] 3.3 Implement per-node `tag→remote-id` cache with refresh-on-miss from `/panel/api/inbounds/list`
- [ ] 3.4 Implement `Manager` caching one `Remote` per node id with `InvalidateNode`
- [ ] 3.5 Implement node API methods: inbound add/update/del, client add/update/del, traffic resets, `FetchTrafficSnapshot`, `Probe`
- [ ] 3.6 Port `sanitizeStreamSettingsForRemote` TLS cert path stripping
- [ ] 3.7 Unit-test transport: envelope decode, tag resolution, idempotent delete of missing tag

## 4. Admin Auth

- [ ] 4.1 Implement admin credential check against env values with constant-time comparison
- [ ] 4.2 Implement `POST /api/admin/auth/login` issuing a JWT with `aud: "admin"`
- [ ] 4.3 Implement `requireAdmin` middleware (rejects non-`admin` audience with 403, invalid/expired with 401)
- [ ] 4.4 Implement `requireUser` middleware and wire route groups `/api/admin/*` vs `/api/user/*`
- [ ] 4.5 Test: user token rejected on admin routes and vice versa; missing admin env aborts startup

## 5. Node Management

- [ ] 5.1 Implement node CRUD service: create/update/delete/enable with normalization + validation; invalidate runtime cache on change
- [ ] 5.2 Implement node probe (`GET /panel/api/server/status`) parsing cpu/mem/xray/uptime into a heartbeat patch
- [ ] 5.3 Implement in-memory per-node cpu/mem metric ring buffer + bucketed aggregation query
- [ ] 5.4 Implement admin node endpoints: list, create, update, delete, enable/disable, on-demand probe, metric history
- [ ] 5.5 Implement the periodic probe job (~30 s), updating heartbeat status and appending metrics
- [ ] 5.6 Emit `node.online/offline/probe_failed` events on status transitions

## 6. Inbound Management

- [ ] 6.1 Implement per-node inbound list/get via `/panel/api/inbounds/list`
- [ ] 6.2 Implement fleet-wide concurrent inbound aggregation with per-node error collection (errgroup + semaphore)
- [ ] 6.3 Implement inbound create/update/delete with tag resolution and update→create fallback
- [ ] 6.4 Implement admin inbound endpoints under `/api/admin/inbounds`
- [ ] 6.5 Test: partial fleet failure surfaces healthy results + per-node errors

## 7. Client Provisioning

- [ ] 7.1 Implement client create/update/delete on a node inbound (UUID/password + sub_id generation by protocol)
- [ ] 7.2 Implement client listing per inbound and fleet-wide email search, annotated with owning user
- [ ] 7.3 Implement the shared `ProvisionClient(userID,nodeID,inboundTag,planParams)` service that creates/extends a client and upserts `client_ownerships`
- [ ] 7.4 Implement admin client endpoints + client↔user link/unlink endpoints
- [ ] 7.5 Test: provision creates+maps; re-provision extends instead of duplicating; delete clears ownership

## 8. Traffic Statistics

- [ ] 8.1 Implement the periodic traffic collection job (~60 s): concurrent `FetchTrafficSnapshot` per node, persist `traffic_samples`
- [ ] 8.2 Implement cumulative→delta computation with counter-reset detection (decreasing sample = new absolute)
- [ ] 8.3 Implement aggregated usage queries: per-node, per-inbound, per-client totals + online flag + last-online
- [ ] 8.4 Implement bucketed traffic-history query for charts
- [ ] 8.5 Implement traffic reset endpoints (client / inbound / node)
- [ ] 8.6 Implement threshold/expiry rule evaluation emitting de-duplicated `client.*` events
- [ ] 8.7 Implement user-facing own-traffic endpoint scoped to owned clients
- [ ] 8.8 Test: counter reset produces no negative delta; user cannot see others' traffic

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
