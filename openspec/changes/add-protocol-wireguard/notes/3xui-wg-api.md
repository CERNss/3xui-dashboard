# 3x-ui WireGuard API surface — T0 capture results

**Date probed**: 2026-05-20
**Node**: `node-1.bwg.us.tcg12345.win:10138` (live production node)
**Xray-core version**: `v26.4.25` (also has `v26.5.9` available)
**3x-ui fork**: stock-flavored (Vue 3 + Vite SPA front-end, Bearer-
token API; CSRF on session paths). Exact panel version not exposed
via any API I could find; the build looks current.

## TL;DR

**There is no WireGuard API surface on this 3x-ui build.** All probed
WG-likely paths return 404. Xray-core (the engine) has no native
WG inbound. The Xray config dump contains zero `wireguard` strings.

The proposal's load-bearing assumption — that recent 3x-ui ships
a separate WG panel section — is **FALSE** for this build, which
appears to be the canonical stock 3x-ui.

## What works (auth confirmed)

| Method | Path | Notes |
|---|---|---|
| GET | `/panel/api/inbounds/list` | full inbound + clients list, `{success,msg,obj}` envelope |
| GET | `/panel/api/server/status` | CPU/mem/disk/xray.version/uptime |
| GET | `/panel/api/server/getXrayVersion` | list of available Xray versions |
| GET | `/panel/api/server/getConfigJson` | full Xray config JSON |
| GET | `/panel/api/server/getDb` | DB dump (sqlite blob) |
| GET | `/panel/api/server/getNewX25519Cert` | generate Reality keypair |

Bearer-token works for `/panel/api/*`. Session login (`POST /login`)
requires the panel UI's CSRF flow and 2FA — token alone is sufficient
for the dashboard's needs.

## What's 404 (WireGuard probe matrix)

```
404  /panel/api/wireguard/list
404  /panel/api/wireguards/list
404  /panel/api/wg/list
404  /panel/wireguard/list
404  /panel/api/inbounds/wireguard/list
404  /panel/wireguards
404  /panel/wireguard
404  /panel/api/wireguard
404  /panel/wg
```

Plus the Xray config dump: zero `wireguard` substrings anywhere.

## What's also 404 (unexpected — common 3x-ui endpoints)

This build's API token surface is much narrower than mainline 3x-ui
documents. The following are missing (likely POST-only, the token
might be scoped, or this fork stripped them):

```
404  /panel/api/inbounds/add
404  /panel/api/inbounds/del
404  /panel/api/inbounds/addClient
404  /panel/api/inbounds/updateClient
404  /panel/api/inbounds/resetClientTraffic
404  /panel/api/inbounds/onlines
404  /panel/api/inbounds/clientIps
404  /panel/api/setting/all
404  /panel/api/server/restartXrayService
```

This is **a separate concern** from WG — it suggests the dashboard's
existing XrayClient may also be incomplete against this build's
real surface. Out of scope for T0; needs its own audit if confirmed
across multiple nodes.

## Conclusion for #8

The change as scaffolded assumed 3x-ui has a WG panel. It doesn't,
at least on the canonical fork running here. Three paths forward:

**A. Drop #8 entirely.** WireGuard is not in 3x-ui's product
surface. This dashboard's mission is "central control panel for
3x-ui nodes", so out-of-scope. Mark as ❌ permanently in ROADMAP
with rationale. Re-prioritize the slot to v2 work (auto-renewal,
coupons, etc.).

**B. Re-scope #8 to "WG via Xray outbound for routing".** Xray-core
DOES support WireGuard as an outbound protocol (for hop-chaining
through a WG endpoint). The dashboard could expose admin-side WG
outbound config so traffic from a 3x-ui node routes through a
WG gateway. This is ops-facing, not customer-facing — no QR, no
peer keypairs to hand to users, no subscription format.

**C. Bring our own WG daemon outside 3x-ui.** Make the dashboard
manage `wg-quick` on each node directly via SSH or a sidecar
agent. Bypasses 3x-ui entirely for the WG protocol. Doubles the
node provisioning surface (3x-ui token + SSH keys); breaks the
"central panel for 3x-ui" framing.

**Recommendation: A.** B has a tiny addressable user base (only
ops, not customers); C breaks the framing. The current 4 Xray
protocols (VLESS/VMess/Trojan/Shadowsocks) cover the customer-
facing WireGuard alternative needs via UDP-friendly transports
(QUIC + Reality + xtls-vision give kernel-comparable latency).

## Side-finding

While probing, the live node's Xray reported a startup error:
```
"xray":{"state":"error","errorMsg":"Failed to start: ... Failed
to build REALITY config. ... empty privateKey"}
```

One of the seeded inbounds has an empty Reality private key.
Unrelated to WG but worth flagging — that inbound's subscriptions
are likely broken.
