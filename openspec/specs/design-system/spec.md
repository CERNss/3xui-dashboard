# design-system

The visual language and reusable UI rules for the React/Ant Design SPA:
typography, theme tokens, density, focus, motion, and shared primitives.

## Purpose & Boundaries

This module defines how UI should look and be assembled. Theme state is owned
by `theme-system`; app wiring is owned by `frontend-platform-react`.

The current frontend uses AntD tokens plus `frontend/src/style.css`.

## Sources

```
frontend/src/theme.ts     - AntD light/dark ThemeConfig and breakpoints
frontend/src/style.css    - global CSS, shell classes, focus/motion rules
frontend/src/components/common/
frontend/src/components/layout/
```

## Requirements

### Requirement: Typography Uses Geist With System Fallbacks

The system SHALL use Geist Sans and Geist Mono as the primary UI typefaces.

#### Scenario: Fontsource imports

- **WHEN** `frontend/src/style.css` is loaded
- **THEN** it SHALL import `@fontsource/geist-sans` and `@fontsource/geist-mono`
- **AND** the root font stack SHALL list Geist before system and CJK fallbacks.

#### Scenario: Code typography

- **WHEN** IDs, tokens, ports, or JSON-like values render in monospace
- **THEN** they SHALL use the Geist Mono stack defined by the design system.

### Requirement: AntD Theme Tokens Are The Color Source

The system SHALL express primary visual color through AntD `ThemeConfig`
tokens in `frontend/src/theme.ts`.

#### Scenario: Light theme tokens

- **WHEN** `lightTheme` is active
- **THEN** AntD primary/link/info colors SHALL use the configured blue palette
- **AND** backgrounds/borders SHALL use the light shell palette from `theme.ts`.

#### Scenario: Dark theme tokens

- **WHEN** `darkTheme` is active
- **THEN** the theme SHALL use AntD `darkAlgorithm`
- **AND** backgrounds, borders, text, and active states SHALL come from the dark token set.

#### Scenario: Component overrides

- **WHEN** AntD Cards, Buttons, Inputs, Layout, Menu, Segmented, Tables, or Tabs render
- **THEN** component-level overrides in `theme.ts` SHALL provide the app-specific selected, border, hover, and focus colors.

### Requirement: Layout Density Is Stable

Operational pages SHALL be dense enough for repeated use while preserving clear
scan paths.

#### Scenario: Page content padding

- **WHEN** admin or portal content renders
- **THEN** layout components SHALL provide stable padding that does not depend on child loading states
- **AND** mobile/narrow shells SHALL leave room for drawers or bottom navigation.

#### Scenario: Tables and repeated lists

- **WHEN** resource tables render
- **THEN** columns, row actions, and empty states SHALL preserve stable dimensions during loading/refetching where practical.

### Requirement: Focus Is Always Visible

Interactive controls SHALL retain a visible keyboard focus state.

#### Scenario: Keyboard navigation

- **WHEN** the user tabs through sidebar actions, buttons, inputs, menus, or custom switches
- **THEN** the focused element SHALL show a visible focus indicator
- **AND** CSS SHALL NOT remove focus outlines without providing an equivalent indicator.

### Requirement: Motion Respects User Preference

The system SHALL keep transitions short and disable them for reduced-motion
users.

#### Scenario: Reduced motion

- **GIVEN** the user has `prefers-reduced-motion: reduce`
- **WHEN** layout, drawer, modal, or hover transitions would run
- **THEN** the UI SHALL reduce or remove animation duration while preserving final visual state.

### Requirement: Shared Primitives Prevent Drift

Common UI patterns SHALL be factored into shared React components before they
are copied across multiple views.

#### Scenario: Page header

- **WHEN** a page needs a title/subtitle/action row
- **THEN** it SHALL use `components/common/PageHeader.tsx`.

#### Scenario: Refresh affordance

- **WHEN** a page provides manual refresh
- **THEN** it SHALL use `components/common/RefreshButton.tsx` or an equivalent shared wrapper.

#### Scenario: Responsive list/table

- **WHEN** multiple pages need a table on desktop and card/list layout on narrow viewports
- **THEN** they SHOULD use `components/common/ResponsiveListTable.tsx`.

### Requirement: Avoid Decorative Noise

Admin and portal surfaces SHALL remain work-focused.

#### Scenario: Operational view composition

- **WHEN** admin pages render resource data
- **THEN** they SHALL avoid nested decorative cards, unrelated gradients, and one-off ornamental elements
- **AND** visual emphasis SHALL come from hierarchy, spacing, AntD tokens, and iconography.

## Out of Scope

- Per-tenant custom palettes.
- Full WCAG audit beyond the focus, contrast-by-token, and reduced-motion requirements here.
