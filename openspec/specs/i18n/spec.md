# i18n

Translation strategy for the React SPA, built on `i18next` and
`react-i18next`.

## Purpose & Boundaries

The frontend supports English and Chinese. Locale selection is purely
client-side and applies to auth, admin, and portal surfaces. Backend API error
strings may still be returned as operator-facing text and are not translated by
this module.

## Files

```
frontend/src/i18n/
  index.ts       - i18next initialization and initial locale resolution
  locales/en.ts  - English messages
  locales/zh.ts  - Chinese messages
```

Locale persistence uses `localStorage["dashboard.locale"]`. `useAppStore`
mirrors the active UI locale as `en-US` or `zh-CN`, while i18next uses `en` or
`zh`.

## Initial Locale Resolution

```
1. localStorage["dashboard.locale"] in {"en", "en-US", "zh", "zh-CN"} -> normalized locale
2. navigator.language starts with "zh" -> "zh"
3. fallback -> "en"
```

## Requirements

### Requirement: Single i18next Instance

The system SHALL initialize one i18next instance in `frontend/src/i18n/index.ts`
and make it available through `react-i18next`.

#### Scenario: Initialization config

- **WHEN** i18next initializes
- **THEN** it SHALL install `initReactI18next`
- **AND** register `zh` and `en` translation resources
- **AND** set `fallbackLng` to `en`
- **AND** set `returnNull: false`.

#### Scenario: Missing key visibility

- **WHEN** a translation key is missing in development
- **THEN** the system SHALL keep the missing key visible or log a missing-key warning
- **AND** the UI SHALL NOT silently render an empty string.

### Requirement: Interpolation Uses Existing Brace Syntax

Locale strings SHALL support `{name}` interpolation syntax.

#### Scenario: Value interpolation

- **GIVEN** a translation string contains `{value}`
- **WHEN** a component calls `t(key, { value: "2.51 GB" })`
- **THEN** the rendered copy SHALL include `2.51 GB` in the placeholder position.

### Requirement: Locale Switch Persists And Propagates

The system SHALL keep `useAppStore().locale`, localStorage, and i18next's active
language in sync.

#### Scenario: User toggles locale

- **WHEN** `LocaleSwitcher` changes from `en-US` to `zh-CN`
- **THEN** `useAppStore().setLocale("zh-CN")` SHALL persist `dashboard.locale = "zh"`
- **AND** `i18n.changeLanguage("zh")` SHALL run in the same user interaction.

#### Scenario: Reload after locale choice

- **GIVEN** localStorage contains `dashboard.locale = "zh"`
- **WHEN** the SPA bootstraps
- **THEN** i18next SHALL initialize with `lng: "zh"` regardless of browser language.

### Requirement: User-Facing Surfaces Use Translation Keys

The frontend SHALL prefer translation keys for visible user-facing copy,
especially auth and portal flows.

#### Scenario: Auth surface copy

- **WHEN** `Login.tsx` renders labels, validation messages, and OIDC copy
- **THEN** those strings SHOULD come from `useTranslation().t(...)` with defaults only where necessary.

#### Scenario: Admin data labels

- **WHEN** admin navigation, settings groups, and common controls render
- **THEN** they SHALL use shared i18n keys so locale switching remains coherent across repeated chrome.

## Out of Scope

- RTL layout support.
- Dynamic locale bundle loading.
- Backend-side error translation.
