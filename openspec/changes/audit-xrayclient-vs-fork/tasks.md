# Tasks — audit-xrayclient-vs-fork

Path-realignment change. No new features; matching the existing
runtime client to the fork's actual route surface.

## 1. Audit

- [x] 1.1 Catalog every HTTP call XrayClient makes (grep
  `r.doGet` / `r.doForm` / `r.doJSON` / `r.doPostEmpty` in
  `internal/runtime/remote.go`)
- [x] 1.2 Cross-check against `web/controller/inbound.go` +
  `web/controller/client.go` route registrations from the fork
  source (MHSanaei/3x-ui `main` — WebFetched 2026-05-20)
- [x] 1.3 Produce a `notes/path-diff.md` table per the proposal's
  format, flagging each callsite to fix

## 2. Path fixes

- [x] 2.1 `internal/runtime/remote.go`: update each affected
  method to use the new path:
  - [x] AddClient → POST /clients/add (was /inbounds/addClient)
  - [x] UpdateClient → POST /clients/update/:email (was /inbounds/updateClient/:clientId)
  - [x] DeleteClientByEmail → POST /clients/del/:email (was /inbounds/:id/delClientByEmail/:email)
  - [x] ResetClientTraffic → POST /clients/resetTraffic/:email (was /inbounds/:id/resetClientTraffic/:email)
  - [x] GetClientTraffic → GET /clients/traffic/:email (was /inbounds/getClientTraffics/:email)
  - [x] FetchTrafficSnapshot.onlines → POST /clients/onlines (was /inbounds/onlines)
  - [x] FetchTrafficSnapshot.lastOnline → POST /clients/lastOnline (was /inbounds/lastOnline)
  - [x] ResetAllClientTraffics → refactor to loop /clients/resetTraffic/:email per client (fork has no per-inbound endpoint)
- [x] 2.2 Update body shape for `AddClient` to
  `{client: model.Client, inboundIds: [int]}` (was `{id, settings:string}`)
- [x] 2.2 Update body shape for `UpdateClient` to a raw
  `model.Client` JSON body with email as the path key (was
  `{id, settings:string}` keyed on UUID/password)
- [ ] 2.3 Add typed wrappers for the newly-discovered endpoints
  (DEFERRED — none of the dashboard's current flows need them;
  documented in `notes/path-diff.md` "Out of audit scope"):
  - AttachClient / DetachClient
  - GetSubLinks(subID)
  - DelDepleted, UpdateTraffic, GetIps/ClearIps

## 3. Mock panel realignment

- [x] 3.1 Rewrite `internal/e2e/mock_panel_test.go` route table to
  match the fork's actual paths
- [x] 3.2 Re-run e2e tests — `internal/e2e` is green after the
  rewrite, confirming the dashboard's XrayClient calls land on
  the new paths

## 4. Add coverage for the formerly-silent-failure cases

- [x] 4.1 `TestAddClient_404SurfacesPath` (in
  `internal/runtime/remote_test.go`): when the panel returns 404
  for `/clients/add`, the dashboard returns an error that names
  the actual 404'd path. No silent provision success.
- [x] 4.2 `TestAddClient_RoutesToClientsAddPath`: asserts both
  the path AND the new `{client, inboundIds}` body envelope —
  guards against future regressions to the old `{id, settings}`
  shape.

## 5. Documentation

- [x] 5.1 `docs/operator/3xui-fork-compat.md`: brief note that
  this dashboard targets the MHSanaei/3x-ui fork. Operators
  running canonical 3x-ui need to either upgrade or
  use a feature-flag (out of scope for v1).
