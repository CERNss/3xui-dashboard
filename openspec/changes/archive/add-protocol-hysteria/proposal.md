# add-protocol-hysteria

## Why

Surfaced during the #8 add-protocol-wireguard T0 probe: the
deployed 3x-ui fork supports 10 inbound protocols, and **Hysteria
(v2)** is one of them. Unlike WG, Hysteria fits the existing
`clients[]` model cleanly — same fields as VLESS but using `auth`
instead of `id` for the per-client credential. URI scheme
`hysteria2://` is widely supported by Mihomo, sing-box, NekoBox,
Karing, and Stash.

This is a much smaller change than #8 because the data model
already accommodates it — `model.Client.Auth` field exists in the
fork's source, our existing `XrayClient.AddClient` flow Just
Works once we wire the protocol + URI builder.

Captured Hysteria settings from T0 (`changes/add-protocol-wireguard/notes/3xui-wg-api.md`):

```json
"settings": {
  "clients": [{ "auth": "...", "email": "...", ...standard fields }],
  "version": 2
}
"streamSettings": {
  "network": "hysteria",
  "security": "tls",                  // mandatory
  "tlsSettings": { "alpn": ["h3"], ... },
  "hysteriaSettings": { "version": 2, "udpIdleTimeout": 60 }
}
```

## What Changes

### Modified capability: `inbound-management`

The inbound editor recognizes `protocol="hysteria"`:

- TLS is mandatory (no transport choice) — editor hides the
  transport tabs, exposes TLS-specific fields (ALPN, fingerprint,
  cert source)
- `hysteriaSettings.udpIdleTimeout` exposed as an advanced field
  (default 60s — matches Hysteria 2's recommended)

### Modified capability: `client-provisioning`

`ClientService.Provision` for a Hysteria inbound:
- Generates a random `auth` string (16 chars from a URL-safe
  alphabet — `crypto/rand` reader, NOT `math/rand`)
- Sets the same `Client{Email, SubID, ExpiryTime, LimitIP, ...}`
  shape as VLESS/VMess/Trojan/SS — only the credential field
  name differs (`auth` not `id`/`password`)
- Calls `POST /panel/api/clients/add` with body
  `{client: {...auth: "..."}, inboundIds: [hysteria_inbound_id]}`
  — same code path as the existing 4 protocols

### Modified capability: `subscription`

URI builder gains a `hysteria` case:

```
hysteria2://<auth>@<node.host>:<port>/?sni=<sni>&alpn=h3&insecure=0#<remark>
```

Mihomo/sing-box accept this format directly. Clash output gains a
`type: hysteria2` proxy entry per Hysteria inbound. sing-box
output gains a `type: hysteria2` outbound entry.

SIP008 SKIPS Hysteria (no representation in the Shadowsocks-only
format). Base64 bundle INCLUDES Hysteria URIs (mixed with VLESS
etc.).

## Out of scope

- **Hysteria v1** — the fork defaults to v2; we don't probe v1.
  If an operator's `hysteriaSettings.version=1` we surface a
  warning and skip subscription rendering for that inbound.
- **Salamander obfuscation** — Hysteria 2's optional UDP-noise
  obfs layer. Adds a settings field; defer to a follow-up.
- **Per-peer bandwidth limit** (Hysteria's `up`/`down` Mbps
  fields) — defer; expose as advanced inbound field in v2.

## Assumptions

- ✅ **Verified 2026-05-20**: API-create a Hysteria inbound via
  `POST /panel/api/inbounds/add` with the dashboard-constructed
  payload (Bearer token, id=18 on the probe node), readback via
  `/inbounds/list` returns identical JSON. The
  `{network:"hysteria", security:"tls", hysteriaSettings:
  {version:2, udpIdleTimeout:60}}` shape from T0 IS the expected
  API shape.
- The fork registers Hysteria with `model.Client.Auth` field for
  per-client credentials. Confirmed via source inspection
  (`web/service/client.go` in MHSanaei/3x-ui).
- TLS certificates: the inbound stores `certificateFile`/`keyFile`
  paths on the node's filesystem. The dashboard CANNOT upload
  certs via API — operator manages certs externally
  (e.g. acme.sh on the node). Our editor exposes the path
  fields but warns "node-local file path required" instead of
  offering an upload widget.
