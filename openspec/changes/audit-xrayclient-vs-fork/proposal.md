# audit-xrayclient-vs-fork

## Why

Side-finding from #8 T0 (`changes/add-protocol-wireguard/notes/3xui-wg-api.md`):
the deployed 3x-ui fork's route surface differs from what the
dashboard's existing `internal/runtime` XrayClient assumes. We
verified WireGuard support is real on this fork (good news for
#8), but we did NOT verify that the rest of XrayClient's
endpoints match the fork's actual paths.

Concrete evidence:

| Endpoint XrayClient probably uses | Status on the live fork |
|---|---|
| `POST /panel/api/inbounds/add` | ✅ 200 |
| `POST /panel/api/inbounds/del/:id` | ✅ 200 |
| `POST /panel/api/inbounds/update/:id` | ✅ 200 (discovered in T0) |
| `POST /panel/api/inbounds/addClient` | ❌ 404 — **moved to `/panel/api/clients/add`** |
| `POST /panel/api/inbounds/updateClient` | ❌ 404 — **moved to `/panel/api/clients/update/:email`** |
| `POST /panel/api/inbounds/resetClientTraffic` | ❌ 404 — **moved to `/panel/api/clients/resetTraffic/:email`** |

The dashboard ships today with these wrong paths. The bug
manifests as: e2e tests pass against `mock_panel_test.go` (which
implements whatever the test author thought 3x-ui's API was), but
real-fork deploys silently fail on every client mutation —
provisioning would "succeed" from the dashboard's perspective
because the call returns 404, the dashboard logs an error but
maybe still inserts the local DB row, then the customer's
subscription has no working client on the node.

This change audits the runtime client against the source-of-truth
fork (https://github.com/MHSanaei/3x-ui/tree/bash) and aligns
every endpoint path.

## What Changes

### Modified capability: `runtime-3xui-client`

Endpoint paths get realigned to match the fork's actual routes:

| Old path | New path |
|---|---|
| `POST /panel/api/inbounds/addClient` | `POST /panel/api/clients/add` (body: `{client, inboundIds}`) |
| `POST /panel/api/inbounds/updateClient/:email` | `POST /panel/api/clients/update/:email` |
| `POST /panel/api/inbounds/delClient/:email` | `POST /panel/api/clients/del/:email` |
| `POST /panel/api/inbounds/resetClientTraffic/:email` | `POST /panel/api/clients/resetTraffic/:email` |
| `GET  /panel/api/inbounds/getClientTrafficsById/:email` | `GET  /panel/api/clients/traffic/:email` |
| `POST /panel/api/inbounds/clientIps/:email` | `POST /panel/api/clients/ips/:email` |
| `POST /panel/api/inbounds/clearClientIps/:email` | `POST /panel/api/clients/clearIps/:email` |
| `POST /panel/api/inbounds/onlines` | `POST /panel/api/clients/onlines` |

Add the missing endpoints that became available:

- `POST /panel/api/clients/:email/attach` — attach existing client to additional inbounds (new capability)
- `POST /panel/api/clients/:email/detach` — detach client from specific inbound
- `GET  /panel/api/clients/subLinks/:subId` — node-rendered per-user sub link list
- `GET  /panel/api/clients/links/:email` — same, by email

### Modified capability: tests

`internal/e2e/mock_panel_test.go` needs to mirror the fork's route
shape, not the assumed-canonical shape. Otherwise e2e tests give a
false sense of correctness.

## Out of scope

- Adding new dashboard features that leverage the newly-discovered
  endpoints (attach/detach, subLinks). Pure path-fix change.
- Migration of existing customer-data — endpoint paths change but
  on-disk data shape is unchanged.
- The WG-specific RMW pattern (covered separately in
  `add-protocol-wireguard`).

## Assumptions

- The fork at https://github.com/MHSanaei/3x-ui/tree/bash is the
  source of truth for what's running on this dashboard's target
  nodes. If different operators run different forks, capability
  detection (from #8) determines which endpoints to use.
- The fork doesn't have version-skew protection (i.e. the routes
  in HEAD of `bash` are the routes running today). If they
  rename paths, we re-audit.
