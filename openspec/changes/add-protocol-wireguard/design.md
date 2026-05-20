# Design — add-protocol-wireguard (post-T0, v2)

> **Status**: T0 capture done (2026-05-20). See
> `notes/3xui-wg-api.md` for raw findings. This document supersedes
> the original v1 design — kept in git history if archeology
> needed.

## What changed vs v1

v1 assumed stock 3x-ui needs a separate WG panel surface that we'd
wrap via a new `runtime.WGClient` interface. T0 disproved that:
the deployed fork unifies WG under the same `/panel/api/inbounds/*`
endpoint set, distinguishing by `protocol="wireguard"` + a
WG-specific `settings` JSON shape. No new runtime client. No new
auth path. No new wire envelope.

What stays carved out:
1. **Subscription rendering** — WG outputs `.conf` not URI
2. **Peer storage** — `peers[]` nested in inbound settings (not a
   `/clients/*` entry), so peer add/remove = read-modify-write
   the whole inbound
3. **Key custody** — node generates both halves of the keypair;
   we read + AES-encrypt + store locally

## Architecture overview

```
                              Existing path (XrayClient)
              ┌────────────────────────────────────────────┐
              │  3xui-dashboard                            │
              │                                            │
              │  ┌─────────────────────────┐               │
              │  │ XrayClient (per-node)    │              │
              │  │                          │              │
              │  │   AddInbound(protocol="vless"/"vmess"   │
              │  │              /"trojan"/"shadowsocks"    │
              │  │              /"wireguard"/"hysteria")   │
              │  │                                         │
              │  │   AddClient(email, inboundId, ...)      │
              │  │     ↓                                   │
              │  │     uses /panel/api/clients/add         │
              │  │     for VLESS/VMess/Trojan/SS/Hysteria  │
              │  │                                         │
              │  │   UpdateInbound(id, full settings)      │
              │  │     ↓                                   │
              │  │     uses /panel/api/inbounds/update/:id │
              │  │     used by WG peer add/remove          │
              │  │     (read settings → mutate peers[] →   │
              │  │      write back)                        │
              │  └─────────────────────────────────────────┘
              └────────────────────────────────────────────┘
                          │
                          ▼ Bearer token
              ┌─────────────────────────────────────────┐
              │ 3x-ui fork                              │
              │  /panel/api/inbounds/*                  │
              │  /panel/api/clients/*                   │
              │  /panel/api/server/*                    │
              └─────────────────────────────────────────┘
```

## API contract for WG (captured from T0)

### Create WG inbound

```
POST /panel/api/inbounds/add
Authorization: Bearer <token>
Content-Type: application/json   (or x-www-form-urlencoded with field
                                  names matching the JSON keys —
                                  empirically the latter also works)

{
  "remark": "wg-tokyo",
  "enable": true,
  "expiryTime": 0,
  "listen": "",
  "port": 51820,
  "protocol": "wireguard",
  "settings": "{...JSON string of wgSettings...}",
  "streamSettings": "",            // EMPTY for WG (no transport layer)
  "sniffing": "{...JSON string...}"
}
```

`settings` for `protocol="wireguard"`:

```json
{
  "mtu": 1420,
  "secretKey": "<base64 32 bytes>",   // server private key (node-generated)
  "peers": [
    {
      "privateKey": "<base64>",       // peer private key (node-generated!)
      "publicKey":  "<base64>",
      "allowedIPs": ["10.0.0.2/32"],
      "keepAlive":  0
    }
  ],
  "noKernelTun": false
}
```

### Add WG peer (no atomic endpoint — RMW)

```
1. GET  /panel/api/inbounds/get/<id>           ← read full inbound
2. parse settings JSON → mutate peers[] (append/remove)
3. POST /panel/api/inbounds/update/<id>
   { ...full inbound row with new settings string... }
```

**Concurrency hazard**: two callers can step on each other.
Mitigation v1: `pg_advisory_xact_lock(inbound_id)` around the
RMW window. v2: optimistic check (re-read, compare peer count,
retry once).

### Subscription rendering

Per `notes/3xui-wg-api.md`, our dashboard derives the WG `.conf`
locally rather than asking the node for a pre-rendered one:

```ini
[Interface]
PrivateKey = <decrypted from wg_peers.private_key_encrypted>
Address    = <wg_peers.allocated_ip>/<subnet bits>
DNS        = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey           = <derived from inbound settings.secretKey via curve25519 scalar mult>
Endpoint            = <node.host>:<inbound.port>
AllowedIPs          = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

The fork stores `secretKey` (server's private key) but doesn't
return a `publicKey` field. We derive client-side via
`golang.org/x/crypto/curve25519.X25519(secretKey, BasePoint)`.

## Where Xray-shaped protocols live vs WG

| Protocol | Inbound endpoint | Client add endpoint | Settings shape | URI sub format |
|---|---|---|---|---|
| vless / vmess / trojan / shadowsocks | `/inbounds/add` | `/clients/add` w/ inboundIds | `clients[]` of `Client{id/password/...}` | URI scheme |
| **wireguard** | same `/inbounds/add` | ❌ no client API path — RMW via `/inbounds/update/:id` | `peers[]` of WGPeer | `.conf` text |
| hysteria | same `/inbounds/add` | `/clients/add` ✓ | `clients[]` of `Client{auth: ...}` | `hysteria2://` URI |

WG is the ONLY protocol that doesn't fit the `/clients/*` API
shape. Everything else uses the unified flow.

## Concurrency: pg_advisory_xact_lock

