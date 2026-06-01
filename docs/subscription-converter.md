# Subscription converter (`/sub`)

The `/sub` endpoint aggregates a user's provisioned clients across the
whole fleet and renders them into client-specific subscription formats,
driven by admin-configurable routing **profiles**. It's the built-in
alternative to running a separate subconverter — and because the input
is our own structured client data (not arbitrary upstream URLs), it can
do things subconverter can't (live preview, per-plan profiles, traffic
headers).

## End-to-end flow

1. **Admin** configures routing profiles + rulesets via
   `/api/admin/subscription/*`. A default profile + its rulesets are
   **boot-seeded**, so the system works out of the box with zero config.
2. **User** opens the portal → *Subscription* → picks a **client type**
   (Base64 / Clash / sing-box / Surge / SIP008 / WireGuard).
3. The portal links to `/sub/<subId>?format=<target>` (or
   `/sub/<target>/<subId>`); `?profile=<key>` selects a non-default
   profile.
4. The handler resolves the profile (`?profile=` → DB default → built-in
   code default), loads its rulesets, runs the policy engine, and renders
   the target format. Each target only includes the **protocols it
   supports** (e.g. Surge skips VLESS), so the link is always
   self-consistent.

## Architecture

- **`internal/sub/policy`** — target-agnostic engine.
  `Resolve(profile, nodeNames, rulesets)` flattens a profile into an IR:
  proxy-groups with computed membership, ordered rules, referenced
  rulesets. `DefaultProfile()` / `DefaultRulesets()` reproduce the
  built-in ACL4SSR-style policy (so a zero-config deployment renders
  exactly as before profiles existed).
- **`internal/sub`** — per-target serializers. `clash_policy.go` (Clash
  YAML blocks), `surge_node.go` + `surge_policy.go` (Surge `.conf`). Node
  serializers return `ok=false` for protocols a target can't speak →
  "client type → supported-protocol subscription".
- **`internal/sub/template`** — substitutes rendered blocks into
  per-target skeletons. Operators can override the base skeleton via the
  `clash_template_yaml` / `singbox_template_json` settings.
- **`internal/sub/ruleset`** — in-memory fetch + TTL cache for
  self-hosted rule lists.

## Data model (migrations `0002`, `0003`)

- `subscription_profiles`: `key`, `is_default`, `enabled`,
  `ruleset_mode` (`passthrough` | `self_hosted`), plus JSONB
  `proxy_groups` / `rules` / `transforms` / `base_overrides`.
- `subscription_rulesets`: `key`, `source_type` (`remote` | `inline`),
  `url` / `content`, `behavior`, `format`, `ttl_seconds`, plus
  server-side fetch-cache columns.

## Ruleset delivery modes (per profile)

- **`passthrough`** (default): rule-providers point at the upstream URL;
  the client fetches the list itself. Lean; the standard ACL4SSR usage.
- **`self_hosted`**: rule-providers point at `/sub/ruleset/<key>`; we
  fetch + cache + serve the list. Survives the client being unable to
  reach the upstream; required for preview.

## Status

| Area | Status |
|---|---|
| Targets: base64, Clash, sing-box, SIP008, WireGuard | ✅ |
| Surge target (`/sub/surge`) | ✅ proxies + groups + inline rules |
| Policy engine + DB-backed profiles + `?profile=` + boot-seed | ✅ |
| Ruleset modes + `/sub/ruleset/:key` | ✅ (Clash) |
| Admin CRUD API (`/api/admin/subscription/{profiles,rulesets}`) | ✅ |
| Portal client-type selector | ✅ |

## Deferred / TODO

1. **Cross-format rulesets** (`TODO(sub-rulesets)` in `surge_policy.go`).
   Rule *lists* are client-format-specific, but the default rulesets are
   Clash-format (Loyalsoldier). Today non-Clash targets emit **inline
   rules only** and skip `RULE-SET` (so Surge etc. stay correct, just
   coarser). Full fix, pick one:
   - **fetch + convert**: in `self_hosted` mode, fetch the Clash list and
     convert to the target's format, served from
     `/sub/ruleset/<key>?target=surge`. (Per-behavior converters:
     domain/ipcidr easy, classical harder.)
   - **per-target URLs**: add `surge_url` / `quanx_url` / … to the
     ruleset model; admin points each target at a target-format list.
2. **QuanX / Loon renderers** — same shape as Surge (node serializer
   skipping VLESS + policy serializer). QuanX additionally needs matcher
   remapping (`DOMAIN-SUFFIX`→`host-suffix`, `MATCH`→`final`) and uses a
   `[filter_remote]` section for rulesets. Inline-rules-only until (1).
3. **Admin profile/ruleset editor UI** — the CRUD API exists; the panel
   UI to manage profiles / rulesets / `ruleset_mode` is not built yet.
   Today: edit via the API, or rely on the boot-seeded default.
4. **Suspend → disable clients on nodes** — design decided (not yet
   wired). On suspend, loop the user's ownerships through
   `clientService.SetClientEnabled(false)` + `ownership.SetEnabled(false)`;
   unsuspend re-enables the non-expired ones; WireGuard has no per-peer
   enable so it's `RemovePeer` / re-provision. Reuses the expiry job's
   node-disable path. Today suspend only blocks the panel API + `/sub`
   fetch — already-downloaded links keep working until this lands.
