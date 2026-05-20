# Design — add-subscription-converter

## Context

Today's `internal/sub.Assembler` resolves a `subId` → `[]Link` and
exposes `FormatBase64` / `FormatJSON` / `UserInfoHeader`. The
`Link.URL` field already contains the canonical scheme URL
(`vless://…`, `vmess://…`, etc.) built by `internal/sub/links.go`.

This change adds three more output formats by extending the same
pipeline: starting from the same `[]Link`, each new format method
walks the list, renders a per-protocol Clash/sing-box/SIP008 node,
and wraps the result in a YAML/JSON document using a template.

No new package abstractions, no new IR, no new endpoints. Just three
more `Format*` methods + a small template engine + one HTTP-layer
detector.

## Goals / Non-Goals

**Goals**

- Land three new formats inside the existing `internal/sub` package
  with the same `(*Assembler, *SubscriptionData) → ([]byte, error)`
  signature shape as the existing `FormatJSON`.
- Ship loyalsoldier-style default templates (Mihomo Clash + sing-box)
  as embedded strings; admins override via four `settings` keys.
- Add User-Agent format auto-detection at the HTTP handler so common
  clients get the right format without `?format=`.
- Preserve every existing behavior of the `subscription` module.

**Non-goals**

- A general URL → Clash converter (the user-stated need is "our
  auto-generated links → Clash"; pasting arbitrary URLs is not in
  scope).
- A new format-neutral Node IR (we already have `runtime.Inbound +
  runtime.Client`, used as input by `links.go` today; new format
  methods consume the same structures).
- WireGuard / Hysteria2 / TUIC node-side support.
- Per-user template overrides (one global template per format in v1).
- Caching of rendered output. Subscription is fetched 1×/day per
  client; rendering is cheap; revisit only if profiling complains.

## Architecture

```
                                  resolved by existing code
                              ┌─────────────────────────────┐
GET /api/public/sub/:subId    │ subscription                │
       │                      │   Assembler.Build(subId)    │
       │                      │     → SubscriptionData{     │
       │                      │         Links: []Link, …    │
       │                      │       }                     │
       │                      └────────┬────────────────────┘
       │                               │
       │       handler/public          ▼
       │      ┌────────────────────────────────────┐
       │      │ detectFormat(qs.format, UA)        │  ← NEW: at handler
       │      │   "clash"    → FormatClash         │
       │      │   "singbox"  → FormatSingBox       │
       │      │   "sip008"   → FormatSIP008        │
       │      │   "base64"   → FormatBase64        │ (existing)
       │      │   "json"     → FormatJSON          │ (existing)
       │      └────────────────────────────────────┘
       │                               │
       ▼                               ▼
                            ┌──────────────────────────────────────┐
                            │ Assembler.FormatClash    [NEW]       │
                            │   ├─ for each Link → clashNode(...)  │
                            │   │    (one helper per protocol,     │
                            │   │     in internal/sub/clash.go)    │
                            │   └─ template.RenderClash(nodes)     │
                            │                                      │
                            │ Assembler.FormatSingBox  [NEW]       │
                            │   └─ singboxOutbound(...) +          │
                            │      template.RenderSingBox(...)     │
                            │                                      │
                            │ Assembler.FormatSIP008   [NEW]       │
                            │   └─ ss-only filter + SIP008 envelope│
                            └──────────────────────────────────────┘
```

## Files touched / added

```
internal/sub/
  assembler.go        -- ADD FormatClash, FormatSingBox, FormatSIP008
  clash.go            -- NEW: per-protocol → clash node helpers
  singbox.go          -- NEW: per-protocol → sing-box outbound helpers
  sip008.go           -- NEW: ss-only SIP008 envelope
  template/
    template.go       -- NEW: RenderClash, RenderSingBox + ${proxies} substitution
    defaults.go       -- NEW: embedded default clash + sing-box templates

internal/handler/public/
  sub.go              -- ADD detectFormat + dispatch on format
                         (existing handler keeps ?format= support)

internal/handler/admin/
  setting.go          -- ADD 4 known keys to allowlist:
                         clash_template_yaml, singbox_template_json,
                         proxy_group_strategy, rule_providers_enabled
```

No new top-level packages (`internal/sub/convert/` from the earlier
draft is dropped). No new database tables.

## Per-format details

### Clash full template

The rendered YAML has this structure (excerpt; full template lives in
`template/defaults.go`):

