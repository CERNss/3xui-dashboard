# Tasks — add-subscription-converter

All checkboxes are `[ ]` to start — no code has shipped yet.

## 1. Template engine (`internal/sub/template/`)

- [ ] 1.1 Create `internal/sub/template/template.go` with two exported functions: `RenderClash(nodes []ClashNode, tmpl string) ([]byte, error)` and `RenderSingBox(outbounds []map[string]any, tmpl string) ([]byte, error)`. Both do string substitution on `${proxies}` and `${proxy_names}` placeholders; both run a post-render parse check (`yaml.Unmarshal` / `json.Unmarshal`) and return a wrapped error on failure so callers can fall back to defaults.
- [ ] 1.2 Create `internal/sub/template/defaults.go` with two `//go:embed` strings:
  - `defaultClashYAML` — Mihomo-targeted template with `mixed-port: 7890`, DNS block, loyalsoldier rule-providers (15 entries: reject/icloud/apple/google/proxy/direct/private/gfw/greatfire/tld-not-cn/telegrambot/lancidr/cncidr/applications), and the standard rule set ending in `MATCH,节点选择`.
  - `defaultSingBoxJSON` — sing-box equivalent with `geoip-cn` rule_set, selector + urltest outbound pair.
- [ ] 1.3 Apply `proxy_group_strategy` (`auto-only` / `select-only` / `auto+select`) to the default Clash template only (not operator overrides). Implemented as a 3-way swap of the `proxy-groups:` block.
- [ ] 1.4 Apply `rule_providers_enabled = false` to strip `rule-providers` + `rules` from the default Clash template (operator overrides bypass this).
- [ ] 1.5 Tests: `template_test.go` — defaults parse cleanly; `${proxies}` substitution produces valid YAML/JSON; invalid override returns wrapped parse error; strategy switches affect group shape; rule_providers_enabled toggle strips sections.

## 2. Per-protocol node mappers

- [ ] 2.1 Create `internal/sub/clash.go` with `clashNode(in *runtime.Inbound, c *runtime.Client, remark string) (map[string]any, error)`. Branch by `in.Protocol`:
  - vless → handle reality / xtls-vision / xtls-rprx-vision / network=tcp,ws,grpc,httpupgrade,h2,xhttp,kcp,quic
  - vmess → handle alterId + all transports
  - trojan → handle sni + transports
  - shadowsocks → cipher + password
- [ ] 2.2 Create `internal/sub/singbox.go` with `singboxOutbound(in *runtime.Inbound, c *runtime.Client, remark string) (map[string]any, error)`. Same protocol matrix but emitting sing-box's shape (`tag`, `type`, `server`, `server_port`, `uuid`, `flow`, `transport`, `tls`, etc.).
- [ ] 2.3 Create `internal/sub/sip008.go` with `func filterSSLinks(links []Link) []sip008Server`. Drops non-shadowsocks links; returns SIP008 server records (id from client UUID, remarks from Link.Remark, server/port from inbound, password+method from client/inbound).
- [ ] 2.4 Tests: `clash_test.go` / `singbox_test.go` / `sip008_test.go` — table-driven per (protocol × transport × security) fixtures, assert produced shape against golden expectations.

## 3. Assembler format methods

- [ ] 3.1 In `internal/sub/assembler.go`, add `FormatClash(d *SubscriptionData, opts FormatOpts) ([]byte, error)`. Walks `d.Links`, calls `clashNode` per Link, collects `[]ClashNode`, fetches override template from settings repo (falls back to default on empty), calls `template.RenderClash`.
- [ ] 3.2 Add `FormatSingBox(d *SubscriptionData, opts FormatOpts) ([]byte, error)` — symmetric to FormatClash but for sing-box.
- [ ] 3.3 Add `FormatSIP008(d *SubscriptionData) ([]byte, error)` — filters to SS-only and emits the SIP008 envelope; no template needed (SIP008 has a fixed shape).
- [ ] 3.4 Define `FormatOpts struct { ProxyGroupStrategy string; RuleProvidersEnabled bool; ClashTemplate, SingBoxTemplate string }`. Populated by the handler from settings before each call so config changes don't require a process restart.
- [ ] 3.5 Tests: `assembler_test.go` (or extend `links_test.go`) — given a fixture `SubscriptionData` with 3 mixed-protocol Links, each FormatX returns deserializable output containing every link's identifier.

