# add-protocol-wireguard

## Why

The 多协议支持 pillar today covers 4 protocols (VLESS, VMess, Trojan,
Shadowsocks) — every one of them an Xray-core inbound spoken over
TCP/TLS. WireGuard is the headline missing piece for users who want
kernel-fast UDP tunnels (mobile, IoT, mesh routing) and is the
single most common request from users coming from sspanel-uim where
WG was a first-class offering.

The reason WireGuard wasn't shipped earlier with the rest of the
protocols is architectural: **Xray-core does not support WireGuard
as an inbound**. WG has no URI scheme, no TLS, no transport
abstraction — it's its own UDP-based VPN protocol that runs as a
kernel module (or `wireguard-go` userspace daemon) outside Xray's
process. Recent 3x-ui releases (≥ v2.4) added a dedicated WireGuard
panel section that manages WG independently of Xray.

Per `openspec/specs/README.md` the pillar's gap is "★ WireGuard
runtime + sub" — that's what this change closes.

## What Changes

### Modified capability: `runtime-3xui-client`

Adds a second API surface to the runtime client targeting 3x-ui's
WireGuard panel routes (separate from the existing
`/panel/api/inbounds/*` Xray surface).

- **New endpoints**: `GET /panel/api/wireguard/list`,
  `POST /panel/api/wireguard/add` (or whatever the 3x-ui WG panel
  exposes — needs verification per assumption below)
- **New types**: `runtime.WGInbound`, `runtime.WGPeer`
- **Reuses**: existing Bearer-auth + `{success, msg, obj}` envelope
  + SSRF guard

### Modified capability: `inbound-management`

The inbound editor + listing recognize a 5th "protocol" — WireGuard
— with a fundamentally different shape:

- No transport / security tabs (the editor's 8×3 matrix doesn't apply)
- Different fields: server private key, listen port, IPv4/IPv6 subnet,
  optional MTU, optional pre-shared key per peer
- Peer management is per-client, not per-inbound (each user gets one
  WG peer assigned to the WG inbound)

The admin Inbounds page gains a "WireGuard" tab/section; the
existing 4-protocol editor stays untouched.

### Modified capability: `client-provisioning`

When provisioning a client onto a WG inbound:
- Generate a fresh WG keypair for the peer (server keeps public,
  hands private back to user via subscription)
- Assign an IP from the inbound's subnet (sequential, low watermark)
- Push the peer to the node's WG config via the new runtime client
- Record in `client_ownerships` with a new `wg_peer_public_key`
  column (so we can revoke without re-generating the keypair)

### Modified capability: `subscription`

Adds two output formats:

- **`?format=wireguard`** — single WG `.conf` file for the user's
  active WG peer. Plain text, Content-Type `application/x-wireguard-conf`.
- **`?format=wireguard-zip`** — when the user has multiple WG peers
  across nodes, returns a ZIP with one `.conf` per peer.

The Clash + sing-box formats also gain WG outbound entries for users
whose subscription mixes WG with the existing protocols. SIP008 +
the URI base64 format are NOT extended (no WG URI scheme exists).

## Critical assumptions to verify before implementation

These need to be confirmed against the actual 3x-ui deployment
before T1 starts:

1. **3x-ui version supports WG panel**. Minimum version with a
   stable WG section TBD; design assumes `≥ v2.4`. If the operator's
   nodes run older versions, this change is a no-op for those nodes.
2. **WG API endpoint shape**. The 3x-ui codebase recently moved
   WG endpoints around — implementation should sniff the exact
   path + response envelope by inspecting one running node before
   writing the client.
3. **Peer key generation location**. Either:
   - Node generates the keypair (we receive both halves, hand
     private to user) — simpler but the private key transits us
   - Dashboard generates locally with `golang.org/x/crypto/curve25519`
     — never sends private key to node, but doubles the surface area
   The design defaults to the second (more secure) but the change
   should re-evaluate based on what 3x-ui's WG panel API expects.

## Out of scope

- **Multiple WG peers per user.** v1 = 1 user × 1 WG inbound × 1 peer.
  Multi-peer (e.g. one phone + one laptop) deferred.
- **Pre-shared keys (PSK).** Optional per-peer PSK adds round-trips
  and noise. Defer.
- **Dynamic endpoint / NAT holepunching.** Server endpoint is
  static (node's public IP + WG listen port).
- **MTU per-peer.** v1 uses inbound-level default (1420 or whatever
  the node config says).
- **QR for WG configs.** A typical WG config is ~250 chars including
  the private key — too long for low-error-correction QR codes that
  most mobile scanners support. Sub is download-only.
- **WG over UDP-via-TCP fronting (wstunnel, udp2raw).** WG-purist
  deployment only.

## Risks

- **If 3x-ui's WG panel API surface diverges between v2.4 and v2.x,**
  the runtime client may need to detect version and branch. The
  current runtime client doesn't do version detection — adding it
  would be a separate prerequisite change.
- **If the node hosts both Xray + WG processes,** port conflicts
  are possible. The inbound add endpoint should reject if the chosen
  listen port collides with an existing Xray inbound. Defer to T5
  with a regression test.
- **Peer key revocation.** Disabling a client today flips a DB flag
  + (optionally) the node-side enable bit. WG has no enable bit —
  to revoke, the peer must be REMOVED from the node's
  `[Peer]` block. Implementation must make sure ExpiryJob's
  node-side disable path calls the WG remove endpoint, not just the
  Xray UpdateClient with Enable=false.
