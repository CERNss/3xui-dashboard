# Path-diff catalog — XrayClient vs. MHSanaei/3x-ui

Generated 2026-05-20 from:
- Source: `internal/runtime/remote.go` (HEAD)
- Fork: `MHSanaei/3x-ui` `main` branch — `web/controller/inbound.go` +
  `web/controller/client.go` route registrations.

## Ground-truth route table (fork)

### `/panel/api/inbounds/*` (registered in `inbound.go` `initRouter`)

```
GET  /list                       a.getInbounds
GET  /options                    a.getInboundOptions    (session-only — Bearer returns 404)
GET  /get/:id                    a.getInbound
GET  /:id/fallbacks              a.getFallbacks
POST /add                        a.addInbound           (form binding — c.ShouldBind)
POST /del/:id                    a.delInbound
POST /update/:id                 a.updateInbound        (form binding)
POST /setEnable/:id              a.setInboundEnable
POST /:id/resetTraffic           a.resetInboundTraffic
POST /resetAllTraffics           a.resetAllTraffics
POST /import                     a.importInbound
POST /:id/fallbacks              a.setFallbacks
```

### `/panel/api/clients/*` (registered in `client.go` `initRouter`)

```
GET  /list                       a.list
GET  /get/:email                 a.get
GET  /traffic/:email             a.getTrafficByEmail
GET  /subLinks/:subId            a.getSubLinks
GET  /links/:email               a.getClientLinks
POST /add                        a.create               (JSON: ClientCreatePayload{Client model.Client; InboundIds []int})
POST /update/:email              a.update               (JSON: model.Client)
POST /del/:email                 a.delete
POST /:email/attach              a.attach
POST /:email/detach              a.detach
POST /resetAllTraffics           a.resetAllTraffics
POST /delDepleted                a.delDepleted
POST /resetTraffic/:email        a.resetTrafficByEmail
POST /updateTraffic/:email       a.updateTrafficByEmail
POST /ips/:email                 a.getIps
POST /clearIps/:email            a.clearIps
POST /onlines                    a.onlines
POST /lastOnline                 a.lastOnline
```

## Diff vs. current XrayClient

Legend: ✅ already correct · ❌ wrong path · ⚠️ wrong body shape

| Method (`remote.go`) | Current call | Real path | Action |
|---|---|---|---|
| `Probe` L186 | `GET  /server/status` | server group — keeps | ✅ |
| `RestartXray` L199 | `POST /server/restartXrayService` | server group — keeps | ✅ |
| `ListInbounds` L210 | `GET  /inbounds/list` | `GET  /inbounds/list` | ✅ |
| `AddInbound` L250 | `POST /inbounds/add` (form) | `POST /inbounds/add` (form) | ✅ |
| `UpdateInbound` L272 | `POST /inbounds/update/:id` (form) | `POST /inbounds/update/:id` (form) | ✅ |
| `DeleteInbound` L293 | `POST /inbounds/del/:id` | `POST /inbounds/del/:id` | ✅ |
| `SetInboundEnable` L308 | `POST /inbounds/setEnable/:id` (form `enable=`) | `POST /inbounds/setEnable/:id` | ✅ |
| `AddClient` L353 | `POST /inbounds/addClient` JSON `{id:int, settings:string}` | `POST /clients/add` JSON `{client:model.Client, inboundIds:[int]}` | ❌⚠️ |
| `UpdateClient` L403 | `POST /inbounds/updateClient/:clientId` JSON `{id:int, settings:string}` | `POST /clients/update/:email` JSON `model.Client` | ❌⚠️ |
| `DeleteClientByEmail` L434 | `POST /inbounds/:id/delClientByEmail/:email` | `POST /clients/del/:email` | ❌ |
| `GetClientTraffic` L493 | `GET  /inbounds/getClientTraffics/:email` | `GET  /clients/traffic/:email` | ❌ |
| `FetchTrafficSnapshot` L518 | `POST /inbounds/onlines` | `POST /clients/onlines` | ❌ |
| `FetchTrafficSnapshot` L528 | `POST /inbounds/lastOnline` | `POST /clients/lastOnline` | ❌ |
| `ResetClientTraffic` L547 | `POST /inbounds/:id/resetClientTraffic/:email` | `POST /clients/resetTraffic/:email` | ❌ (drop `:id`) |
| `ResetInboundTraffic` L557 | `POST /inbounds/:id/resetTraffic` | `POST /inbounds/:id/resetTraffic` | ✅ |
| `ResetAllClientTraffics` L567 | `POST /inbounds/resetAllClientTraffics/:id` | ❌ **does not exist on fork** | ❌ refactor — see below |
| `ResetAllTraffics` L573 | `POST /inbounds/resetAllTraffics` (inbound counters) | `POST /inbounds/resetAllTraffics` | ✅ |

## Body-shape fixes

### `AddClient` — `POST /clients/add`

Before:
```go
body := map[string]any{"id": id, "settings": string(settings)}  // wrong shape
r.doJSON(ctx, "/inbounds/addClient", body)
```

After:
```go
type addBody struct {
    Client     Client `json:"client"`
    InboundIDs []int  `json:"inboundIds"`
}
r.doJSON(ctx, "/clients/add", addBody{Client: client, InboundIDs: []int{int(id)}})
```

### `UpdateClient` — `POST /clients/update/:email`

Before:
```go
body := map[string]any{"id": in.ID, "settings": string(mergedSettings)}
r.doJSON(ctx, "/inbounds/updateClient/"+clientID, body)
```

After:
```go
r.doJSON(ctx, "/clients/update/"+url.PathEscape(client.Email), client)
```

(Fork's `update` handler binds directly to `model.Client`. Path
param is the email, not the credential id.)

## `ResetAllClientTraffics` refactor

The fork has no per-inbound "reset every client on this inbound"
endpoint. Closest options:

1. `/clients/resetAllTraffics` — global, no inbound scoping
2. Loop: GetInbound(tag) → for each client, POST
   `/clients/resetTraffic/:email`

Caller `service/traffic.Service.ResetInbound` semantically wants
option 2 ("zeroes one inbound's counters and every client on it").
Implementation: keep `ResetAllClientTraffics(inboundTag)` signature;
internally fetch the inbound's clients and loop.

## Out of audit scope (defer)

Routes the dashboard does NOT call today but could leverage later:

- `GET  /inbounds/get/:id` — single-inbound fetch (cheaper than `/list`)
- `POST /clients/:email/attach` + `/detach` — multi-inbound clients
- `GET  /clients/subLinks/:subId` — node-side sub render (fallback for our own)
- `POST /clients/delDepleted` — batch cleanup of expired/exceeded clients
- `POST /clients/updateTraffic/:email` — set traffic counter (vs. reset)
- `POST /clients/ips/:email` + `/clearIps/:email` — multi-IP enforcement
- `POST /inbounds/import` — backup/restore
- `POST /inbounds/:id/fallbacks` — fallback chain editor

These are real endpoints; we just don't have callers. Adding
typed wrappers is out of scope for #11 (path-fix only).
