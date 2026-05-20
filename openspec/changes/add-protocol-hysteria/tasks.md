# Tasks — add-protocol-hysteria

Smaller change than #8 — Hysteria fits the unified
`/panel/api/clients/*` flow. Major work is URI rendering + UI.

## 0. Prereq verification

- [ ] 0.1 Confirm fork's Hysteria settings shape matches T0
  sample (re-run a create-via-API + read-back; previous T0 only
  captured a UI-created inbound)
- [ ] 0.2 Confirm `auth` field of `model.Client` is the per-client
  credential location (already confirmed via source inspection;
  re-verify at PR time in case fork evolves)

## 1. Runtime client

- [ ] 1.1 Add `runtime.HysteriaSettings` + `HysteriaStreamSettings`
  Go structs matching the captured shape
- [ ] 1.2 No new endpoints — `AddInbound` / `AddClient` /
  `UpdateInbound` / `DelClient` all reusable

## 2. Provisioning

- [ ] 2.1 `ClientService.Provision` branch for hysteria:
  - Generate `auth` via `crypto/rand` (16 URL-safe chars)
  - Build `Client{Email, Auth, SubID, ...}` (NO Id field)
  - Call `XrayClient.AddClient(client, inboundIds=[id])`
- [ ] 2.2 `ExpiryJob.disableOnNode`: hysteria branch flips
  `client.Enable = false` via `UpdateClient` (same path as VLESS)
- [ ] 2.3 Tests: integration with mockPanel returning the captured
  hysteria settings shape; verify ProvisionClient lands a
  well-formed Client row with Auth populated

## 3. Subscription

- [ ] 3.1 `internal/sub/links.go` add case `"hysteria"`:
  ```go
  return fmt.Sprintf("hysteria2://%s@%s:%d/?sni=%s&alpn=h3&insecure=0#%s",
      url.QueryEscape(client.Auth),
      host, inbound.Port,
      url.QueryEscape(sni),
      url.QueryEscape(remark),
  )
  ```
- [ ] 3.2 Clash converter: emit
  `type: hysteria2` proxy with `password: <auth>`, `sni`, `alpn:
  [h3]`, `up: 0`, `down: 0`
- [ ] 3.3 sing-box converter: emit
  `type: hysteria2` outbound
- [ ] 3.4 SIP008 SKIPS hysteria; Base64 INCLUDES
- [ ] 3.5 Tests: link builder fixture; Clash YAML / sing-box JSON
  parse cleanly

## 4. Frontend

- [ ] 4.1 Inbound editor: when `protocol == 'hysteria'`:
  - Hide transport tabs (TLS mandatory, network locked to
    'hysteria')
  - Show TLS cert path inputs with "node-local path" hint
  - Show `udpIdleTimeout` advanced field
- [ ] 4.2 Inbound list: hysteria appears alongside other protocols;
  client management UI works unchanged (same `clients[]` model)
- [ ] 4.3 Portal subscription page: no new UI; hysteria entries
  appear in existing URI bundle / Clash / sing-box outputs
- [ ] 4.4 vitest smoke — hysteria inbound editor renders

## 5. Capability gating

- [ ] 5.1 Reuse `nodes.supported_protocols` from #8 — hide
  hysteria editor on nodes that don't support it

## 6. Spec promotion + ROADMAP

- [ ] 6.1 Fold spec deltas into canonical specs
- [ ] 6.2 ROADMAP: 多协议 5/5 → 6/6; this change ✅
