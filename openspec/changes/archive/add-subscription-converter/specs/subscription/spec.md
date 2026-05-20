## MODIFIED Requirements

### Requirement: Subscription Output Formats

The system SHALL serve subscription content in **five** client-compatible
formats — base64, Xray JSON, Clash YAML (full Mihomo template),
sing-box JSON, and SIP008 — selectable by `?format=` query parameter
or by User-Agent auto-detect.

#### Scenario: Base64 subscription (unchanged)

- **WHEN** a client GETs `/api/public/sub/:subId` with `?format=base64` (or with no format AND no recognizable Clash/sing-box/Shadowsocks User-Agent)
- **THEN** the system SHALL return the newline-joined link list, base64-encoded
- **AND** include the `Subscription-Userinfo` and update-interval headers
- **AND** Content-Type SHALL be `text/plain; charset=utf-8`

#### Scenario: Xray JSON subscription (unchanged)

- **WHEN** `?format=json` is requested
- **THEN** the system SHALL return an Xray-client JSON config assembled from the user's clients
- **AND** Content-Type SHALL be `application/json`

#### Scenario: Clash YAML — full Mihomo config

- **WHEN** `?format=clash` is requested, OR the User-Agent contains `clash` / `mihomo` / `stash` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a complete Mihomo-compatible YAML config
- **AND** the YAML SHALL include the `proxies` array (one entry per user link), `proxy-groups` (default: `节点选择` selector + `自动选择` url-test), `rule-providers` (loyalsoldier ruleset URLs by default), `rules` (loyalsoldier rule set ending in `MATCH,节点选择`), and a `dns` block
- **AND** the YAML SHALL be ready to use in Clash Verge / Mihomo / ClashX **without further user edits**
- **AND** Content-Type SHALL be `text/yaml; charset=utf-8`

#### Scenario: Sing-box JSON config

- **WHEN** `?format=singbox` is requested, OR the User-Agent contains `sing-box` / `singbox` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a complete sing-box JSON config
- **AND** the JSON SHALL contain `outbounds[]` (one entry per user link), `route.rule_set[]` (geoip-cn), `route.rules[]` (geoip-cn → direct, else selector), `dns`, and a urltest + selector outbound pair
- **AND** Content-Type SHALL be `application/json`

#### Scenario: SIP008 — Shadowsocks-only

- **WHEN** `?format=sip008` is requested, OR the User-Agent contains `shadowsocks` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a SIP008 v1 JSON document containing only the user's Shadowsocks clients
- **AND** non-Shadowsocks clients (VLESS / VMess / Trojan) SHALL be omitted from the `servers` array
- **AND** if the user has zero Shadowsocks clients, the response SHALL be `{"version": 1, "servers": []}` (NOT 404 / 204)
- **AND** Content-Type SHALL be `application/json`

#### Scenario: Per-protocol mapping into Clash

- **WHEN** a Link's protocol + transport + security combination is mapped to a Clash proxy entry
- **THEN** the field mapping SHALL be:
  - VLESS / Reality → `type: vless` + `tls: true` + `reality-opts: {…}` + `client-fingerprint: chrome` + `flow` when present
  - VLESS / WS → `type: vless` + `network: ws` + `ws-opts: {path, headers.Host}`
  - VLESS / gRPC → `type: vless` + `network: grpc` + `grpc-opts: {grpc-service-name}`
  - VMess → `type: vmess` + `alterId` + `cipher: auto` + transport-specific opts
  - Trojan → `type: trojan` + `password` + `sni` + `skip-cert-verify: false` + transport opts
  - Shadowsocks → `type: ss` + `cipher: <method>` + `password`
- **AND** every proxy SHALL default `udp: true` unless the source inbound disables it

### Requirement: User-Agent-Based Format Auto-Detection

The system SHALL inspect the request's User-Agent when `?format=` is
absent and select a sensible default format so common clients work
without manual configuration.

#### Scenario: Explicit ?format= always wins

- **GIVEN** the request URL is `/api/public/sub/<id>?format=clash` AND the User-Agent is `Shadowrocket/...`
- **WHEN** the handler dispatches
- **THEN** the response SHALL be Clash YAML (not SIP008)

#### Scenario: Clash family UA → clash

- **WHEN** the User-Agent contains any of: `clash`, `mihomo`, `stash` (case-insensitive) AND `?format=` is absent
- **THEN** the response SHALL be Clash YAML

#### Scenario: sing-box UA → singbox

- **WHEN** the User-Agent contains `sing-box` or `singbox` AND `?format=` is absent
- **THEN** the response SHALL be sing-box JSON

#### Scenario: Shadowsocks UA → sip008

- **WHEN** the User-Agent contains `shadowsocks` AND `?format=` is absent
- **THEN** the response SHALL be SIP008 JSON

#### Scenario: Unrecognized UA falls back to base64

- **WHEN** the User-Agent does not match any of the above (e.g. V2RayN, curl, generic browsers) AND `?format=` is absent
- **THEN** the response SHALL be base64 (preserves the historical default — existing clients are unaffected by this change)

#### Scenario: Unsupported ?format= is rejected

- **WHEN** `?format=foo` is supplied (not one of base64/json/clash/singbox/sip008)
- **THEN** the handler SHALL respond HTTP 400 with a clear error naming the supported formats

### Requirement: Admin-Editable Subscription Templates

The system SHALL expose four runtime settings keys allowing the
administrator to customize the Clash and sing-box output without
rebuilding the binary.

#### Scenario: Default templates ship embedded

- **WHEN** no settings overrides exist
- **THEN** the system SHALL use embedded default templates (Mihomo Clash + sing-box) modeled on the loyalsoldier ruleset
- **AND** these defaults SHALL be sufficient for a fresh deploy to serve usable Clash subscriptions out of the box

#### Scenario: clash_template_yaml override

- **WHEN** an admin PUTs a non-empty `clash_template_yaml` value via `/api/admin/settings/clash_template_yaml`
- **THEN** the system SHALL validate the value parses as YAML before persisting (returns HTTP 400 on parse failure with the parser error)
- **AND** subsequent `?format=clash` requests SHALL render through the operator's template
- **AND** if the rendered output (after `${proxies}` substitution) fails to parse, the system SHALL fall back to the embedded default + log ERROR — never serve broken YAML to the client

#### Scenario: singbox_template_json override

- **WHEN** an admin PUTs a non-empty `singbox_template_json` value
- **THEN** the same validate-on-PUT + render-time fallback logic SHALL apply, using `json.Unmarshal` instead of YAML

#### Scenario: proxy_group_strategy

- **WHEN** the setting `proxy_group_strategy` is one of `auto-only` / `select-only` / `auto+select`
- **THEN** the default Clash template's `proxy-groups:` block SHALL adjust accordingly:
  - `auto-only`: only the `自动选择` url-test group, all proxies in it
  - `select-only`: only the `节点选择` selector group, with `DIRECT` as the first option
  - `auto+select`: both groups (the default — `节点选择` selects between `自动选择`, `DIRECT`, and individual proxies)
- **AND** when `clash_template_yaml` is non-empty, this setting SHALL be ignored (the operator template is authoritative)

#### Scenario: rule_providers_enabled

- **WHEN** `rule_providers_enabled = false`
- **THEN** the default Clash template SHALL emit a rule-set-free config (no `rule-providers`, no `rules` beyond a single `MATCH,<group>` fallback)
- **AND** operators who run their own external rule provider can use this to get just the proxy list
- **AND** when `clash_template_yaml` is non-empty, this setting SHALL be ignored

## REMOVED Requirements

(none — this change is additive)
