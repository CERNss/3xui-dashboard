# 3x-ui protocol API surface — T0 capture (REVISED)

**Date probed**: 2026-05-20
**Node**: `node-1.bwg.us.tcg12345.win:10138` (live production node)
**Xray-core version**: `v26.4.25` (also has `v26.5.9` available)
**3x-ui fork**: **non-canonical** — a heavily-extended fork that
supports 10 inbound protocols, including WireGuard, Hysteria,
mixed (SOCKS+HTTP), and routing primitives (tun, tunnel). Stock
3x-ui covers only 4.

## TL;DR — T0 conclusion REVERSED

The first-pass T0 said "drop #8 — stock 3x-ui has no WG". That
was correct for the canonical fork but **wrong for this user's
deployment**. The actual fork running here supports WG natively
via the same `/panel/api/inbounds/*` endpoint set used by Xray
protocols, with `protocol="wireguard"` + a protocol-specific
`settings` JSON shape.

There is **no separate WG API surface**. The original design.md
assumption of `runtime.WGClient` as a distinct interface is wrong.

## Protocol enumeration (from the "添加入站" UI dropdown)

```
vmess, vless, trojan, shadowsocks, wireguard, hysteria, mixed,
http, tunnel, tun
```

Classified:

| Protocol | Customer-facing? | client/peer shape | streamSettings? | In-scope for v1? |
|---|---|---|---|---|
| vless | ✓ | `clients[]` UUID | ✓ | ✅ existing |
| vmess | ✓ | `clients[]` UUID + alterId | ✓ | ✅ existing |
| trojan | ✓ | `clients[]` password | ✓ | ✅ existing |
| shadowsocks | ✓ | `clients[]` password (2022 ciphers) | ✓ | ✅ existing |
| **wireguard** | ✓ | `peers[]` keypair | ✗ (empty) | 🆕 #8 |
| **hysteria** | ✓ | `clients[].auth` + mandatory TLS | ✓ network=hysteria | 🆕 #10 (new change) |
| mixed | ops only | `accounts[]` user/pass | ✗ | ❌ out of scope |
| http | ops only | `accounts[]` user/pass | ✗ | ❌ out of scope |
| tun | infra | name/mtu/gateway/dns | ✗ | ❌ out of scope |
| tunnel | infra | portMap/allowedNetwork | ✗ | ❌ out of scope |

## Confirmed working endpoints (Bearer token)

| Verb | Path | Notes |
|---|---|---|
| GET  | `/panel/api/inbounds/list` | full inbound + clients/peers, `{success,msg,obj}` envelope |
| POST | `/panel/api/inbounds/add` | form-encoded; body keys = top-level inbound fields; settings + streamSettings + sniffing are JSON strings (NOT nested objects) |
| POST | `/panel/api/inbounds/del/<id>` | idempotent delete |
| GET  | `/panel/api/server/status` | CPU/mem/disk + xray.state/version |
| GET  | `/panel/api/server/getXrayVersion` | available + current Xray versions |
| GET  | `/panel/api/server/getConfigJson` | full Xray config JSON |
| GET  | `/panel/api/server/getDb` | sqlite DB dump |
| GET  | `/panel/api/server/getNewX25519Cert` | Reality keypair generator |

The token surface is narrower than I initially feared — earlier
404s on `/inbounds/addClient` `/inbounds/updateClient` etc. were
GET probes; those routes ARE POST-only. The fork uses a different
URL convention than canonical 3x-ui docs.

## Captured schemas

### wireguard (id=6, port=57149)

```json
"settings": {
  "mtu": 1420,
  "secretKey": "GD0qayFUvk3QqW7BIPI8HD99meNHn0+C5dGoAiUNtGs=",
  "peers": [
    {
      "privateKey": "YGNvWBWuN7WfS83mRLdfB1OV8LngwBolDCTIHV7E73o=",
      "publicKey":  "gF26nH2j8tztCY2ru1trmfa2d/JM0IBy2T5NHoJjrxk=",
      "allowedIPs": ["10.0.0.2/32"],
      "keepAlive":  0
    }
  ],
  "noKernelTun": false
}
"streamSettings": ""   // empty — WG is its own transport
```

**Key finding**: node generates BOTH the server `secretKey` AND
each peer's `privateKey`. The peer's private key is stored
server-side. This contradicts the original design.md goal of
"private key never traverses 3x-ui's API". Revised approach:
read the privateKey out via `inbounds/list`, AES-encrypt it
immediately at dashboard layer, render only at subscription
fetch time.

### hysteria (id=10, port=34587)

```json
"settings": {
  "clients": [
    {
      "security": "",
      "auth": "V5iNls6qQa",          // ← Hysteria's "password" field
      "email": "q1h291un",
      "limitIp": 0,
      "totalGB": 0,
      "expiryTime": 0,
      "enable": true,
      "tgId": 0,
      "subId": "aynamllv56osc0ox",
      "comment": "",
      "reset": 0,
      "created_at": 1779276720000,
      "updated_at": 1779276720000
    }
  ],
  "version": 2
}
"streamSettings": {
  "network": "hysteria",            // ← protocol-specific value
  "security": "tls",                // ← TLS mandatory (no transport choice)
  "tlsSettings": {
    "serverName": "",
    "minVersion": "1.2",
    "maxVersion": "1.3",
    "alpn": ["h3"],
    "certificates": [...],
    "settings": {"fingerprint": "chrome", "echConfigList": ""}
  },
  "hysteriaSettings": {
    "version": 2,
    "auth": "",
    "udpIdleTimeout": 60
  }
}
```

