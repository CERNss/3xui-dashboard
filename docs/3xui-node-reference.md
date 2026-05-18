# 3x-ui Node Reference

Authoritative reference for the **vanilla 3x-ui node API** that `3xui-dashboard`
controls, the **data structures** exchanged, and the **feature-extraction map**
(what we lift from upstream vs. what we drop).

Source scanned: `/Users/cern/LocalDisk/D/Repo/infra/cern-3x-ui` (3x-ui v3).
Keep this in sync if the pinned 3x-ui version changes.

---

## 1. Connection & Auth Model

Each node is a stock 3x-ui panel. The dashboard talks to it over HTTP(S).

| Aspect | Detail |
|---|---|
| Base URL | `{scheme}://{address}:{port}{basePath}` — `scheme` ∈ `http`/`https`, `basePath` normalized to be `/`-bounded (empty ⇒ `/`) |
| API root | `{basePath}panel/api` |
| Auth | `Authorization: Bearer {apiToken}` — node-side `ApiToken` row, matched by `apiTokenService.Match()` |
| Auth fallback | A logged-in session cookie also works, but the dashboard uses **only the Bearer token** |
| Unauthorized | `401` if `X-Requested-With: XMLHttpRequest`, otherwise `404` (3x-ui hides the panel) |
| CSRF | `/panel/api` group applies `CSRFMiddleware()`; token-authed requests pass through |
| Content types accepted | `application/json` and `application/x-www-form-urlencoded` (most handlers use `c.ShouldBind`, which accepts both) |

### Response envelope

Every `/panel/api/*` response is `entity.Msg`:

```json
{ "success": true, "msg": "human text", "obj": <payload | null> }
```

- `success=false` ⇒ treat `msg` as the error. `obj` may still be present.
- Decode `obj` as `json.RawMessage` first, then into the typed struct.

---

## 2. Node API Surface

### 2.1 Server — `{basePath}panel/api/server`

| Method | Path | Purpose | Used by dashboard |
|---|---|---|---|
| GET | `/status` | Host + Xray status (cpu/mem/uptime/version) | ✅ probe loop |
| POST | `/restartXrayService` | Restart Xray on the node | ✅ after config changes |
| POST | `/stopXrayService` | Stop Xray | ⚠️ optional |
| GET | `/getNewUUID` | Generate a UUID | ⚠️ optional (we can generate locally) |
| GET | `/getNewX25519Cert` | Reality keypair | ⚠️ optional (inbound creation helper) |
| GET | `/getConfigJson` | Full Xray config | ❌ not needed |
| GET/POST | `cpuHistory`, `history/*`, `xrayMetrics*`, `getXrayVersion`, `installXray`, `updatePanel`, `updateGeofile`, `logs`, `getDb`, `importDB`, ... | Local panel ops | ❌ dropped |

**`GET /status` → `obj` (`service.Status`)** — the only fields the probe uses:

```jsonc
{
  "cpu": 12.5, "cpuCores": 4, "logicalPro": 8, "cpuSpeedMhz": 2400,
  "mem":  { "current": 800000000, "total": 4000000000 },
  "swap": { "current": 0, "total": 0 },
  "disk": { "current": 0, "total": 0 },
  "xray": { "state": "running", "errorMsg": "", "version": "25.x.x" },
  "uptime": 123456,                       // seconds
  "loads": [0.1, 0.2, 0.3],
  "tcpCount": 10, "udpCount": 5,
  "netIO": { "up": 0, "down": 0 },
  "netTraffic": { "sent": 0, "recv": 0 },
  "publicIP": { "ipv4": "...", "ipv6": "..." },
  "appStats": { "threads": 20, "mem": 0, "uptime": 0 }
}
```

Probe consumes: `cpu`, `mem.current/total` (⇒ mem %), `xray.version`, `uptime`.

### 2.2 Inbounds — `{basePath}panel/api/inbounds`

