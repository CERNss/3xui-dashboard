# 3xui-dashboard

A central control panel for fleets of stock 3x-ui nodes.

The dashboard replaces 3x-ui's per-node admin UI + subscription endpoint
with one unified surface: admins see every inbound, client, and traffic
counter across every node; end users get one subscription URL that
aggregates clients living on multiple nodes; plans + ownership +
auditable balance let you turn a homelab fleet into a tiny commercial
service if you want to.

It is **not** a fork of 3x-ui — every node keeps running vanilla 3x-ui;
the dashboard speaks to each node over the public `{basePath}panel/api`
surface with the panel's own Bearer token.

## What's in v1

- **Multi-node fleet** — admin manages nodes (host, scheme, port,
  base-path, api-token), with periodic probe + cpu/mem metric ring
  buffer. Status transitions emit `node.online` / `node.offline` /
  `node.probe_failed` events.
- **Inbound + client CRUD** mediated through `runtime.NodeRuntime`
  (Bearer-auth, envelope decode, tag→id cache, SSRF-guarded
  transport, TLS-cert-path sanitization).
- **`ProvisionClient`** flow used by both admin "create client" and
  user "purchase plan" — creates or extends a panel-side client and
  upserts the matching `ClientOwnership` row.
- **Traffic statistics** — 60s collector + counter-reset-safe delta
  computation + bucketed history charts.
- **Central subscription** at `GET /sub/:subId` and `/sub/json/:subId`
  — one URL spans every node a user owns clients on, with
  `Subscription-Userinfo` headers. VLESS/VMess/Trojan/Shadowsocks
  link formats implemented.
- **Portal user accounts** — email/password registration (gated by
  `PUBLIC_REGISTRATION` and `EMAIL_DOMAIN_ALLOWLIST`), bcrypt-hashed
  passwords, admin moderation (list/suspend/balance/delete).
- **Billing** — plans + idempotent `Purchase` (charge → provision →
  refund on failure), order history, `balance_logs` audit trail.
- **Outbound webhooks** — admin-configured event subscriptions,
  HMAC-SHA256 signed envelopes, SSRF-guarded delivery transport
  (with per-webhook `allow_private` override), exponential-backoff
  retry, full delivery log + test + replay.

Shipped post-v1:

- **OIDC SSO** (`/api/user/auth/oidc/*`) — authorization-code + PKCE,
  RS256 verification, JWKS cache with refetch-on-kid-miss. Verified
  against Zitadel v2.71.10; see `docs/operator/oidc-setup.md`.
- **SMTP / email verification** — 6-digit code with 10m TTL, hashed
  at rest, 60s resend cooldown. Email-binding now sets
  `email_verified=true` after a successful Consume.
- **Admin frontend** — Nodes / Inbounds / Users / Plans / Orders /
  Stats / Webhooks / Audit Log / Settings all shipped.
- **Portal pages** — Subscription / Usage / Plans / Orders / Profile
  + OIDC callback.
- **Messages / notifications split** — `service/messages` is the
  user-facing SMTP surface; `service/notify` + `service/webhook`
  cover ops fanout.

Still deferred (operator opt-in / future):

- Clash YAML subscription format (base64 + JSON ship today).
- CI workflow (`.github/workflows/`).
- Headless browser E2E for the full OIDC redirect chain.

## Prerequisites

- Go 1.26
- Node.js 22+
- PostgreSQL 16+ (docker-compose includes one)
- One or more **stock 3x-ui** nodes you control, each with an API
  token issued from `Settings → API Tokens` on the node's admin UI.

## Quickstart (docker)

```sh
cp deploy/.env.example deploy/.env
$EDITOR deploy/.env   # set JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD

docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d
```

The dashboard is now on `http://localhost:8080`. Log in at
`/admin/login` and POST `/api/admin/nodes` with each node's
host/port/api_token.

## Quickstart (local dev)

```sh
# 1. Postgres
docker run --rm -d --name pg-dashboard \
  -e POSTGRES_PASSWORD=dashboard \
  -e POSTGRES_USER=dashboard \
  -e POSTGRES_DB=dashboard \
  -p 5432:5432 postgres:16-alpine

# 2. Configure
cp deploy/.env.example deploy/.env
$EDITOR deploy/.env

# 3. Run
make dev                # frontend on :5173, backend on :8080
```

