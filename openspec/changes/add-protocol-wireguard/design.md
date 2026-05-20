# Design — add-protocol-wireguard

## Why this is structurally different from #1-#7

Every previous protocol-adjacent change extended the same Xray-shaped
abstractions: a `runtime.Inbound` row, an `inbound-management`
editor with transport/security tabs, a `BuildLink` switch that emits
`vless://` / `vmess://` / `trojan://` / `ss://`. WireGuard breaks
the model:

| Dimension | Xray-shaped (4 existing) | WireGuard |
|---|---|---|
| Engine | Xray-core | wireguard-go / kernel module |
| API surface on 3x-ui | `/panel/api/inbounds/*` | `/panel/api/wireguard/*` (separate) |
| Transport | TCP/TLS layered | UDP, single layer |
| Auth | Per-client UUID / password | Per-peer ed25519/curve25519 keypair |
| Sub format | URI scheme + base64 | `.conf` file |
| Client identity | `email` field on inbound settings | `[Peer]` block matched by PublicKey |
| Enable/disable | `enable: false` on client | Add/remove from `[Peer]` list |

The design choice is to **carve out a WG-specific path through
each layer** rather than try to unify with the existing 4. Trying
to unify produces a leaky abstraction (every callsite branches on
"is WG?" anyway).

## Architecture overview

```
              ┌──────────────────────────────────────────────────────┐
              │  3xui-dashboard                                      │
              │                                                      │
              │  ┌──────────────────┐  ┌────────────────────────┐   │
              │  │ Xray inbound     │  │ WireGuard inbound      │   │
              │  │ flow (existing)  │  │ flow (NEW)             │   │
              │  └────────┬─────────┘  └───────────┬────────────┘   │
              │           │ runtime.AddClient      │ runtime.AddWGPeer
              │           │ runtime.UpdateClient   │ runtime.RemoveWGPeer
              │           ▼                        ▼                 │
              │   ┌───────────────┐        ┌──────────────────┐     │
              │   │ XrayClient    │        │ WGClient (NEW)   │     │
              │   │ (per-node)    │        │ (per-node)       │     │
              │   └───────┬───────┘        └────────┬─────────┘     │
              └───────────┼─────────────────────────┼───────────────┘
                          │ /panel/api/inbounds/*   │ /panel/api/wireguard/*
                          ▼                         ▼
                  ┌─────────────────────────────────────┐
                  │ 3x-ui panel on node                 │
                  │  (single Bearer token serves both   │
                  │   surfaces)                         │
                  └─────────────────────────────────────┘
```

## Runtime client split

Today `runtime.Manager` exposes a single client type per node. The
clean version: keep `Manager` as the registry but split the
per-node client into two interfaces.

```go
package runtime

// XrayClient is the existing surface, renamed.
type XrayClient interface {
    ListInbounds(ctx) ([]Inbound, error)
    AddClient(ctx, tag string, c Client) error
    UpdateClient(ctx, tag string, c Client) error
    // …
}

// WGClient is the new WG panel surface.
type WGClient interface {
    ListWGInbounds(ctx) ([]WGInbound, error)
    AddWGPeer(ctx, inboundID int64, p WGPeer) error
    RemoveWGPeer(ctx, inboundID int64, publicKey string) error
}

// Node bundles both — Manager.Get returns this.
type Node interface {
    XrayClient
    WGClient
    Probe(ctx) (Probe, error)  // unified, both surfaces healthy
}
```

`Probe` returns `online` only when BOTH `/inbounds/list` AND
`/wireguard/list` (or whatever the actual paths are) return 200.
If WG isn't installed on a particular node, the WG endpoint
returns 404; the probe SHOULD treat 404 as "WG capability absent"
not "node offline" — the node can still serve Xray inbounds.

## Key generation: client-side

Choose **dashboard generates the keypair**, not the node. Reasons:

- Private keys never traverse 3x-ui's API surface
- No dependency on a specific 3x-ui WG endpoint version's behavior
  around key returns
- `golang.org/x/crypto/curve25519` is stdlib-adjacent (we already
  pull in golang.org/x/crypto for password hashing)

Flow:
1. `ProvisionClient(user, wg_inbound)` generates a fresh curve25519
   keypair locally
2. Allocates next-free IP from `wg_inbound.subnet` (transactional)
3. Stores `(public_key, private_key_encrypted, allocated_ip)` in
   a new `wg_peers` table
