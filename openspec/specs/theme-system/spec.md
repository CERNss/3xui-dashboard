# theme-system

Light/dark/system theme state for the React SPA, persisted client-side and
applied through AntD `ConfigProvider` plus the `<html>` `dark` class.

## Purpose & Boundaries

The theme system decides which visual theme is active and where users can
change it. Token values are owned by `design-system`.

## State Machine

Tracked in `frontend/src/stores/theme.ts` with Zustand:

```
localStorage["cp.theme"] in {"light","dark","system"}
       │
       ├─ "light"  -> resolvedTheme "light"
       ├─ "dark"   -> resolvedTheme "dark"
       └─ "system" or missing -> matchMedia("(prefers-color-scheme: dark)")

toggle() flips the resolved light/dark state and persists the explicit mode.
setMode("system") restores OS-following behavior.
```

## Requirements

### Requirement: Initial Theme Resolves Before First Paint Where Possible

The system SHALL initialize theme state from localStorage and OS preference as
the SPA starts.

#### Scenario: Stored explicit theme

- **GIVEN** localStorage contains `cp.theme = "dark"`
- **WHEN** `useThemeStore` initializes
- **THEN** `mode` SHALL be `dark`
- **AND** `resolvedTheme` SHALL be `dark`
- **AND** the `<html>` element SHALL have class `dark` after initialization.

#### Scenario: Stored system mode

- **GIVEN** localStorage contains `cp.theme = "system"`
- **WHEN** `useThemeStore` initializes
- **THEN** `resolvedTheme` SHALL follow `window.matchMedia("(prefers-color-scheme: dark)")`.

#### Scenario: Missing storage

- **GIVEN** no supported `cp.theme` value exists
- **WHEN** the SPA initializes
- **THEN** `mode` SHALL default to `system`.

### Requirement: Theme Applies Through AntD And HTML Class

The system SHALL apply the active theme to AntD and to global CSS hooks.

#### Scenario: Root provider chooses theme config

- **WHEN** `resolvedTheme` is `dark`
- **THEN** `main.tsx` SHALL pass `darkTheme` to AntD `ConfigProvider`
- **AND** when `resolvedTheme` is `light`, it SHALL pass `lightTheme`.

#### Scenario: HTML dark class

- **WHEN** theme mode changes
- **THEN** `stores/theme.ts::applyToHtml` SHALL toggle `document.documentElement.classList` so global CSS can style shell backgrounds.

### Requirement: User Override Persists Across Reloads

User-driven theme changes SHALL persist to localStorage.

#### Scenario: Toggle from light to dark

- **GIVEN** `resolvedTheme` is `light`
- **WHEN** the user activates the theme toggle
- **THEN** the store SHALL persist `cp.theme = "dark"`
- **AND** AntD tokens and the `<html>` class SHALL update without replacing the route tree.

#### Scenario: Toggle from dark to light

- **GIVEN** `resolvedTheme` is `dark`
- **WHEN** the user activates the theme toggle
- **THEN** the store SHALL persist `cp.theme = "light"`.

### Requirement: Theme Toggle Is Post-Auth Chrome

The system SHALL expose theme controls from authenticated shells, not as a
primary auth-page workflow.

#### Scenario: Admin shell

- **WHEN** an admin route renders
- **THEN** `AdminLayout` SHALL provide a sidebar theme toggle action.

#### Scenario: Portal shell

- **WHEN** a portal route renders
- **THEN** PortalLayout MAY expose theme switching as part of authenticated portal chrome.

## Out of Scope

- Server-side theme persistence.
- Per-component theme overrides outside AntD/theme CSS.
- Live OS preference listeners after the app is already running.