## 4. Handler dispatch + UA detection

- [ ] 4.1 In `internal/handler/public/sub.go`, add unexported `detectFormat(qsFormat, userAgent string) Format` per design (Mihomo/Clash/Stash → clash; sing-box → singbox; Shadowsocks → sip008; else base64). `?format=` always wins.
- [ ] 4.2 Extend the existing Sub handler to dispatch on the detected format: case base64/json (existing) + case clash/singbox/sip008 (new).
- [ ] 4.3 Set the right Content-Type per format: `text/yaml; charset=utf-8` for clash; `application/json` for the others (base64 stays `text/plain`).
- [ ] 4.4 Tests: `sub_test.go` — UA matrix table; `?format=` override beats UA; unsupported `?format=foo` returns 400.

## 5. Settings keys

- [ ] 5.1 In `internal/handler/admin/setting.go`, add 4 known keys with type + validation:
  - `clash_template_yaml` (string, no length cap, validated by attempting yaml.Unmarshal on PUT)
  - `singbox_template_json` (string, validated by json.Unmarshal on PUT)
  - `proxy_group_strategy` (string, enum: `auto-only` | `select-only` | `auto+select`)
  - `rule_providers_enabled` (bool, default true)
- [ ] 5.2 Verify the Settings.vue admin page renders all 4 new entries (existing page auto-renders based on the list endpoint, so should "just work" — confirm).

## 6. Wiring + integration

- [ ] 6.1 In `internal/app/app.go::Build`, ensure the Assembler is constructed with a reference to the SettingRepo so it can read template overrides per request. If not already there, add the dep.
- [ ] 6.2 Confirm `internal/sub` imports stay clean — no cycle through handler/admin or model that wasn't there before.
- [ ] 6.3 `go build ./...` succeeds.

## 7. Manual verification

- [ ] 7.1 Provision a portal user with mixed-protocol clients on the demo node; rebuild dashboard.
- [ ] 7.2 `curl /api/public/sub/<subId>?format=clash` → drop output into Clash Verge on macOS; rules + groups render; one node connects.
- [ ] 7.3 `curl /api/public/sub/<subId>?format=singbox` → drop into sing-box mobile app; same outcome.
- [ ] 7.4 `curl /api/public/sub/<subId>?format=sip008` → verify SS-only filter works.
- [ ] 7.5 `curl -A "Mihomo/1.18.0" /api/public/sub/<subId>` (no `?format=`) → returns clash content auto-detected.
- [ ] 7.6 `curl -A "V2RayN/6.0" /api/public/sub/<subId>` (no `?format=`) → returns base64 unchanged.
- [ ] 7.7 PUT a deliberately broken `clash_template_yaml` → re-fetch; verify we fall back to default + log ERROR (not 500 to the client).
- [ ] 7.8 PUT `rule_providers_enabled=false` → re-fetch default template; verify `rule-providers` and `rules` are absent in output.

## 8. Spec deltas

- [ ] 8.1 `openspec/changes/add-subscription-converter/specs/subscription/spec.md` — MODIFIED requirements covering the three new formats + UA detection + template settings.
- [ ] 8.2 After the change ships: fold the delta into `openspec/specs/subscription/spec.md` (canonical state).
- [ ] 8.3 Update `openspec/ROADMAP.md`: flip Clash/sing-box/SIP008/UA rows ❌ → ✅; recompute 多协议 percentage (target ~85% post-change) + composite.

## 9. Out of this change (re-emphasized)

- No `internal/sub/convert/` package.
- No URL parsers (`parse_vless.go` etc.).
- No paste-to-convert endpoint.
- No node-side WireGuard / Hysteria2 / TUIC.
- No per-user template overrides.
