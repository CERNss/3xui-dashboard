## MODIFIED Requirements

### Requirement: Subscription Output Formats

The subscription handler SHALL include Hysteria entries in formats
that have a Hysteria representation (URI bundle, Clash, sing-box)
and skip them in formats that don't (SIP008).

#### Scenario: Hysteria URI in base64 bundle

- **WHEN** a user's subscription contains a Hysteria entry and `?format=base64` (or auto-detected) is requested
- **THEN** the link bundle SHALL include a line `hysteria2://<auth>@<host>:<port>/?sni=<sni>&alpn=h3&insecure=0#<remark>`
- **AND** the URI SHALL URL-encode the `auth`, `sni`, and `remark` query/fragment components

#### Scenario: Clash hysteria2 proxy

- **WHEN** `?format=clash` is requested and the user has at least one Hysteria entry
- **THEN** the Mihomo YAML output SHALL contain a `proxies:` entry of type `hysteria2` per Hysteria client, with `password: <auth>`, `sni: <sni>`, `alpn: [h3]`, `up: 0`, `down: 0`
- **AND** the proxy SHALL be included in auto-select + select groups alongside other protocols

#### Scenario: sing-box hysteria2 outbound

- **WHEN** `?format=singbox` is requested and the user has at least one Hysteria entry
- **THEN** the sing-box JSON output SHALL contain an `outbound` entry of type `hysteria2` per Hysteria client

#### Scenario: SIP008 skips Hysteria

- **WHEN** `?format=sip008` is requested
- **THEN** Hysteria entries SHALL be omitted (SIP008 only represents Shadowsocks)
- **AND** the `X-Subscription-Hint` header SHALL list non-SS-representable protocols the user has, separated by `;`
