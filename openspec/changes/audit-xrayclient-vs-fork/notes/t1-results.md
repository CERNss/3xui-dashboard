# T1 results — real-node validation 2026-05-21

Live probe against `node-1.bwg.us.tcg12345.win:10138`,
panel reports Xray v26.4.25 + v26.5.9, ~113 day uptime.

## Headline finding

**#11 path migration was wrong**. The `/panel/api/clients/*`
route group is **404 on this production fork**. The legacy
`/panel/api/inbounds/*` routes are intact and behave exactly
as the pre-#11 code expected.

This is a production-blocking regression that was shipped on
2026-05-20 (`d2598ec` and its #8.1 successor `587aa10`). Reverted
in this round.

## Route audit (verified by direct curl)

| Path | Status | Notes |
|---|---|---|
| `POST /panel/api/inbounds/onlines` | ✅ 200 `{success:true,obj:[]}` | Legacy lives |
| `POST /panel/api/inbounds/lastOnline` | ✅ 200 `{success:true,obj:{}}` | Legacy lives |
| `GET  /panel/api/inbounds/getClientTraffics/:email` | ✅ 200 (success:false on miss but route OK) | Legacy lives |
| `POST /panel/api/inbounds/addClient` | ✅ 200 | Body: `{id, settings:stringified-json}` |
| `POST /panel/api/inbounds/updateClient/:clientID` | ✅ 200 | Same body shape; param is UUID/password/auth |
| `POST /panel/api/inbounds/:id/delClientByEmail/:email` | ⚠️ 200 success:false when leaving 0 clients; ✅ otherwise | Quirk: fork refuses to leave clientless inbound |
| `POST /panel/api/inbounds/:id/resetClientTraffic/:email` | ✅ 200 | |
| `POST /panel/api/inbounds/resetAllClientTraffics/:id` | ✅ 200 | Per-inbound bulk reset DOES exist (I had refactored this away in #11) |
| `POST /panel/api/inbounds/resetAllTraffics` | ✅ 200 | |
| `POST /panel/api/clients/*` (any) | ❌ 404 | Entire route group absent |

## End-to-end roundtrip (all dashboard-wire-format)

Each step uses the exact payload our `internal/runtime` would
send. All succeeded:

1. **Create VLESS inbound** via form-encoded `POST /inbounds/add`
   → 200, id=19, settings echoed back unchanged
2. **AddClient** via `POST /inbounds/addClient` with body
   `{id:19, settings:"{\"clients\":[{id,email,...}]}"}` → 200
3. **List** confirms client landed with all fields preserved
4. **delClientByEmail** with 1 client → quirk surfaced
   ("no client remained in Inbound" → success:false). Re-push
   fallback path proven to handle this (see step 4b).
5. **Re-push to empty clients[]** via `POST /inbounds/update/19`
   with `settings={"clients":[],...}` → 200, accepted; our
   `rePushInboundWithMutation` fallback works correctly.
6. **Create Hysteria inbound** with `protocol=hysteria` + full
   `streamSettings.hysteriaSettings.{version:2, udpIdleTimeout:60}`
   block → 200. AddClient with `auth=t1-hys-secret-001` → 200;
   readback shows `"auth"` field preserved in JSON exactly as
   the dashboard expects. Hysteria backend is **verified
   compatible**.
7. **Create WG inbound** with empty `secretKey` → 200, but
   **node does NOT auto-generate the server keypair**. Settings
   echoed back with `secretKey:""`. Required dashboard-side fix:
   `WGProvisioner.ProvisionPeer` now lazily fills `secretKey`
   on the first peer provision (also stamps MTU=1420 if 0).
8. **UpdateInbound with filled secretKey + 1 peer** via
   `POST /inbounds/update/21` form-encoded → 200. Readback
   shows peer present with all fields preserved
   (privateKey/publicKey/allowedIPs/keepAlive). **WG RMW
   wire shape is verified end-to-end.**
9. Cleanup: deleted inbounds 19/20/21 via `POST /inbounds/del/:id` → all 200.

## Verified ground-truth schemas

### VLESS client object (json.Unmarshal target on the fork)

```json
{
  "id": "t1-test-uuid-001",
  "email": "t1-alice",
  "flow": "",
  "limitIp": 0, "totalGB": 0, "expiryTime": 0,
  "enable": true, "subId": "t1sub", "tgId": 0,
  "comment": "", "reset": 0,
  "created_at": <unix-ms>, "updated_at": <unix-ms>
}
```

### Hysteria client object

```json
{
  "auth": "t1-hys-secret-001",
  "email": "t1-bob",
  "limitIp": 0, "totalGB": 0, "expiryTime": 0,
  "enable": true, "subId": "t1hys", "tgId": 0,
  "comment": "", "reset": 0,
  "created_at": <unix-ms>, "updated_at": <unix-ms>
}
```

Same envelope as VLESS, just `auth` instead of `id`. Runtime
`Client.Auth` field added in #10 confirmed correct.

### WireGuard inbound.settings

```json
{
  "mtu": 1420,
  "secretKey": "<server-curve25519-priv-b64>",
  "peers": [
    {
      "privateKey": "<peer-priv-b64>",
      "publicKey":  "<peer-pub-b64>",
      "allowedIPs": ["10.0.0.2/32"],
      "keepAlive": 25
    }
  ],
  "noKernelTun": false
}
```

`secretKey` MUST be supplied by the caller — fork stores empty
string verbatim. The lazy-fill in `WGProvisioner.ProvisionPeer`
addresses this.

## Quirks worth knowing

1. **delClientByEmail refuses to leave inbound clientless**.
   Returns `success:false` with message `"no client remained in
   Inbound"`. The dashboard's existing `isPanelClientError`
   detects this and triggers `rePushInboundWithMutation` which
   succeeds.

2. **Node does NOT auto-generate WG server keypair**. Dashboard
   must supply `secretKey`. Implemented as a lazy fill on first
   `ProvisionPeer` call (server keypair generated via
   `wgcrypto.GenerateKeypair()`).

3. **All `/clients/*` routes absent**. The MHSanaei/3x-ui main
   HEAD source I read via WebFetch in `#11` either reflected a
   newer/unreleased version or the WebFetch summary misled. The
   deployed fork's actual routes are exclusively under
   `/inbounds/*`.

## Implication for the spec

The "Fork-Aligned Client Routes" requirement I added to
`openspec/specs/runtime-3xui-client/spec.md` in commit
`0e83586` is wrong. The CORRECT requirement is "Legacy
Inbounds-Grouped Client Routes" — those are what production
ships. Revert in the same patch.