| Method | Path | Purpose | Used |
|---|---|---|---|
| GET  | `/list` | All inbounds + per-client traffic stats | ✅ |
| GET  | `/get/:id` | One inbound by node-side id | ✅ |
| GET  | `/getClientTraffics/:email` | One client's traffic by email | ✅ |
| GET  | `/getClientTrafficsById/:id` | Client traffics by inbound id | ⚠️ |
| GET  | `/getSubLinks/:subId` | Link array for a subId (no base64) | ⚠️ (we generate links ourselves) |
| GET  | `/getClientLinks/:id/:email` | Link(s) for one client | ⚠️ |
| POST | `/add` | Create an inbound | ✅ |
| POST | `/update/:id` | Update an inbound (full settings JSON) | ✅ |
| POST | `/del/:id` | Delete an inbound | ✅ |
| POST | `/setEnable/:id` | Flip only the enable flag (cheap) | ✅ |
| POST | `/addClient` | Add client(s) to an inbound | ✅ |
| POST | `/updateClient/:clientId` | Update one client (by UUID/password) | ✅ |
| POST | `/:id/delClient/:clientId` | Delete one client | ✅ |
| POST | `/:id/delClientByEmail/:email` | Delete one client by email | ✅ |
| POST | `/:id/resetClientTraffic/:email` | Reset one client's counters | ✅ |
| POST | `/:id/resetTraffic` | Reset one inbound's counters | ✅ |
| POST | `/resetAllClientTraffics/:id` | Reset all clients on an inbound | ✅ |
| POST | `/resetAllTraffics` | Reset every counter on the node | ✅ |
| POST | `/onlines` | Emails currently online | ✅ traffic loop |
| POST | `/lastOnline` | `{email: unixTs}` last-seen map | ✅ traffic loop |
| POST | `/delDepletedClients/:id` | Delete traffic-exhausted clients | ⚠️ |
| POST | `/clientIps/:email` | IP log for a client | ⚠️ |
| POST | `/clearClientIps/:email` | Clear IP log | ⚠️ |
| POST | `/:id/copyClients` | Copy clients between inbounds | ❌ |
| POST | `/import` | Import an inbound from a blob | ❌ |
| POST | `/updateClientTraffic/:email` | Manually set up/down | ⚠️ |

> ✅ extract & use · ⚠️ available, use if needed · ❌ ignore

---

## 3. Data Structures

### 3.1 Inbound (`database/model/model.go`)

```jsonc
{
  "id": 1,                       // node-side id — UNSTABLE, do not key on it
  "up": 0, "down": 0,            // bytes; cumulative counters
  "total": 0,                    // traffic cap in bytes (0 = unlimited)
  "allTime": 0,                  // all-time usage
  "remark": "my-inbound",
  "enable": true,
  "expiryTime": 0,               // unix ms (0 = never)
  "trafficReset": "never",       // never|daily|weekly|monthly|...
  "lastTrafficResetTime": 0,
  "clientStats": [ ClientTraffic, ... ],
  "listen": "",                  // "" / 0.0.0.0 / :: ⇒ all interfaces
  "port": 443,
  "protocol": "vless",           // see enum below
  "settings": "{...}",           // STRINGIFIED JSON — holds clients
  "streamSettings": "{...}",     // STRINGIFIED JSON — transport/TLS/Reality
  "tag": "inbound-443",          // STABLE identifier — key on this
  "sniffing": "{...}",           // STRINGIFIED JSON
  "nodeId": null                 // 3x-ui's own node field; irrelevant to us
}
```

Key facts:
- `settings`, `streamSettings`, `sniffing` are **JSON encoded as strings**.
- **Clients live inside `settings`** as `{"clients":[Client,...], ...}`.
- `tag` is the stable handle. If absent on create, 3x-ui derives it:
  `inbound-{port}` (wildcard listen) or `inbound-{listen}:{port}`.
- Node-side `id` changes if an inbound is recreated — always resolve
  `tag → id` from `/list` and cache it.

### 3.2 Protocol enum

`vmess`, `vless`, `trojan`, `shadowsocks`, `mixed`, `http`, `wireguard`,
`hysteria`, `hysteria2`, `tunnel`. (`IsHysteria()` treats `hysteria`/`hysteria2`
together.)

### 3.3 Client (lives in `inbound.settings.clients[]`)