4. Calls `WGClient.AddWGPeer(inbound_id, {public_key, allowed_ips: allocated_ip/32})`
   — only the PUBLIC key goes to the node
5. Subscription handler decrypts + emits the private key in the
   `.conf` file

Private key encryption uses a dashboard-level master key from a
new `WG_MASTER_KEY` env var (32-byte AES key, hex-encoded). If the
operator loses it, all WG peers must be regenerated — documented as
a "rotate carefully" footnote in `.env.example`.

## Subscription format

```ini
[Interface]
PrivateKey = <generated>
Address = <allocated_ip>/<subnet_bits>
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = <inbound.server_public_key>
Endpoint = <node.host>:<inbound.listen_port>
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

- **DNS**: hardcoded 1.1.1.1 + 8.8.8.8 in v1. Per-deployment override
  via settings is a follow-up.
- **AllowedIPs**: full-tunnel (`0.0.0.0/0, ::/0`) by default. Split-
  tunnel = follow-up.
- **PersistentKeepalive**: 25s (the WireGuard quickstart default).
  Important for NAT scenarios.

`?format=wireguard` returns one config for the user's first active
WG peer. `?format=wireguard-zip` returns a ZIP when multiple peers
exist (e.g. user has WG enabled on tokyo-1 AND singapore-1).

The Clash + sing-box converters gain a WG outbound stanza per peer
when mixed subscriptions are requested. URI-bundle + SIP008 formats
SKIP WG entries (no representation possible).

## Schema additions

```sql
-- New table: 1:1 with client_ownerships rows that target a WG inbound.
CREATE TABLE wg_peers (
    id                    BIGSERIAL PRIMARY KEY,
    client_ownership_id   BIGINT NOT NULL UNIQUE REFERENCES client_ownerships(id) ON DELETE CASCADE,
    public_key            TEXT NOT NULL,
    private_key_encrypted BYTEA NOT NULL,
    allocated_ip          INET NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX wg_peers_ownership ON wg_peers (client_ownership_id);

-- WG-specific fields on the existing inbounds metadata.
-- (Stored as JSON in settings today; new flag distinguishes WG.)
ALTER TABLE node_inbound_snapshots
    ADD COLUMN is_wireguard BOOLEAN NOT NULL DEFAULT FALSE;
```

`client_ownerships` does NOT gain a column — the WG-ness is derived
from the inbound (one WG inbound, all its ownerships are WG-typed).

## Revocation path

ExpiryJob today calls `XrayClient.UpdateClient(tag, {Enable: false})`
on the node side. WG has no "disable" flag — peers must be REMOVED.

Plan: ExpiryJob's `disableOnNode` helper grows a branch:
```go
if ownership.IsWG() {
    rt.RemoveWGPeer(inboundID, ownership.WGPublicKey)
} else {
    rt.UpdateClient(tag, runtime.Client{Email: o.ClientEmail, Enable: false})
}
```

When the order is renewed (user buys again), provisioning re-adds
the SAME public key (we keep the row in `wg_peers`, just re-issue
the AddWGPeer call). User's existing config file still works — no
re-download required.

## Frontend changes

- **Inbounds list page**: new tab/section "WireGuard". Lives next
  to the existing 4-protocol table.
- **WG inbound editor**: completely new component — no
  transport/security tabs. Fields: listen port, subnet, server
  public/private keys (server-generated on first save).
- **WG peer list inside an inbound**: shows allocated IPs + masked
  public keys. Useful for ops debugging.
- **Portal subscription page**: existing format picker gains
  "WireGuard" — when active, button text changes to "下载配置文件"
  (download .conf) instead of "扫码 / 复制 URL".

## What we'll regret if we don't do it this way

- **Try to fold WG into the existing Inbound model** — every editor,
  every BuildLink call, every settings parser gets an "is this WG?"
  branch. Carving out a separate path is the cleaner cost.
- **Trust the node to manage keypairs** — couples us to a specific
  3x-ui WG endpoint version's behavior, and exposes private keys
  to 3x-ui's internal logs.
- **Skip the wg_peers table, derive everything from ownership** —
  ownership rows lose the public key, can't reconstruct subscription
  after a node ↔ dashboard config drift.
- **Ship without verifying 3x-ui WG endpoint shape** — if the actual
  path differs from this design's assumed `/panel/api/wireguard/*`,
  the runtime client refactor balloons. Prereq before T1.
