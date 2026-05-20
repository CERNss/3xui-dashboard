# theme-system

Light/dark theme with system-preference fallback and per-user override
persisted in localStorage.

## Purpose & boundaries

Both admin and portal SPAs need to:
- Render in the user's preferred OS theme on first visit.
- Let the user override that choice from inside the app.
- Persist the override across reloads.
- Avoid the "light flash" before the chosen theme is applied.

The theme system is purely client-side — no server roundtrip. Backend
templates / emails (if any) do not honor it.

## State machine

Tracked in `frontend/src/stores/theme.ts` (Pinia store):

```
            ┌─ localStorage has 'dark' ─► dark
readInitial ┼─ localStorage has 'light' ► light
            │                              ▲
            └─ no entry ───────────────────┤
                            │              │
              prefers-color-scheme: dark?  │
                  yes → dark               │
                  no  → light              │
                                           │
                                           │
        toggle() ─► flip + persist + class▼
                    ▲                      │
        ┌───────────┴─── user clicks sidebar/topbar theme icon
```

## Storage key

`cp.theme` in `localStorage` — single string, `"dark"` or `"light"`. Any
other value (or missing) triggers the system-preference fallback.

## Requirements

### Requirement: First-visit theme follows OS preference

The system SHALL default to the OS's `prefers-color-scheme` setting on
first visit, before any user interaction.

#### Scenario: First visit on a dark-mode macOS

- **GIVEN** localStorage has no `cp.theme` entry
- **AND** the browser reports `prefers-color-scheme: dark`
- **WHEN** the SPA bootstraps via `main.ts`
- **THEN** `useThemeStore().init()` SHALL apply `class="dark"` to `<html>` before mount
- **AND** the first paint SHALL render in dark mode (no light flash)

#### Scenario: First visit on a light-mode OS

- **GIVEN** no localStorage entry and `prefers-color-scheme: light`
- **WHEN** the SPA bootstraps
- **THEN** the `<html>` tag SHALL NOT have the `dark` class
- **AND** Tailwind's default (light) palette SHALL apply

#### Scenario: First visit on a no-preference browser

- **WHEN** `window.matchMedia` is undefined OR doesn't match `prefers-color-scheme: dark`
- **THEN** the system SHALL default to light

### Requirement: User override persists across reloads

When the user toggles the theme inside the app, the choice SHALL persist
to localStorage and win over the OS preference on subsequent loads.

#### Scenario: User toggles from light to dark

- **GIVEN** the SPA is currently rendering in light mode
- **WHEN** the user clicks the "深色模式" item in the sidebar/topbar
- **THEN** `useThemeStore().toggle()` SHALL flip the in-memory theme to dark
- **AND** SHALL write `"dark"` to `localStorage.cp.theme`
- **AND** SHALL toggle the `dark` class on `<html>` in the same tick

#### Scenario: Reload after override

- **GIVEN** the user previously persisted `"dark"`
- **AND** the OS preference is now light
- **WHEN** the SPA bootstraps
- **THEN** `readInitial()` SHALL return `"dark"` (localStorage wins over OS)

### Requirement: Toggle is only available post-authentication

The system SHALL NOT show a theme toggle on the pre-login pages.
Pre-login pages SHALL render whatever theme `init()` chose; user can
adjust only after entering the app.

#### Scenario: Login page chrome

- **WHEN** the user is on `/login`
- **THEN** there SHALL be no theme toggle anywhere on the page (top-right was removed)
- **AND** the page SHALL still respect the active theme (dark/light styles cascade)

#### Scenario: Toggle in admin sidebar

- **WHEN** the user is on any `/admin/*` route
- **THEN** the sidebar SHALL contain a labeled nav-style row:
  `[icon] 浅色模式` if currently dark, `[icon] 深色模式` if currently light
  (label shows the mode they would SWITCH TO — matches Sub2API convention)
- **AND** the row SHALL sit above the user/logout footer block

#### Scenario: Toggle in portal topbar

- **WHEN** the user is on any `/portal/*` route
- **THEN** the topbar SHALL contain a small icon-only theme toggle
  between the nav links and the logout button

### Requirement: Theme tokens cascade through Tailwind dark variants

All UI components SHALL use Tailwind's `dark:` variant exclusively to
express dark-mode styling — no `:root.dark` raw CSS overrides except
the global body background in `style.css`.

#### Scenario: Component palette swap

- **WHEN** a component declares e.g. `bg-surface-0 dark:bg-surface-900`
- **THEN** toggling the `dark` class on `<html>` SHALL switch the rendered background without re-rendering the component

## Implementation notes

- `applyToHtml(theme)` in `stores/theme.ts` is the single mutation point
  for the `<html>` class. Both `init()` and `set()` call it.
- `init()` is called in `main.ts` BEFORE `app.mount('#app')` to avoid the
  flash-of-wrong-theme during hydration.
- Style sheet `style.css` already sets `:root.dark body` background as a
  belt-and-suspenders default; component-level `dark:` variants cover
  the rest.

## Out of scope

- Per-component theme overrides (e.g. "always dark sidebar even in light
  mode") — not needed today.
- Automatic theme switching on OS-preference change while the SPA is
  running (would require a `matchMedia` listener; current behavior: OS
  preference is only consulted on first load before any localStorage
  override exists).
- Persisting the theme server-side to mirror across devices.