```yaml
mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
ipv6: false

dns:
  enable: true
  nameserver:
    - https://1.1.1.1/dns-query
    - https://dns.google/dns-query
  fallback:
    - https://dns.cloudflare.com/dns-query

proxies:
  ${proxies}           # ← substituted from []clashNode

proxy-groups:
  - name: 节点选择
    type: select
    proxies: [自动选择, DIRECT, ${proxy_names}]
  - name: 自动选择
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies: [${proxy_names}]

rule-providers:
  reject:
    type: http
    behavior: domain
    url: https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/reject.txt
    interval: 86400
  # … (icloud, apple, google, proxy, direct, private, gfw, greatfire,
  #     tld-not-cn, telegrambot, lancidr, cncidr, applications)

rules:
  - DOMAIN-SUFFIX,local,DIRECT
  - RULE-SET,private,DIRECT
  - RULE-SET,reject,REJECT
  - RULE-SET,icloud,DIRECT
  - RULE-SET,apple,DIRECT
  - RULE-SET,google,节点选择
  - RULE-SET,proxy,节点选择
  - RULE-SET,direct,DIRECT
  - GEOIP,CN,DIRECT
  - MATCH,节点选择
```

Per-protocol node mapping (`internal/sub/clash.go`), starting from the
existing `runtime.Inbound + runtime.Client`:

| Source | → Clash proxy fields |
|---|---|
| VLESS (network=tcp, security=reality) | `type: vless`, `network: tcp`, `tls: true`, `reality-opts: {…}`, `client-fingerprint: chrome`, `flow: xtls-rprx-vision` |
| VLESS (network=ws) | `type: vless`, `network: ws`, `ws-opts: {path, headers.Host}` |
| VLESS (network=grpc) | `type: vless`, `network: grpc`, `grpc-opts: {grpc-service-name}` |
| VMess | `type: vmess`, `alterId`, `cipher: auto`, transport-specific opts as above |
| Trojan | `type: trojan`, `password`, `sni`, `skip-cert-verify: false`, transport opts |
| Shadowsocks | `type: ss`, `cipher: <method>`, `password` |

All proxies inherit `udp: true` (Mihomo default semantics) unless the
inbound disables it.

### Sing-box JSON

Same starting point, different shape (sing-box uses `outbounds[]` not
`proxies:`). The template ships a urltest selector group and basic
GeoIP-CN rule:

```json
{
  "log": {"level": "info"},
  "dns": {"servers": [{"address": "https://1.1.1.1/dns-query"}]},
  "outbounds": [
    {"type": "selector", "tag": "select", "outbounds": ["auto", ...names]},
    {"type": "urltest",  "tag": "auto",   "outbounds": [...names], "url": "...", "interval": "5m"},
    ${proxies},
    {"type": "direct", "tag": "direct"},
    {"type": "block",  "tag": "block"}
  ],
  "route": {
    "rule_set": [{"tag": "geoip-cn", "type": "remote", "format": "binary",
                  "url": "https://.../geoip-cn.srs"}],
    "rules": [
      {"rule_set": ["geoip-cn"], "outbound": "direct"},
      {"outbound": "select"}
    ]
  }
}
```

### SIP008

Shadowsocks-only. We filter `[]Link` to `Protocol == "shadowsocks"`,
then emit:

```json
{
  "version": 1,
  "username": "<sub_id>",
  "servers": [
    {"id": "<uuid>", "remarks": "<remark>", "server": "<host>",
     "server_port": <port>, "password": "<pw>", "method": "<cipher>"}
  ]
}
```

If the user has no Shadowsocks clients, SIP008 returns
`{"version": 1, "servers": []}` and HTTP 200 — clients that fetch
this format know how to handle empty servers.

## Template engine

`internal/sub/template/template.go`:

```go
// RenderClash substitutes ${proxies} and ${proxy_names} in tmpl
// (YAML text) with the supplied nodes. tmpl can be either the
// embedded default or an admin-supplied override fetched from
// the settings table.
func RenderClash(nodes []ClashNode, tmpl string) ([]byte, error)

func RenderSingBox(outbounds []map[string]any, tmpl string) ([]byte, error)
```

We use raw string substitution, not a YAML/JSON parser-and-recompose,
because:
- Templates are operator-controlled — they're free to put non-YAML
  text in (comments, fenced markers) and we shouldn't reformat it.
- `${proxies}` substitution into a `proxies:` slot is straightforward
  text replacement at known indentation.
- Defaults are pinned, so we control the indent. Operator overrides
  are validated by attempting a YAML/JSON parse on the rendered
  result; failure surfaces as a 500 with the parse error.