```jsonc
{
  "id": "uuid",            // VLESS/VMess identifier
  "password": "...",       // Trojan / Shadowsocks
  "security": "auto",
  "flow": "xtls-rprx-vision",
  "email": "alice",        // unique label — primary client handle
  "limitIp": 0,            // concurrent IP cap (0 = unlimited)
  "totalGB": 0,            // traffic cap in BYTES despite the name (0 = unlimited)
  "expiryTime": 0,         // unix ms (0 = never; negative = relative-from-first-use)
  "enable": true,
  "tgId": 0,               // Telegram id — unused by us
  "subId": "abc123",       // subscription id
  "comment": "",
  "reset": 0,              // traffic-reset period in days
  "created_at": 0, "updated_at": 0
}
```

### 3.4 ClientTraffic (`xray/client_traffic.go`) — read model in `inbound.clientStats[]`

```jsonc
{
  "id": 1, "inboundId": 1,
  "enable": true,
  "email": "alice",
  "uuid": "...",           // gorm:"-" (joined at runtime)
  "subId": "abc123",       // gorm:"-"
  "up": 12345, "down": 67890,   // cumulative bytes
  "allTime": 0,
  "expiryTime": 0,
  "total": 0,              // cap in bytes
  "reset": 0,
  "lastOnline": 0          // unix seconds
}
```

---

## 4. Wire Formats for Key Calls

### 4.1 Create / update inbound — `POST /add`, `POST /update/:id`

Body = an Inbound. Upstream `runtime/remote.go::wireInbound` sends
**form-encoded** (`application/x-www-form-urlencoded`):

```
total, remark, enable, expiryTime, listen, port, protocol,
settings, streamSettings, tag, sniffing, trafficReset
```

`settings`/`streamSettings`/`sniffing` are stringified JSON. JSON body also
works. Before sending, strip redundant TLS cert **file paths** when inline cert
content is present (`sanitizeStreamSettingsForRemote`).

### 4.2 Client mutation — two strategies

**Strategy A — surgical endpoints** (what our specs target):
- `POST /addClient` — body = Inbound JSON `{ "id": <inboundId>, "settings": "{\"clients\":[Client]}" }`
- `POST /updateClient/:clientId` — `clientId` = client UUID/password; body same shape
- `POST /:id/delClient/:clientId` — path-only

**Strategy B — full re-push** (what upstream `remote.go` actually does):
`AddUser`/`RemoveUser` just call `UpdateInbound` and re-push the entire inbound
with the modified `settings.clients[]`. Simpler, no `clientId` resolution, but
heavy for inbounds with thousands of clients.

> Decision for the dashboard: prefer **Strategy A** for single-client ops
> (provisioning, plan purchase); fall back to Strategy B only for bulk edits.

### 4.3 Traffic snapshot (per `remote.go::FetchTrafficSnapshot`)

1. `GET  /list`        → `[]Inbound` with `clientStats`
2. `POST /onlines`     → `["alice","bob"]` currently-online emails
3. `POST /lastOnline`  → `{"alice": 1716000000}` last-seen unix seconds

Steps 2 & 3 are best-effort — log and continue on failure.

### 4.4 Resets

| Call | Effect |
|---|---|
| `POST /:id/resetClientTraffic/:email` | one client up/down → 0 |
| `POST /resetAllClientTraffics/:id` | all clients on inbound → 0 |
| `POST /:id/resetTraffic` | inbound counters → 0 |
| `POST /resetAllTraffics` | every counter on the node → 0 |

---

## 5. Feature-Extraction Map

What we **extract / adapt** from upstream 3x-ui into `3xui-dashboard/backend`,
and what we **drop**.

### 5.1 Extract & adapt

