# settings

The runtime-mutable key/value store that backs admin toggles which
shouldn't require a restart to change (public-registration flag,
domain allowlist, traffic warning thresholds, expiry warning days).

## Purpose & boundaries

`settings` lives in a Postgres table; values are TEXT and coerced via
typed helpers at the read site. The repository (`internal/repository/setting.go`)
is the only writer; admin HTTP handlers consume it via
`SettingHandler` (`internal/handler/admin/setting.go`).

Adjacent: `user-accounts` reads `public_registration_enabled` and
`email_domain_allowlist`; the traffic threshold settings are consumed
by the event publishers in `traffic-statistics`.

## Known keys

The system ships with the following recognized keys (admin UI gates
edits to this list):

| Key | Type | Default | Purpose |
|---|---|---|---|
| `public_registration_enabled` | bool | `cfg.PublicRegistration` env | Gates portal self-serve registration. |
| `email_domain_allowlist` | string (CSV) | `cfg.EmailDomainAllowlist` env (joined) | Restricts which email domains may register / bind. |
| `subscription_remark_model` | string | (no default; uses upstream default) | Template for link remark formatting in subscription output. |
| `traffic_warn_pct` | int | 80 | % of limit at which `client.traffic_threshold` fires. |
| `traffic_critical_pct` | int | 95 | % of limit at which `client.traffic_threshold` fires a second event. |
| `expiry_warn_days` | int | 3 | Days before client expiry at which `client.expiring_soon` fires. |

Unrecognized keys SHALL be rejected by the admin handler so typos
don't silently persist.

## Requirements

### Requirement: Key/Value Persistence

The system SHALL persist settings in the `settings` table with `(key,
value, updated_at)` columns, where key is the primary key.

#### Scenario: Get a missing key

- **WHEN** `SettingRepo.Get(ctx, "missing")` is called and no row exists
- **THEN** it SHALL return `("", false, nil)` — empty string, present=false, nil error
- **AND** SHALL NOT return `gorm.ErrRecordNotFound`

#### Scenario: Set upserts

- **WHEN** `SettingRepo.Set(ctx, "k", "v")` is called and no row exists
- **THEN** the system SHALL insert `(k, v, now())`

- **WHEN** Set is called again with `(k, "v2")`
- **THEN** the system SHALL UPDATE the row (value + updated_at), not insert a duplicate
- **AND** rely on Postgres `ON CONFLICT (key) DO UPDATE` (GORM `clause.OnConflict`)

#### Scenario: Delete removes the override

- **WHEN** `SettingRepo.Delete(ctx, "k")` is called
- **THEN** the row SHALL be removed
- **AND** subsequent reads SHALL hit the env-default fallback

### Requirement: Typed Accessors With Env Fallback

The system SHALL expose typed read helpers (`GetBool`, `GetInt`,
`GetString`) that return the env-derived default when no DB row exists,
so the system is always usable on a fresh database.

#### Scenario: GetBool reads from DB then env

- **GIVEN** the env var `PUBLIC_REGISTRATION=true` and no DB row for `public_registration_enabled`
- **WHEN** the user service calls `publicRegistrationEnabled(ctx)`
- **THEN** the system SHALL return `true` (env default)

- **GIVEN** an admin has set the DB value to `"false"`
- **WHEN** the same call runs
- **THEN** the system SHALL return `false` (DB overrides env)

#### Scenario: Type coercion failure falls back

- **GIVEN** an operator manually wrote a malformed value (e.g. `traffic_warn_pct = "abc"`)
- **WHEN** the typed getter runs
- **THEN** the getter SHALL log a WARN with the bad value and fall back to the env default
- **AND** SHALL NOT crash the request

### Requirement: Admin HTTP Surface

The system SHALL expose `GET /api/admin/settings`, `PUT
/api/admin/settings/:key`, and `DELETE /api/admin/settings/:key` for
admin-controlled toggling of known keys.

#### Scenario: List with current values

- **WHEN** an admin GETs `/api/admin/settings`
- **THEN** the response SHALL be a JSON array, one element per known key, each containing:
  - `key`
  - `label` (human-readable)
  - `type` (`bool` / `int` / `string`)
  - `value` (current effective value as string)
  - `has_override` (true if a DB row exists, false if value comes from env)

#### Scenario: Unknown key rejected on PUT

- **WHEN** an admin PUTs `/api/admin/settings/foo` where `foo` is not in the known-key set
- **THEN** the system SHALL respond HTTP 400 with a "unknown setting key" error
- **AND** SHALL NOT insert a row

#### Scenario: Type-validated PUT

- **WHEN** an admin PUTs `/api/admin/settings/traffic_warn_pct` with body `"value": "abc"`
- **THEN** the system SHALL respond HTTP 400 because `abc` is not parseable as int
- **AND** SHALL NOT insert the row

#### Scenario: DELETE reverts to default

- **WHEN** an admin DELETEs `/api/admin/settings/public_registration_enabled`
- **THEN** the DB row SHALL be removed
- **AND** subsequent reads SHALL return the env default value

## Out of scope

- Per-user or per-tenant settings (only one administrator and a single
  effective config today).
- Setting change history / audit log (use git-tracked `.env` for the
  immutable subset; runtime changes are not audited yet).
- Notify other dashboard replicas of setting changes (single-process
  deployment; restart picks up new values regardless).