## Settings keys (admin-editable)

Added to the known-key list in `handler/admin/setting.go`:

| Key | Type | Default | Effect |
|---|---|---|---|
| `clash_template_yaml` | string | embedded default | Override entire Clash template. Must contain `${proxies}` and `${proxy_names}`. |
| `singbox_template_json` | string | embedded default | Override sing-box template. Must contain `${proxies}` and `${proxy_names}`. |
| `proxy_group_strategy` | string | `"auto+select"` | One of: `auto-only`, `select-only`, `auto+select`. Affects only the default templates' group section. Operator overrides bypass this. |
| `rule_providers_enabled` | bool | `true` | When false, default templates strip the `rule-providers` + `rules` sections and emit a no-rule config (just `proxies` + a default-direct group). |

When an operator sets `clash_template_yaml` to a non-empty string, the
`proxy_group_strategy` and `rule_providers_enabled` knobs are ignored
— the operator's template is authoritative.

## Handler dispatch

`internal/handler/public/sub.go::Sub` (existing handler, currently
dispatches base64/json):

```go
func detectFormat(qs string, ua string) Format {
    if qs != "" { return Format(qs) }
    l := strings.ToLower(ua)
    switch {
    case strings.Contains(l, "clash"),
         strings.Contains(l, "mihomo"),
         strings.Contains(l, "stash"):
        return FormatClash
    case strings.Contains(l, "sing-box"),
         strings.Contains(l, "singbox"):
        return FormatSingBox
    case strings.Contains(l, "shadowsocks"):
        return FormatSIP008
    default:
        return FormatBase64
    }
}
```

`?format=` always wins; UA is only the fallback. Existing clients
(V2RayN with no `format=`) get base64 unchanged.

Content-Type headers:

| Format | Content-Type |
|---|---|
| base64 | `text/plain; charset=utf-8` |
| json   | `application/json` |
| clash  | `text/yaml; charset=utf-8` |
| singbox| `application/json` |
| sip008 | `application/json` |

## Risks

| Risk | Mitigation |
|---|---|
| Operator overrides Clash template into invalid YAML. | Render → parse-check via `yaml.Unmarshal` → if fails, fall back to embedded default + log ERROR; never serve broken YAML. |
| loyalsoldier ruleset URL drifts / goes offline. | Operator can override `clash_template_yaml`. Default uses HTTPS GitHub raw URLs — when GitHub is offline so is most of the ecosystem. Acceptable. |
| Reality / XTLS-Vision field mapping diverges across Mihomo / Clash Verge versions. | Target Mihomo current stable; document supported Mihomo minimum in `defaults.go` header. Operator can override per-version if needed. |
| Subscription rendering slow with many clients. | Each format is O(N) string assembly; N ≤ dozens typically. No DB calls inside render. Measure before optimizing. |
| Existing flat `proxies:` consumers (none — we never shipped it) break on upgrade. | N/A — `FormatClash` does not exist today, so nothing breaks. |

## Test plan

Unit:

- `internal/sub/clash_test.go` — table-driven per-protocol mapping: a
  `runtime.Inbound + runtime.Client` fixture per (protocol, transport,
  security) combination, assert the produced clash node struct
  matches expected shape.
- `internal/sub/singbox_test.go` — same matrix for sing-box outbound
  shape.
- `internal/sub/sip008_test.go` — `[]Link` with mixed protocols →
  filter result has only SS clients; `[]Link` with no SS → empty
  `servers`.
- `internal/sub/template/template_test.go` — `RenderClash` round-trip:
  embedded default + 3-node list → output parses with `yaml.Unmarshal`;
  override with invalid YAML returns parse error.
- `internal/handler/public/sub_test.go` — `detectFormat` UA matrix:
  Mihomo / Clash Verge / V2RayN / curl / sing-box / Shadowrocket.

Integration:

- Smoke test against the demo dashboard: provision a portal user
  with mixed-protocol clients, hit `/api/public/sub/:subId?format=clash`
  and `?format=singbox`, parse the response with the official
  Mihomo / sing-box validation tools if available in CI.

Manual e2e:

- Drop the rendered YAML into Clash Verge on macOS; verify rules +
  groups render and traffic flows through `自动选择`.
- Drop the rendered JSON into sing-box mobile app; verify same.

## Out of scope (re-emphasized from proposal)

- No `internal/sub/convert/` package.
- No URL → IR parsers.
- No paste-to-convert endpoint.
- No node-side protocol additions.
