# Tasks — audit-xrayclient-vs-fork

Path-realignment change. No new features; matching the existing
runtime client to the fork's actual route surface.

## 1. Audit

- [ ] 1.1 Catalog every HTTP call XrayClient makes (grep
  `c.do(` / `c.post(` / `c.get(` in `internal/runtime/`)
- [ ] 1.2 Cross-check against `web/controller/inbound.go` +
  `web/controller/client.go` route registrations from the fork
  source (https://github.com/MHSanaei/3x-ui/tree/bash)
- [ ] 1.3 Produce a `notes/path-diff.md` table per the proposal's
  format, flagging each callsite to fix

## 2. Path fixes

- [ ] 2.1 `internal/runtime/remote.go`: update each affected
  method to use the new path. Methods to touch (per the
  proposal table — confirm exact names during audit):
  - AddClient → POST /clients/add
  - UpdateClient → POST /clients/update/:email
  - DelClient → POST /clients/del/:email
  - ResetClientTraffic → POST /clients/resetTraffic/:email
  - GetClientTrafficsByEmail → GET /clients/traffic/:email
  - GetClientIPs → POST /clients/ips/:email
  - ClearClientIPs → POST /clients/clearIps/:email
  - OnlineClients → POST /clients/onlines
- [ ] 2.2 Update body shape for `AddClient` (now takes a
  `{client, inboundIds}` envelope, not the per-inbound
  `clientId` path param)
- [ ] 2.3 Add typed wrappers for the newly-discovered endpoints:
  - AttachClient(email, inboundIDs)
  - DetachClient(email, inboundID)
  - GetSubLinks(subID) — returns the node's pre-rendered link
    bundle (useful as a fallback for our own sub render)

## 3. Mock panel realignment

- [ ] 3.1 Rewrite `internal/e2e/mock_panel_test.go` route table to
  match the fork's actual paths
- [ ] 3.2 Run existing e2e tests — they SHOULD still pass after
  the rename if our XrayClient was the only thing affected. If
  they break, the test code itself was calling old paths and
  needs the same fix.

## 4. Add coverage for the formerly-silent-failure cases

- [ ] 4.1 e2e test: AddClient on canonical-3x-ui path returns
  404, dashboard surfaces a clear error (no silent provision
  success)
- [ ] 4.2 e2e test: AddClient on fork path returns 200, dashboard
  reflects the new client correctly

## 5. Documentation

- [ ] 5.1 `docs/operator/3xui-fork-compat.md`: brief note that
  this dashboard targets the MHSanaei/3x-ui `bash` branch fork.
  Operators running canonical 3x-ui need to either upgrade or
  use a feature-flag (out of scope for v1).
