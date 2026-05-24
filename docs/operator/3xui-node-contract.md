# 3x-ui node contract

This dashboard targets **MHSanaei/3x-ui** specifically — the
extended fork at https://github.com/MHSanaei/3x-ui that ships
WireGuard + Hysteria protocol support in addition to the four
Xray protocols (VLESS / VMess / Trojan / Shadowsocks).

## Why this matters

The fork moved per-client mutation routes from the `/inbounds/*`
group to a new `/clients/*` group:

| Action | Old (canonical 3x-ui) | New (MHSanaei fork) |
|---|---|---|
| Add client | `POST /panel/api/inbounds/addClient` | `POST /panel/api/clients/add` |
| Update client | `POST /panel/api/inbounds/updateClient/:uuid` | `POST /panel/api/clients/update/:email` |
| Delete client | `POST /panel/api/inbounds/:id/delClientByEmail/:email` | `POST /panel/api/clients/del/:email` |
| Reset client traffic | `POST /panel/api/inbounds/:id/resetClientTraffic/:email` | `POST /panel/api/clients/resetTraffic/:email` |
| Per-client traffic | `GET  /panel/api/inbounds/getClientTraffics/:email` | `GET  /panel/api/clients/traffic/:email` |
| Onlines / lastOnline | `POST /panel/api/inbounds/{onlines,lastOnline}` | `POST /panel/api/clients/{onlines,lastOnline}` |

The dashboard speaks the **fork** route shape only. Pointing it
at a canonical 3x-ui (no `/clients/*` group) will surface a clear
"endpoint not found" error for every client mutation — the
dashboard will NOT silently appear to succeed.

## Supported fork versions

- ✅ MHSanaei/3x-ui `main` and `bash` branches (content-identical
  at the controller layer as of 2026-05-20)
- ✅ Recent tagged releases on `main` (the `/clients/*` group is
  stable and present)
- ❌ Forks deleted the `/clients/*` group or pre-fork builds —
  unsupported

## How to confirm your node matches the contract

```sh
curl -sI \
  -H "Authorization: Bearer <API_TOKEN>" \
  -H "X-Requested-With: XMLHttpRequest" \
  "https://<node>/<basePath>/panel/api/clients/onlines" \
  -X POST
```

A `200 OK` (with the dashboard's standard envelope) confirms the
fork is current. A `404 Not Found` means the node is running an
older codebase and needs an update before this dashboard can
manage clients on it.

## Out of scope for v1

- **Multi-fork support** — feature flags or dual-path runtime
  behavior. If demand emerges, track as a separate change.
- **Capability detection** — the `/panel/api/inbounds/options`
  endpoint exists but rejects API-token (Bearer) callers; the
  dashboard cannot use it to probe a node's protocol support at
  runtime. The required node shape is declared statically.