## Onboarding a 3x-ui node

1. SSH into the node, log in to the 3x-ui admin UI.
2. *Settings → API Tokens → Add token*. Copy the token.
3. On the dashboard: `POST /api/admin/nodes`

   ```json
   {
     "name": "tokyo-1",
     "scheme": "https",
     "host": "node1.example.com",
     "port": 2053,
     "base_path": "",
     "api_token": "<the token>",
     "enabled": true
   }
   ```

4. The dashboard's probe loop will mark the node `online` within
   30 seconds. Inbounds + clients become visible immediately
   under `/api/admin/inbounds` and `/api/admin/clients/...`.

## Supported 3x-ui version range

Tested against the upstream tree at `cern-3x-ui` (v3). Older v2.x
should work for read-only flows (probe, list); client mutation via
the surgical `/addClient` endpoint may not be available on older
versions and the runtime falls back to the full inbound re-push
transparently.

If you bump 3x-ui across a major release, re-validate by:

1. Adding the node to a staging dashboard.
2. Provisioning a test plan against it.
3. Confirming the subscription URL renders correct links.

If anything regresses, check `docs/3xui-node-reference.md` for the
wire-format assumptions the runtime makes and file an issue.

## Configuration

Every config knob is documented in `deploy/.env.example`. Required:

- `DATABASE_URL`
- `JWT_SECRET` (≥32 random bytes)
- `ADMIN_USERNAME` + `ADMIN_PASSWORD`

Optional (have working defaults or graceful degradation):

- `LISTEN_ADDR`, `READ_TIMEOUT`, `WRITE_TIMEOUT`, `SHUTDOWN_TIMEOUT`
- `LOG_LEVEL`, `LOG_FORMAT` (auto-picks JSON in prod, text in dev)
- `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_MIGRATE_ON_BOOT`
- `ACCESS_TOKEN_TTL`
- `PUBLIC_REGISTRATION`, `EMAIL_DOMAIN_ALLOWLIST` (also runtime-
  mutable via the `settings` table)

Stubbed in v1 but the config slot is present:

- `OIDC_*`
- `SMTP_*`

## Useful Makefile targets

| Target            | What it does |
|-------------------|--------------|
| `make build`      | Build SPA + Go binary into `./3xui-dashboard` |
| `make dev`        | Vite dev server + Go server, both with hot reload |
| `make test`       | `go test ./...` + `vue-tsc --noEmit` |
| `make lint`       | `go vet` + ESLint |
| `make docker-build` / `make docker-up` / `make docker-down` | Container lifecycle |

## Project layout

```
backend/
  cmd/dashboard/       # main package
  internal/
    config/            # viper-backed env loader
    handler/{admin,public,user}/   # gin handlers
    middleware/        # auth middleware (admin / user / claims)
    model/             # GORM-mapped persistence schema
    repository/        # DB access
    runtime/           # 3x-ui node client (transport, envelope, cache, manager)
    service/           # domain logic (auth, billing, client, event, inbound, node, traffic, user, webhook)
    sub/               # central subscription assembler + link builders
    netsafe/           # SSRF-guarded dialer
    job/               # cron jobs (probe, traffic)
    web/               # //go:embed dist + SPA fallback
  migrations/          # //go:embed of *.sql

frontend/
  src/
    api/{admin,portal}/  # axios-backed API client modules
    components/layout/   # AdminLayout / PortalLayout / AuthLayout
    router/              # admin + portal route trees + auth guards
    stores/              # adminAuth + portalAuth + app
    views/{admin,portal}/  # pages

deploy/
  .env.example
  Dockerfile
  docker-compose.yml

docs/
  3xui-node-reference.md  # authoritative 3x-ui API + wire-format notes
```

## Status

This is the initial bootstrap. See `openspec/changes/bootstrap-central-panel/tasks.md`
for the line-by-line ship/deferred list.

Co-authored with [Claude Code](https://claude.com/claude-code).
