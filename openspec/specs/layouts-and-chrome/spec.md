# layouts-and-chrome

The three top-level layout shells the SPA hangs every page under:
`AuthLayout`, `AdminLayout`, `PortalLayout`. Owns brand block,
sidebar / topbar structure, theme toggle placement, footer.

## Purpose & boundaries

These layouts are the chrome — they decide what the page looks like
*around* the route-view slot. They depend on `design-system` for
tokens and `theme-system` for the dark/light state.

Page-level content (Status / Nodes / Inbounds / Settings dashboards,
portal Dashboard / Subscription pages) lives in `views/` and is
covered by `admin-views`.

## Files

```
frontend/src/components/layout/
  AuthLayout.vue       — wraps the unified /login page; brand + card
  AdminLayout.vue      — wraps /admin/* routes; sidebar + main
  PortalLayout.vue     — wraps /portal/* routes; topbar + main
```

## Requirements

### Requirement: AuthLayout For Pre-Login Pages

The system SHALL render every pre-login page (today: `/login`) inside
`AuthLayout`, which provides a centered card on a textured background
with a centered brand block above the card.

#### Scenario: Background composition

- **WHEN** AuthLayout renders
- **THEN** the page SHALL show, in z-order from back to front:
  - Page background: `bg-surface-50` (light) / `bg-[#0b1018]` (dark)
  - Two ambient gradient blobs (accent top-left, primary bottom-right) at `opacity-50%` / 24px blur
  - A 24px dotted radial-gradient grid overlay at `opacity-[0.35]` (light) / `[0.22]` (dark)
  - Top + bottom edge gradient fades so the grid doesn't fight content
  - The brand block (logo + brand name + slogan), centered
  - The form card, centered, max-w-md
  - A "© 2026 3xui Central · 自托管 multi-node 控制面板" footer below the card

#### Scenario: Brand block style

- **WHEN** the brand block renders
- **THEN** the logo SHALL be a 64px solid `bg-accent-500` square with `rounded-2xl`, a lightning glyph, and `shadow-elevated` + `ring-1 ring-accent-700/40`
- **AND** an ambient glow (`bg-accent-500/40 blur-2xl`) SHALL sit behind the logo
- **AND** the brand name "3xui Central" SHALL render at `text-[2rem] font-bold` with `bg-clip-text text-transparent` filled by a `from-accent-500 to-accent-700` linear gradient
- **AND** the slogan SHALL read "Multi-node 3x-ui · Fleet 聚合 · 流量分账 · 订阅导出" at `text-sm text-surface-500`

#### Scenario: Card slot

- **WHEN** AuthLayout receives `card-title` + `card-subtitle` props
- **THEN** the card SHALL show a centered `text-2xl font-bold` title and an optional `text-sm text-surface-500` subtitle above the slot

#### Scenario: No theme toggle pre-login

- **WHEN** AuthLayout renders
- **THEN** the page SHALL NOT show a theme toggle anywhere — see `theme-system` Requirement: "Toggle is only available post-authentication"

### Requirement: AdminLayout Sidebar Topology

The system SHALL render every `/admin/*` route inside `AdminLayout`,
which provides a fixed 256px left sidebar plus a fluid main area.

#### Scenario: Sidebar structure (top to bottom)

- **WHEN** AdminLayout renders
- **THEN** the sidebar SHALL contain:
  1. Brand block: 40px accent gradient square (rounded-2xl) + "3xui Central" `text-body-md` + "CENTRAL PANEL" `text-eyebrow` eyebrow
  2. Section "总览": `系统状态` nav item
  3. Section "运维": `节点列表`, `入站列表` nav items
  4. Section "系统": `面板设置` nav item
  5. Theme toggle row (Sub2API pattern: labeled item with sun/moon icon + "浅色模式" / "深色模式" label)
  6. User card footer: 32px ink square with username initial + username + "SIGNED IN" eyebrow + logout icon button

#### Scenario: Section header style

- **WHEN** a nav section header renders
- **THEN** it SHALL be `text-eyebrow uppercase tracking-eyebrow text-surface-400` with `px-3 pb-1`
- **AND** SHALL NOT be a clickable element

#### Scenario: Active nav item state

- **WHEN** the current route matches a nav item's `:to`
- **THEN** that item SHALL have `bg-accent-50 text-accent-700` (light) / `bg-accent-950/40 text-accent-300` (dark)
- **AND** `shadow-rail` (2px left accent bar)
- **AND** other nav items SHALL be `text-surface-600` resting / `hover:bg-surface-100 hover:text-ink-900`

#### Scenario: Main content max-width

- **WHEN** AdminLayout renders the route-view slot
- **THEN** the `<main>` element SHALL constrain its content to `max-w-page` (1500px)
- **AND** apply `px-8 py-9` padding

### Requirement: PortalLayout Topbar Topology

The system SHALL render every `/portal/*` route inside `PortalLayout`,
which provides a 56px-tall horizontal top bar plus a fluid content
area below.

#### Scenario: Topbar structure (left to right)

- **WHEN** PortalLayout renders
- **THEN** the topbar SHALL contain:
  - Brand name "3xui 控制台" (or `app.title` i18n key) at `text-base font-semibold tracking-tight text-ink-900`
  - Right-side nav: `Dashboard` link, theme toggle icon button, logout button
- **AND** the topbar SHALL have `border-b border-surface-100` and `bg-surface-0` (light) / `bg-surface-900` (dark)

#### Scenario: Active nav link style

- **WHEN** the current route matches a topbar link's `:to`
- **THEN** the link SHALL be `text-accent-700` (light) / `text-accent-300` (dark)
- **AND** other links SHALL be `text-surface-600` with hover surface tint

#### Scenario: Logout routes through unified login

- **WHEN** the user clicks the topbar logout button
- **THEN** the portal store SHALL be cleared
- **AND** the router SHALL push `{name: 'login', query: {hint: 'portal'}}` (NOT the obsolete `portal.login` name)

### Requirement: Layout Loaded Per Route Meta

The router SHALL select the layout based on each route's parent:
`/login` mounts the Login view inside `AuthLayout` (the view itself
imports the layout); `/admin/*` is wrapped by `AdminLayout` (declared
as the parent component); `/portal/*` by `PortalLayout`.

#### Scenario: Admin route mounts under AdminLayout

- **WHEN** the user navigates to `/admin/status`
- **THEN** the rendered tree SHALL be `AdminLayout > router-view (=> Status.vue)`
- **AND** AdminLayout SHALL provide the sidebar + main scaffolding

#### Scenario: Portal route mounts under PortalLayout

- **WHEN** the user navigates to `/portal/dashboard`
- **THEN** the rendered tree SHALL be `PortalLayout > router-view (=> portal/Dashboard.vue)`

## Out of scope

- Mobile responsive collapse of the admin sidebar (sidebar is currently
  always 256px; mobile layout is deferred).
- Internationalization of the brand name (literal "3xui Central"
  everywhere; brand is not translated).
- Per-page chrome variants (no per-view layout overrides).
