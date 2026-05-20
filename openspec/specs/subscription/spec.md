# subscription

Per-client link generation, multi-format subscription output (base64 /
JSON / Clash), and the tokenized public URL clients use to fetch
without dashboard auth.

## Purpose & boundaries

Adjacent modules: **`client-provisioning`** owns the underlying clients;
**`traffic-statistics`** feeds the `Subscription-Userinfo` header.

## Requirements

### Requirement: Per-Client Link Generation

The system SHALL generate Xray connection links for a client from its inbound
configuration.

#### Scenario: Generate links for a client

- **WHEN** the system builds links for a client identified by node + inbound + email
- **THEN** it produces one link per applicable protocol/stream combination
  (VLESS, VMess, Trojan, Shadowsocks) encoding address, port, identifier,
  transport, and TLS/Reality parameters

#### Scenario: Remark formatting

- **WHEN** a link is generated
- **THEN** its remark SHALL be formatted from a configurable remark model
  (inbound remark, email, and node info) so links are human-distinguishable

### Requirement: Subscription Output Formats

The system SHALL serve subscription content in seven client-compatible
formats — base64, Xray JSON, Clash YAML (full Mihomo template),
sing-box JSON, SIP008, WireGuard `.conf`, and WireGuard ZIP bundle —
selectable by `?format=` query parameter or by User-Agent auto-detect
(see Requirement: User-Agent-Based Format Auto-Detection).

#### Scenario: Base64 subscription

- **WHEN** `?format=base64` is requested (or with no format AND no recognizable Clash/sing-box/Shadowsocks User-Agent)
- **THEN** the system SHALL return the newline-joined link list, base64-encoded
- **AND** include `Subscription-Userinfo` and `Profile-Update-Interval` headers
- **AND** Content-Type SHALL be `text/plain; charset=utf-8`

#### Scenario: Xray JSON subscription

- **WHEN** `?format=json` is requested
- **THEN** the system SHALL return a JSON config assembled from the user's clients
- **AND** Content-Type SHALL be `application/json`

#### Scenario: Clash YAML — full Mihomo config

- **WHEN** `?format=clash` is requested, OR the User-Agent contains `clash` / `mihomo` / `stash` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a complete Mihomo-compatible YAML config
- **AND** the YAML SHALL include `proxies` (one entry per user link), `proxy-groups` (default: `节点选择` selector + `自动选择` url-test), `rule-providers` (loyalsoldier ruleset URLs by default), `rules` (standard set ending in `MATCH,节点选择`), and a `dns` block
- **AND** the YAML SHALL be ready to use in Clash Verge / Mihomo / ClashX without further user edits
- **AND** Content-Type SHALL be `text/yaml; charset=utf-8`

#### Scenario: Sing-box JSON config

- **WHEN** `?format=singbox` is requested, OR the User-Agent contains `sing-box` / `singbox` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a sing-box JSON config containing `outbounds[]`, `route.rule_set[]`, `route.rules[]`, `dns`, and a urltest + selector outbound pair
- **AND** Content-Type SHALL be `application/json`

#### Scenario: SIP008 — Shadowsocks-only

- **WHEN** `?format=sip008` is requested, OR the User-Agent contains `shadowsocks` and no `?format=` was supplied
- **THEN** the system SHALL respond HTTP 200 with a SIP008 v1 JSON document containing only the user's Shadowsocks clients
- **AND** non-Shadowsocks clients SHALL be omitted from the `servers` array
- **AND** if the user has zero Shadowsocks clients, the response SHALL be `{"version":1,"servers":[]}` (NOT 404)

#### Scenario: WireGuard .conf

- **WHEN** `?format=wireguard` is requested (or `/sub/wireguard/:subId`)
- **THEN** the system SHALL respond HTTP 200 with `text/plain; charset=utf-8` whose body is the wg-quick / wireguard-android compatible `[Interface]` + `[Peer]` ini text
- **AND** `Content-Disposition: attachment; filename="wireguard.conf"` SHALL be set so browsers offer save-as
- **AND** non-WG ownerships SHALL be skipped silently (Base64/Clash/sing-box still carry them)
- **AND** v1 Hysteria-style endpoints (`hysteriaSettings.version=1`) are out of scope: emitting v1 WG inbounds returns the body unchanged but documents WG inbounds remain v2-only on the dashboard

#### Scenario: WireGuard ZIP bundle

- **WHEN** `?format=wireguard-zip` is requested (or `/sub/wireguard-zip/:subId`)
- **THEN** the system SHALL respond HTTP 200 with `application/zip` whose archive contains one `.conf` per WG peer, named by sanitized remark (fallback to `wg-<pubkey-suffix>` when remark is empty/non-ASCII)
- **AND** `Content-Disposition: attachment; filename="wireguard.zip"` SHALL be set
- **AND** name collisions inside the archive SHALL be disambiguated with a numeric suffix

#### Scenario: Hysteria 2 URI in Base64 + Clash + sing-box

