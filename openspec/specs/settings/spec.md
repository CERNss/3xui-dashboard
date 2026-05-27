# settings

The runtime-mutable key/value system that backs admin settings, public branding,
OIDC runtime configuration, subscription templates, and data-collection knobs.

## Purpose & Boundaries

Settings are stored in PostgreSQL as string values and interpreted through
typed descriptors and service-level helpers. The admin HTTP surface owns
descriptor listing, validation, updates, deletes, SMTP test, and brand-icon
upload. Consumers such as `user-accounts`, `oidc-providers`,
`traffic-statistics`, `subscription`, and data collectors own their runtime
semantics.

## Storage

`backend/internal/model/setting.go` defines the setting row:

```go
type Setting struct {
    Key       string `gorm:"primaryKey"`
    Value     string
    UpdatedAt time.Time
}
```

## Descriptor Groups

Known setting descriptors live in `backend/internal/handler/admin/setting.go`
and are exposed to the UI. The group field is the stable UI grouping key.

| Group | Examples |
|---|---|
| `registration` | public registration, email verification, email domain allowlist, starter balance/plans |
| `subscription` | remark model, Clash template YAML, sing-box template JSON, proxy group strategy, rule providers |
| `traffic` | traffic warning/critical percentages, expiry warning days |
| `data_collection` | node health and traffic collection enablement, interval, concurrency, timeout, retry, retention |
| `other` | OIDC runtime values and brand title/subtitle/description/footer/icon |

Unknown persisted keys MAY be returned by the list endpoint for operator
visibility, but new first-class behavior SHOULD add a descriptor.

## Requirements

### Requirement: Key/Value Persistence

The system SHALL persist settings in the `settings` table with `(key, value,
updated_at)` semantics.

#### Scenario: Get a missing key

- **WHEN** `SettingRepo.Get(ctx, "missing")` is called and no row exists
- **THEN** it SHALL return `("", false, nil)`
- **AND** SHALL NOT surface `gorm.ErrRecordNotFound` to callers.

#### Scenario: Set upserts

- **WHEN** `SettingRepo.Set(ctx, "k", "v")` is called and no row exists
- **THEN** the system SHALL insert the row
- **AND** a later `Set(ctx, "k", "v2")` SHALL update the existing row using key conflict handling.

#### Scenario: Delete removes override

- **WHEN** `SettingRepo.Delete(ctx, "k")` is called
- **THEN** the row SHALL be removed
- **AND** subsequent typed reads SHALL fall back to their configured default.

### Requirement: Typed Accessors Provide Defaults

The repository SHALL expose typed accessors for bool, int, and string settings
with caller-provided defaults.

#### Scenario: Bool setting missing

- **GIVEN** no DB row exists for `public_registration_enabled`
- **WHEN** user service reads the setting with default `cfg.PublicRegistration`
- **THEN** the returned value SHALL be the config default.

#### Scenario: Malformed int value

- **GIVEN** a persisted int setting contains non-integer text
- **WHEN** a typed int accessor reads it
- **THEN** the accessor SHALL return an error or a validated fallback according to the caller contract
- **AND** the request path SHALL not panic.

### Requirement: Admin Settings HTTP Surface Is Descriptor-Driven

The system SHALL expose admin-authenticated settings endpoints under
`/api/admin/settings`.

#### Scenario: List settings

- **WHEN** an admin GETs `/api/admin/settings`
- **THEN** the response SHALL contain `settings`
- **AND** each known setting item SHALL include `key`, `label`, optional `label_zh`, `type`, `group`, `default`, `description`, optional `description_zh`, `value`, `has_override`, and `env_fallback`.

#### Scenario: Unknown persisted rows are visible

- **GIVEN** a row exists in the table whose key has no descriptor
- **WHEN** an admin lists settings
- **THEN** the row SHALL appear as a string setting in group `other`
- **AND** `has_override` SHALL be true.

#### Scenario: PUT validates type and state

- **WHEN** an admin PUTs `/api/admin/settings/:key` with `{"value": "..."}`
- **THEN** the handler SHALL validate the value against the descriptor type and per-key rules
- **AND** on success SHALL upsert the row and return the stored key/value.

#### Scenario: DELETE reverts to fallback

- **WHEN** an admin DELETEs `/api/admin/settings/:key`
- **THEN** the handler SHALL delete the row
- **AND** respond `204 No Content`.

### Requirement: Validation Matches Setting Semantics

The admin handler SHALL validate values that have operational constraints.

#### Scenario: Bool validation

- **WHEN** a bool setting is updated
- **THEN** accepted values SHALL include `true`, `false`, `1`, `0`, `yes`, `no`, `on`, and `off`
- **AND** other values SHALL be rejected with HTTP 400.

#### Scenario: Data collection bounds

- **WHEN** data-collection interval, concurrency, timeout, retry, or retention settings are updated
- **THEN** the handler SHALL enforce the configured numeric bounds
- **AND** timeout SHALL NOT exceed the matching interval.

#### Scenario: Subscription templates

- **WHEN** `clash_template_yaml` or `singbox_template_json` is updated with a non-empty value
- **THEN** the handler SHALL parse it as YAML/JSON object content
- **AND** require the `${proxies}` placeholder.

#### Scenario: OIDC URL settings

- **WHEN** OIDC issuer, redirect, icon, auth, token, JWKS, or userinfo URL settings are updated
- **THEN** each non-empty value SHALL be an absolute `http` or `https` URL.

#### Scenario: Branding text

- **WHEN** brand title, subtitle, description, or footer is updated
- **THEN** the handler SHALL enforce the per-field character limits.

### Requirement: Branding Is Public And Server-Driven

The system SHALL expose public branding metadata for the SPA chrome.

#### Scenario: Public branding endpoint

- **WHEN** a client GETs `/api/public/branding`
- **THEN** the response SHALL include `icon_url`, `title`, `subtitle`, `description`, and `footer`
- **AND** empty stored values SHALL fall back to built-in brand defaults.

#### Scenario: Brand icon upload

- **WHEN** an admin POSTs `/api/admin/settings/branding/icon` with a supported image
- **THEN** the handler SHALL store it under `uploads/branding`
- **AND** persist the resulting `/uploads/branding/...` URL in `brand_icon_url`.

### Requirement: SMTP Test Is Admin-Only

The system SHALL let admins test SMTP delivery without triggering a user flow.

#### Scenario: SMTP test succeeds

- **WHEN** an admin POSTs `/api/admin/settings/smtp-test` with a valid `to`
- **AND** SMTP is enabled
- **THEN** the handler SHALL send a one-shot test email
- **AND** respond `200 OK` when accepted by the SMTP relay.

#### Scenario: SMTP unavailable

- **WHEN** SMTP is not enabled
- **THEN** the test endpoint SHALL respond `503 Service Unavailable`.

## Out of Scope

- Per-user or per-tenant settings.
- Setting change history beyond the general admin audit system.
- Cross-process setting cache invalidation.