**Key finding**: Hysteria fits the existing `clients[]` model
that VLESS/VMess/Trojan/SS use — the auth field is named differently
(`auth` not `password` not `id`) but the lifecycle is the same. URI
scheme is `hysteria2://`.

### mixed (id=11, port=39337)

```json
"settings": {
  "auth": "password",
  "accounts": [{"user":"VhiqJycb24","pass":"MBA6uOeAvs"}],
  "udp": false,
  "ip": "127.0.0.1"
}
```

SOCKS5+HTTP combo proxy. Not customer-facing — admin tool use
(curl/wget proxy, dev tunnels). Out of v1 scope.

### http (id=13, port=36314)

```json
"settings": {
  "accounts": [{"user":"Z9Ba2lOTXK","pass":"AJAHZVQkfJ"}],
  "allowTransparent": false
}
```

HTTP CONNECT proxy. Same admin-tool category as mixed. Out of
v1 scope.

### tun (id=14, port=36549)

```json
"settings": {
  "name": "xray0",
  "mtu": 1500,
  "gateway": [],
  "dns": [],
  "userLevel": 0,
  "autoSystemRoutingTable": [],
  "autoOutboundsInterface": "auto"
}
```

Routing layer — kernel tun device for redirecting host traffic
through Xray. Out of scope (no customer concept).

### tunnel (id=15, port=21070)

```json
"settings": {
  "portMap": {},
  "allowedNetwork": "tcp,udp",
  "followRedirect": false
}
```

Port forwarder — proxies a local port to a downstream service.
Out of scope.

## Implications for #8 (WireGuard)

### Original design.md decisions vs reality

| Original (design.md) | Revised (after T0) |
|---|---|
| Split `runtime.XrayClient` + `WGClient` interfaces | ❌ Don't split. Same `/inbounds/*` for everything. |
| `WGClient.AddWGPeer / RemoveWGPeer` | ❌ No such endpoints exist. peers nested in `settings.peers[]`. Add/remove = read-modify-write the whole inbound. |
| Dashboard generates curve25519 locally | ❌ Node generates. We read the peer's privateKey from the inbound list. |
| Probe checks WG capability via 404 on WG list endpoint | ❌ Probe instead checks the protocol dropdown indirectly (capability inferred from successful `/inbounds/add` with `protocol=wireguard` on probe + delete). |
| `wg_peers` table 1:1 with ownership | ✅ Keep. We need our own index since the source of truth is buried in inbound JSON. |
| Per-channel dedup keys via `notification_log` | ✅ No change (unrelated to WG mechanics). |

### Concurrency hazard (NEW)

Two admins (or two automated provisions) adding peers
simultaneously will race on the inbound's settings JSON:

```
T1: read settings, peers = [A, B]
T2: read settings, peers = [A, B]
T1: write settings, peers = [A, B, C]
T2: write settings, peers = [A, B, D]   ← C lost!
```

v1 mitigation: pg_advisory_lock per inbound_id, serialize all
peer mutations. v2: optimistic concurrency — re-read after
write, retry on peer-count mismatch. v3 ideal: ask the 3x-ui
fork to add atomic `/inbounds/<id>/peers/add` — out of our
control.

### Subscription mechanics (UNCHANGED but re-confirmed)

WG `.conf` shape stays as design.md described. The `[Interface]`
block uses the peer's privateKey we decrypt from our wg_peers
row. The `[Peer]` block uses the inbound's server publicKey
(derivable from `secretKey` via curve25519, OR we read the
server's public key from a separate field if the fork exposes
one — TODO verify next visit).

## Implications for the broader ROADMAP

The "节点 4/4 是 stock 3x-ui 天花板" claim in my last commit
is WRONG. This fork's ceiling is 6 customer-facing protocols
(adds WG + Hysteria). Should:

1. **Reopen #8** — un-drop, scope as: WG inbound + peer
   management + .conf subscription format + Clash/sing-box WG
   outbound rendering
2. **Add #10 `add-protocol-hysteria`** — separate change since
   Hysteria fits the existing `clients[]` model (smaller delta
   than WG)
3. **Bump multi-protocol pillar score** — was 85%, with WG +
   Hysteria addressable becomes ~75% (since gap is now 2/6 not
   1/4 — re-baselining)
4. **Document the FORK assumption** in operator setup: this
   dashboard targets stock 3x-ui as baseline; WG/Hysteria
   features only enabled on this specific fork. Detection
   should probe gracefully and disable affected UI on
   stock-3x-ui nodes.

## Side findings (separate concerns)

- **No `/api-docs` JSON spec endpoint accessible to Bearer
  token.** The `/panel/api-docs` SPA route exists but renders
  client-side; the swagger spec URL is in a post-login bundle
  we can't fetch without session login (which needs 2FA we
  don't have).
- **API token surface is narrow.** Several common 3x-ui write
  endpoints (`/inbounds/addClient`, `/inbounds/updateClient`,
  `/inbounds/resetClientTraffic`) return 404 — they might be
  scope-restricted on this token, OR this fork uses different
  paths. Worth a separate audit if our existing XrayClient
  needs to hit those.
- **The fork retains `{success,msg,obj}` envelope.** Good — we
  don't need to write a new response parser. Same XrayClient
  response handling works.
