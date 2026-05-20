# add-subscription-converter

## Why

This dashboard targets sspanel-uim parity in product surface (the 5
pillars in `openspec/ROADMAP.md`) on top of a **stock 3x-ui** node
fleet. Node-side protocol scope is therefore bound by 3x-ui /
Xray-core (VLESS, VMess, Trojan, Shadowsocks today; WireGuard noted
in `add-protocol-wireguard` separately). What this change addresses
is the **订阅分发格式** side of the 多协议 pillar:

Today's `internal/sub.Assembler` exposes two output formats:

- `FormatBase64(d)` — newline-joined link bundle, base64 — works in
  V2RayN, Shadowrocket.
- `FormatJSON(d)` — Xray JSON config — works in `xray run -c`.

There is **no `FormatClash` method at all** (despite the package
header comment claiming there is). That means anyone on Mihomo /
Clash Verge / Clash for Windows / ClashX has to drop our link bundle
into an external converter before they can use the subscription. The
same gap exists for sing-box and the Shadowsocks-native SIP008.

Reference [siiway/urlclash-converter](https://github.com/siiway/urlclash-converter)
shows the conversion logic for protocol → Clash node + the wrapper
shape (rule-providers, proxy-groups, DNS). We're porting that wrapper
shape, not the URL parsers — our subscription starts from
`runtime.Inbound + runtime.Client` rows in our DB, so URL→node parsing
is unnecessary.

## What Changes

### Modified capability: `subscription`

The existing module gains three new output formats, an admin template
hook, and User-Agent-based format auto-selection at the HTTP handler.

- **`Assembler.FormatClash(d, opts)`** — emits a **complete** Mihomo
  YAML config: `proxies` (one per Link), `proxy-groups`
  (auto-select + select group by default, configurable),
  `rule-providers` (loyalsoldier ruleset URLs by default), `rules`,
  and a `dns` block. Ready to drop into a Clash app without further
  edits.
- **`Assembler.FormatSingBox(d, opts)`** — emits a sing-box JSON
  config: `outbounds[]` (one per Link), `route.rule_set[]` +
  `route.rules[]`, `dns`.
- **`Assembler.FormatSIP008(d)`** — emits a SIP008 JSON document
  with the Shadowsocks-only subset of the user's clients (other
  protocols are dropped since SIP008 is an SS-specific format).
- **Template engine** (`internal/sub/template`) — minimal package
  exposing `RenderClash(nodes, tmpl)` and `RenderSingBox(nodes, tmpl)`.
  Ships embedded defaults (one for Clash, one for sing-box) modeled on
  the loyalsoldier ruleset; admins override via four new settings
  keys (`clash_template_yaml`, `singbox_template_json`,
  `proxy_group_strategy`, `rule_providers_enabled`).
- **Handler-level User-Agent detection** — `GET
  /api/public/sub/:subId` without an explicit `?format=` honors the
  request's `User-Agent`: `clash` / `mihomo` / `stash` → clash;
  `sing-box` / `singbox` → singbox; `shadowsocks` → sip008; anything
  else → base64 (the current default).

### Behavior NOT changing

- The subscription URL surface (`/api/public/sub/:subId`).
- Tokenized URLs, the `Subscription-Userinfo` header, the
  per-client/per-user mapping.
- `FormatBase64` / `FormatJSON` output — unchanged.
- The 4 supported node protocols (VLESS/VMess/Trojan/Shadowsocks).
- The internal node IR — there is none. We use `runtime.Inbound +
  runtime.Client` directly as the input to the new format methods,
  the same way `links.go` already does.

### Explicitly NOT in this change

These were in an earlier draft but cut after a scope review:

- A new `internal/sub/convert/` package with a format-neutral `Node`
  IR — duplicates what `runtime.Inbound + runtime.Client` already
  represent for us.
- URL parsers (`vless://…` → IR) — only needed if we accepted
  arbitrary pasted URLs as input, which we don't.
- `POST /api/user/converter/clash` paste-to-convert endpoint — the
  user-stated requirement is "convert our auto-generated link config
  to Clash", not "build a general URL → Clash converter".
- Node-side WireGuard / Hysteria2 / TUIC — 3x-ui doesn't serve them
  (Hy2/TUIC are not Xray-core protocols; WireGuard is 3x-ui-native
  but out of scope here — see `add-protocol-wireguard`).

## Capabilities

### Modified Capabilities

- `subscription`: three new output formats (Clash full / sing-box /
  SIP008), an admin-editable template engine, and HTTP-layer
  User-Agent format detection.