```go
func (s *ClientService) AddWGPeer(ctx, inboundID int64, peer WGPeer) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        // advisory lock — one mutation at a time per inbound
        if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, inboundID).Error; err != nil {
            return err
        }
        inbound, err := s.rt.Inbound(ctx, inboundID)   // GET /inbounds/get/:id
        if err != nil { return err }

        var settings wgSettings
        json.Unmarshal([]byte(inbound.Settings), &settings)
        settings.Peers = append(settings.Peers, peer)
        newSettings, _ := json.Marshal(settings)
        inbound.Settings = string(newSettings)

        return s.rt.UpdateInbound(ctx, inboundID, inbound)  // POST /inbounds/update/:id
    })
}
```

The advisory lock is shed at COMMIT — by then the inbound on the
node is already updated. Lock duration = one round-trip to the
node, typically <200ms. Acceptable serialization for v1.

### Limitation: lock doesn't extend to panel UI

The `pg_advisory_xact_lock` only serializes mutations within the
dashboard process. **If an operator simultaneously edits the same
WG inbound through the 3x-ui panel UI (or another dashboard
instance against the same node), peer updates can still race.**
The fork's `/inbounds/update/:id` endpoint accepts the full
inbound JSON wholesale — no optimistic-concurrency check on a
`version` or `updated_at` field exists in the fork's response
schema (T0 verified the captured shape: no version field).

Documented operator constraint: "while the dashboard is
provisioning users to a WG inbound, don't simultaneously edit
that inbound from the 3x-ui UI." Likelihood of harm low (admins
typically don't co-edit), but the constraint deserves to be in
docs. v2 may add panel-side optimistic locking — needs an
upstream change to MHSanaei/3x-ui, out of scope here.

## Capability detection — REMOVED

Original plan: call `GET /panel/api/inbounds/options` to enumerate
supported protocols per node. **Verified 2026-05-20: this
endpoint returns 404 with Bearer-token auth.** The controller
calls `session.GetLoginUser(c)` which only resolves for cookie-
session callers, not API-token callers (per `web/controller/inbound.go`
in MHSanaei/3x-ui).

Revised stance: **don't detect, just target.** We declare in spec
+ docs that this dashboard targets MHSanaei/3x-ui (recent commits,
either branch). That fork has WG + Hysteria built in. There is no
"canonical 3x-ui without WG" upstream — MHSanaei IS the canonical
maintainer.

Implications:
- No `nodes.supported_protocols TEXT[]` column — drop from the
  schema migration
- No probe-time capability check
- If an operator runs a different fork that doesn't speak WG,
  the inbound add will fail server-side with a protocol-not-
  supported error; the admin UI surfaces that as a regular error
  (no special branching)
- Documented in `docs/operator/3xui-fork-compat.md`: "this dashboard
  requires MHSanaei/3x-ui; other forks at your own risk"

## Schema additions (REVISED from v1)

```sql
-- wg_peers: 1:1 with client_ownerships rows that target a WG inbound.
-- Source of truth for the peer's PRIVATE key (encrypted).
CREATE TABLE wg_peers (
    id                    BIGSERIAL PRIMARY KEY,
    client_ownership_id   BIGINT NOT NULL UNIQUE REFERENCES client_ownerships(id) ON DELETE CASCADE,
    public_key            TEXT NOT NULL,
    private_key_encrypted BYTEA NOT NULL,
    allocated_ip          INET NOT NULL,
    inbound_id            BIGINT NOT NULL,   -- denormalized for RMW lookup
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- (No supported_protocols column — capability detection removed,
-- per the "Capability detection — REMOVED" section above. Spec
-- targets MHSanaei/3x-ui as a monolithic surface.)
```

Drop the v1 `node_inbound_snapshots.is_wireguard` column — the
protocol is already on the inbound row itself, we don't need a
mirror.

## Key custody (revised security model)

v1 said "dashboard generates locally, private key never traverses
3x-ui's API". T0 ruled this out — the fork generates server-side.

Revised model:
1. Dashboard creates WG inbound via `/inbounds/add` with an empty
   `peers: []`
2. Dashboard provisions a peer by calling `/inbounds/update/:id`
   with `peers: [{...}]` where the privateKey/publicKey are
   GENERATED LOCALLY via `curve25519` BEFORE the call
3. Read-back via `/inbounds/get/:id` to confirm the node accepted
   the keypair (this is the privacy-preserving path)

**Test in T1**: does the fork accept dashboard-supplied peer
keypairs, or does it overwrite them with its own? If the latter,
we lose the privacy property and fall back to "read back the
peer's privateKey after the node generates it, AES-encrypt
immediately". Either way our `wg_peers.private_key_encrypted`
holds the truth.

## What v1 got right (preserved)

- `wg_peers` table 1:1 with `client_ownerships`
- `WG_MASTER_KEY` AES-256-GCM env var for private-key encryption
  at rest
- `.conf` + `wireguard-zip` subscription formats
- Clash + sing-box WG outbound emission for mixed subscriptions
- URI base64 / SIP008 skip WG with `X-Subscription-Hint` header
  (covered in the subscription spec delta)

## What we'll regret if we don't do it this way

- **Try to add a WGClient interface anyway** — pointless
  abstraction when the wire is the same
- **Pre-allocate peer IPs locally before calling the node** —
  the node already does this; we just read back `allowedIPs`
- **Bypass `/clients/*` for Hysteria too** — Hysteria fits the
  unified flow; reusing the same code path is half the diff
- **Skip the advisory lock** — peer-add races on a busy panel
  will silently lose entries

---

# Historical: v1 design (superseded)

The v1 design assumed a separate WG panel surface needing
`runtime.WGClient`. Kept in this file's git history for the
record. Diff against this v2 by checking out the commit before
"T0 capture" landed.
