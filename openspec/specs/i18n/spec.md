# i18n

Translation strategy and locale-routing for the Vue 3 SPA, built on
`vue-i18n` v9 in composition (non-legacy) mode. Lives in
`frontend/src/i18n/`.

## Purpose & boundaries

Two locales today: `en` and `zh-CN` (key `zh`). The dashboard targets
operators in both English-default and Chinese-default environments;
admin views were authored Chinese-first based on operator feedback, so
some labels in `views/admin/*.vue` use literal Chinese text rather than
`$t()` keys. This is acceptable for the admin surface (single
operator, won't translate); the portal surface DOES use `$t()` end-to-
end so it can ship in either language.

## Layout

```
frontend/src/i18n/
  index.ts            — createI18n + initialLocale + bindI18nToStore
  locales/
    en.ts             — default fallback
    zh.ts             — primary for ops in mainland China
```

Locale persistence: `localStorage["dashboard.locale"]` + an `app`
Pinia store that mirrors it. Bound in `bindI18nToStore` so toggling
the locale in the store updates the i18n active locale in lockstep.

## Initial locale resolution

```
1. localStorage["dashboard.locale"] in {"en", "zh"} → use it
2. navigator.language starts with "zh" → "zh"
3. fallback → "en"
```

Resolved BEFORE Pinia activates so the first paint is in the right
language (matches theme-system's same "set before mount" pattern).

## Requirements

### Requirement: Single i18n Instance For The SPA

The system SHALL construct exactly one `createI18n` instance in
`i18n/index.ts` and inject it via `app.use(i18n)` in `main.ts`.

#### Scenario: Composition API mode

- **WHEN** `createI18n` is called
- **THEN** the config SHALL set `legacy: false` and `globalInjection: true`
- **AND** the `$t` function SHALL be available in every template

#### Scenario: Fallback chain

- **WHEN** a translation key is missing in the active locale
- **THEN** the system SHALL fall back to the `en` locale
- **AND** if the key is also missing in `en`, vue-i18n's default behavior of rendering the key path SHALL apply (the missing key SHALL be visible in dev so it gets caught at PR review)

### Requirement: Initial Locale Detection

The system SHALL determine the active locale before Vue mounts so the
first paint avoids a flash of wrong language.

#### Scenario: User has a persisted preference

- **GIVEN** localStorage holds `dashboard.locale = "zh"`
- **WHEN** the SPA bootstraps
- **THEN** the i18n instance SHALL initialize with locale `zh` regardless of `navigator.language`

#### Scenario: First visit, Chinese browser

- **GIVEN** no localStorage entry AND `navigator.language === "zh-CN"`
- **WHEN** the SPA bootstraps
- **THEN** the active locale SHALL be `zh`

#### Scenario: First visit, English browser

- **GIVEN** no localStorage entry AND `navigator.language === "en-US"`
- **WHEN** the SPA bootstraps
- **THEN** the active locale SHALL be `en`

### Requirement: Locale Bound To App Store

The system SHALL keep the i18n locale in lockstep with the `useAppStore().locale`
so user-facing locale switches persist and propagate.

#### Scenario: Store mutation updates i18n

- **WHEN** code calls `appStore.setLocale("en")`
- **THEN** `i18n.global.locale.value` SHALL become `"en"` in the same tick (via the binding installed by `bindI18nToStore`)
- **AND** localStorage SHALL persist the new value

### Requirement: Admin Surface May Use Literal Text

The system SHALL allow admin views (`frontend/src/views/admin/*.vue`)
to use literal Chinese text without `$t()` lookups, because the admin
audience is the single operator and translation is not required.

#### Scenario: Admin view uses literal label

- **WHEN** an admin view button reads `添加节点` directly in the template
- **THEN** this SHALL be acceptable — no spec violation
- **AND** the `en.ts` / `zh.ts` files SHALL NOT be required to carry an entry for it

#### Scenario: Portal surface MUST use $t

- **WHEN** a portal view (`frontend/src/views/portal/*.vue`) renders user-facing copy
- **THEN** the copy SHALL come through `$t('key.path')`
- **AND** both `en.ts` and `zh.ts` SHALL provide the key

## Out of scope

- RTL languages (Arabic, Hebrew) — not in scope for v1.
- Plural rules beyond what vue-i18n's defaults handle.
- Dynamic locale loading (the two locale files are bundled into the SPA
  — total size is trivial).
- Backend-side translations (error messages from the API are returned
  in operator-facing Chinese / English mix; not gated on locale).