- **WHEN** the user has a Hysteria 2 ownership
- **THEN** Base64 + JSON outputs SHALL include `hysteria2://<auth>@<host>:<port>/?sni=<sni>&alpn=h3&insecure=0#<remark>` per [Hysteria URI Scheme](https://hysteria.network/docs/developers/URI-Scheme/)
- **AND** Clash SHALL emit a `type: hysteria2` proxy entry; sing-box SHALL emit a `type: hysteria2` outbound (tls.server_name falls back to connect host when SNI is empty)
- **AND** SIP008 SHALL omit Hysteria entries (SS-only format)

#### Scenario: Empty user — minimal fallback config

- **WHEN** a Clash or sing-box request resolves to a user with zero provisioned clients
- **THEN** the system SHALL return a minimal valid config (Clash: empty `proxies` + DIRECT-only group + MATCH rule; sing-box: direct/block/dns outbounds + final-direct route)
- **AND** SHALL NOT emit broken YAML/JSON despite the empty proxy list

#### Scenario: Per-protocol mapping into Clash

- **WHEN** a Link's protocol + transport + security combination is mapped to a Clash proxy entry
- **THEN** the field mapping SHALL be:
  - VLESS / Reality → `type: vless` + `tls: true` + `reality-opts` + `client-fingerprint: chrome` + `flow` when present
  - VLESS / WS → `type: vless` + `network: ws` + `ws-opts: {path, headers.Host}`
  - VLESS / gRPC → `type: vless` + `network: grpc` + `grpc-opts: {grpc-service-name}`
  - VMess → `type: vmess` + `alterId: 0` (modern AEAD) + `cipher: auto` + transport-specific opts
  - Trojan → `type: trojan` + `password` + `sni` + `skip-cert-verify: false` + transport opts
  - Shadowsocks → `type: ss` + `cipher` + `password`
  - WireGuard → `type: wireguard` + `private-key` + `public-key: <server>` + `ip` + `udp: true`
  - Hysteria 2 → `type: hysteria2` + `password: <auth>` + `sni` + `alpn: [h3]` + `skip-cert-verify` when `allowInsecure: true`
- **AND** every proxy SHALL default `udp: true`

### Requirement: User-Agent-Based Format Auto-Detection

The system SHALL inspect the request's User-Agent when `?format=` is
absent and select a sensible default format so common clients work
without manual configuration.

#### Scenario: Explicit ?format= always wins

- **GIVEN** the request URL is `/sub/<id>?format=clash` AND the User-Agent is `Shadowrocket/...`
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

- **WHEN** the User-Agent does not match any of the above (V2RayN, curl, generic browsers) AND `?format=` is absent
- **THEN** the response SHALL be base64

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

#### Scenario: clash_template_yaml override validation

- **WHEN** an admin PUTs a non-empty `clash_template_yaml` value
- **THEN** the value SHALL be validated by `yaml.Unmarshal` into a `map[string]any` (rejects bare scalars)
- **AND** SHALL be required to contain the `${proxies}` placeholder; missing placeholder returns HTTP 400
- **AND** if validation passes, subsequent `?format=clash` requests SHALL render through the operator's template
- **AND** if rendering fails at request time (substitution result fails parse), the system SHALL log ERROR and fall back to the embedded default — never serve broken YAML

#### Scenario: singbox_template_json override validation

- **WHEN** an admin PUTs a non-empty `singbox_template_json` value
- **THEN** the same validate-then-fallback logic applies, using `json.Unmarshal` into a `map[string]any`

#### Scenario: proxy_group_strategy

- **WHEN** the setting `proxy_group_strategy` is one of `auto-only` / `select-only` / `auto+select`
- **THEN** the default Clash template's `proxy-groups:` block adjusts accordingly:
  - `auto-only`: only `自动选择` url-test
  - `select-only`: only `节点选择` selector with DIRECT first
  - `auto+select` (default): both groups
- **AND** invalid values are rejected at PUT time with HTTP 400
- **AND** when `clash_template_yaml` is non-empty, this setting SHALL be ignored

#### Scenario: rule_providers_enabled

- **WHEN** `rule_providers_enabled = false`
- **THEN** the default Clash template SHALL emit no `rule-providers` and no `rules` (just proxies + groups + a final MATCH fallback)
- **AND** when `clash_template_yaml` is non-empty, this setting SHALL be ignored

### Requirement: Tokenized Public Subscription URL

The system SHALL serve subscriptions at a public, unguessable URL bound to a
subscription id, without requiring a dashboard login.

#### Scenario: Valid subscription id

- **WHEN** a client app fetches `/{subPath}/{subId}` for a known subscription id
- **THEN** the system returns that subscription's content in the requested format
  and HTTP 200

#### Scenario: Unknown subscription id

- **WHEN** the requested subscription id matches no client
- **THEN** the system returns HTTP 404 and SHALL NOT leak whether other ids exist

#### Scenario: Aggregated multi-client subscription

- **WHEN** a subscription id maps to several clients across nodes (one user,
  multiple nodes)
- **THEN** the subscription output SHALL include the links for all of that
  subscription id's clients

### Requirement: Subscription Info Headers

The system SHALL include usage and lifecycle metadata in subscription responses.

#### Scenario: Userinfo header

- **WHEN** a base64 subscription is served
- **THEN** the response SHALL include a `Subscription-Userinfo` header reporting
  upload, download, total, and expiry derived from the client's traffic

#### Scenario: Update interval advertised

- **WHEN** any subscription is served
- **THEN** the response SHALL advertise the configured refresh interval to client apps

### Requirement: User Portal Subscription View

End users SHALL be able to view and copy their subscription in the portal.

#### Scenario: User retrieves subscription link

- **WHEN** an authenticated `user` opens the Subscription page
- **THEN** the system returns the user's public subscription URL and the system
  renders it as copyable text and a QR code

#### Scenario: No subscription linked

- **WHEN** a `user` has no client mapped to their account
- **THEN** the portal SHALL show an empty state directing them to contact an
  administrator or purchase a plan