| Upstream file | Into | Notes |
|---|---|---|
| `web/runtime/remote.go` | `internal/runtime` | Bearer transport, envelope decode, `tag→id` cache, `wireInbound`, cert sanitize, `FetchTrafficSnapshot` |
| `web/runtime/runtime.go`, `manager.go` | `internal/runtime` | `NodeRuntime` interface + `Manager` (Remote only; **drop `local.go`**) |
| `web/service/node.go` | `internal/service/node` | node CRUD, `normalize`, `Probe`, heartbeat patch, in-memory metric ring buffer + bucketed aggregation |
| `util/netsafe` (SSRF guard) | `internal/pkg/netsafe` | guarded dialer, `NormalizeHost`, allow-private context |
| `database/model/model.go` (`Inbound`, `Client`, `Node`) | `internal/model` | adapt: GORM→our schema, drop 3x-ui's own `User`/`Setting` |
| `xray/client_traffic.go` (`ClientTraffic`) | `internal/model` | traffic read model |
| `sub/subService.go` | `internal/sub` | link builders: VLESS/VMess/Trojan/SS URL assembly, remark model |
| `sub/subJsonService.go` | `internal/sub` | JSON subscription (Xray-client config) |
| `sub/subClashService.go` | `internal/sub` | Clash/Mihomo YAML subscription |
| `sub/links.go` | `internal/sub` | `LinkProvider` — `SubLinksForSubId`, `LinksForClient` |
| `sub/subController.go` | `internal/handler/sub` | public `/sub/:subId` routing, base64 wrap, headers |
| `entity.Msg` envelope shape | `internal/runtime` | for decoding node responses |

### 5.2 Drop entirely

- Local Xray process management — `xray/process.go`, `web/service/xray.go`,
  `panel*.go`, `installXray`, `updateGeofile`, geo-asset tooling.
- Telegram bot — `web/service/tgbot.go`.
- The legacy multi-page HTML UI — `web/dist/*`, `frontend/*.html`.
- 3x-ui's own auth (`web/service/user.go`, sessions, LDAP, TOTP) — replaced by
  our admin-auth + user-accounts.
- 3x-ui's own "node" feature controller (`web/controller/node.go`) — *we are*
  the central panel; we don't expose a node-management API to be polled.
- WARP / NordVPN integration, custom-geo, observatory metrics, panel
  self-update, DB import/export endpoints.
- WebSocket live-push (`web/websocket`) — optional, can add later if needed.

### 5.3 Net-new (no upstream equivalent)

- `admin-auth` — env-credential admin login.
- `user-accounts` — OIDC + email/password, email binding, SMTP, registration
  controls, domain allowlist.
- `client-ownership` — the user↔client mapping 3x-ui lacks.
- `billing-and-plans` — plans, balance, orders.
- `webhook-notifications` — outbound event webhooks.
- Multi-node aggregation — fleet-wide inbound/traffic views, one `subId`
  spanning clients on multiple nodes.

---

## 6. Subscription Internals (from `sub/`)

The dashboard generates subscriptions **itself** (does not proxy node `/sub`).

| Format | Route (ours) | Output |
|---|---|---|
| Base64 | `GET /sub/:subId` | newline-joined links, base64-encoded |
| JSON | `GET /sub/json/:subId` | Xray-client JSON config (fragment/noise/mux/rules optional) |
| Clash | `GET /sub/clash/:subId` | Clash/Mihomo YAML (proxies + proxy group) |

Response headers to emit:
- `Subscription-Userinfo: upload=..; download=..; total=..; expire=..`
- `Profile-Update-Interval: <hours>`
- `Profile-Title`, `Profile-web-page-url` (optional, from settings)

Link generation needs the **inbound config** (`settings`, `streamSettings`,
host/port). The dashboard fetches inbound configs from nodes (short-TTL cache)
and feeds them to the extracted `sub` builders. `remarkModel` (default `-ieo`:
inbound-email-other) controls how each link is named.

---

## 7. Open Items to Pin Before Coding

- **3x-ui version range**: this reference reflects the `cern-3x-ui` checkout
  (v3). Pin a supported min/max and re-scan on bump.
- Confirm `addClient`/`updateClient` body shape against a live node — upstream
  `remote.go` never calls them (it re-pushes inbounds), so the surgical-endpoint
  contract is inferred from the controller signatures, not exercised upstream.
- `totalGB` field is **bytes**, not GB — verify against a node before relying.
- `expiryTime` negative-value semantics (relative-from-first-use) — confirm if
  we expose duration-based plans.
