# design-system

The token taxonomy that backs every UI surface — fonts, palette, spacing,
radii, shadows, motion. Single source of truth:
`frontend/tailwind.config.js` + `frontend/src/style.css`.

## Purpose & boundaries

Without enforced tokens, AI-generated UI drifts into "template look":
Inter font, purple gradients, card-on-card stacks, bouncy animations.
This module pins the language Linear/Vercel/Sub2API-grade UI requires.

References that shaped the current values: Marzban (dark Xray panel),
Xboard (Shadcn-based admin), Sub2API (dark grid + brand-tinted name),
集换社 (login page with bold accent heading + secondary auth row).

## Tokens

### Typography

Self-hosted via `@fontsource/geist-sans` and `@fontsource/geist-mono`
(no Google Fonts runtime dep). Imported in `main.ts`.

| Stack | Value |
|---|---|
| `font-sans` | Geist, DM Sans, Inter, Apple system, PingFang SC, Microsoft YaHei, sans-serif |
| `font-mono` | Geist Mono, JetBrains Mono, ui-monospace, SFMono-Regular, Menlo |

`font-feature-settings: 'cv02','cv03','cv04','cv11','ss01'` on `body`
enables Geist's stylistic alternates (rounder `g`, etc.).

### Font scale (CRITICAL — bumped one notch from Tailwind defaults)

| Token | Size | Purpose |
|---|---|---|
| `text-eyebrow` | 11px | Uppercase eyebrows / sectioned-nav headers |
| `text-2xs` | 12px | Dense table meta, breadcrumb-like info |
| `text-xs` | **13px** (was 12) | Smaller body / captions |
| `text-body-sm` | 14px | Denser body alt |
| `text-sm` | **15px** (was 14) | Body baseline for admin views |
| `text-base` | 16px | (rarely used directly) |
| `text-body-md` | 16px | Sub-headings, brand-name slots |
| `text-lg` | 18px | Small section heads |
| `text-xl` | 20px | Modal titles |
| `text-2xl` | 24px (`-0.01em`) | Page H1 |
| `text-3xl` | 30px (`-0.015em`) | Large H1 |
| `text-display-sm` | 32px (`-0.015em` `weight 600`) | KPI big number |
| `text-display-md` | 40px (`-0.02em` `weight 600`) | Hero |

Magic `text-[Npx]` arbitrary values are forbidden in admin views (every
instance has been refactored to one of the above; future code SHALL use
the named tokens).

### Color palette (HEX — OKLCH deferred)

All three palettes have a `0/50/100/.../950` ladder.

| Palette | Anchor `500` | Use |
|---|---|---|
| `accent-*` | `#14b8a6` (teal) | Primary brand. Restrained — used for active states, primary CTAs in dark mode, icons. |
| `primary-*` | `#6366f1` (indigo) | Semantic only — secondary signal (e.g. download arrow). |
| `surface-*` | warm stone (`#fafaf9` → `#0c0a09`) | Backgrounds and borders. Tinted away from pure gray. |
| `ink-*` | cool near-black (`#0c0e12` peak) | Primary CTAs in light mode, headings. |

Tailwind's neutral `red/amber/violet/pink` are also used for semantic
states (error/warn/Reality protocol/decorative).

### Spacing & layout

- All padding/margin SHALL use 4px-multiples — no `13px` etc.
- Max content width: `max-w-page` = 1500px (matches `AdminLayout`).
- Card padding: `p-5` (small) / `p-8` (modals).
- Card gap in grids: `gap-3` (compact) / `gap-4` (KPI strip).

### Border radius

Bumped from Tailwind defaults — cards feel softer:

| Token | Value | Tailwind default | Notes |
|---|---|---|---|
| `rounded` | 8px | 4px | Inputs |
| `rounded-lg` | 8px (no change) | 8px | Small chips, icon buttons |
| `rounded-xl` | **14px** | 12px | Inputs / buttons |
| `rounded-2xl` | **18px** | 16px | Cards |
| `rounded-3xl` | 24px (no change) | 24px | Avatar tiles |
| `rounded-full` | full | full | Pills |

### Shadow

Premium-UI principle: depth from contrast, not stacked drop shadows.

| Token | Value | When |
|---|---|---|
| `shadow-hairline` | inset 1px ring of surface-200 | Card "border" when no border declared |
| `shadow-card` | `0 1px 2px rgba(15,23,42,.04)` | Resting state on colored bg only |
| `shadow-card-hover` | small lift | Hover on cards |
| `shadow-elevated` | bigger lift | Modals only |
| `shadow-rail` | `inset 2px 0 0 accent-600` | Active sidebar item (left bar) |
| `shadow-focus` | `0 0 0 3px accent-500/25` | Keyboard focus ring |

### Motion

| Token | Value |
|---|---|
| `ease-brand` | `cubic-bezier(0.16, 1, 0.3, 1)` (Vercel-style spring-out) |
| `animate-fade-in` | 0.2s ease-brand |
| `animate-slide-up` | 0.24s ease-brand |
| `animate-scale-in` | 0.22s ease-brand (modal entry) |
| `animate-shimmer` | 1.4s linear infinite (skeleton) |

Durations cap at 600ms. Bounce/elastic curves are forbidden.

`prefers-reduced-motion: reduce` kills every animation + transition
(set in `style.css` `@layer base`).

## Requirements

### Requirement: Typeface is exclusively Geist (with system fallbacks)

The system SHALL use Geist as the primary typeface in admin and portal
SPAs. Arial / system-ui / Inter MUST NOT be the active rendered face
when Geist is available.

#### Scenario: Geist loads via fontsource

- **WHEN** `main.ts` executes
- **THEN** `@fontsource/geist-sans` weights 400/500/600/700 SHALL be imported
- **AND** the `font-sans` Tailwind stack SHALL list `Geist` first

### Requirement: No magic font sizes in admin views

The system SHALL use named font-size tokens. Arbitrary `text-[Npx]`
classes SHALL NOT appear in files under `frontend/src/views/admin/` or
`frontend/src/components/layout/`.

#### Scenario: Code review check

- **WHEN** linting a PR that touches admin views
- **THEN** any new `text-[\d+px]` pattern SHALL be replaced with the
  matching scale token (e.g. `text-2xs` for 12px) before merge

### Requirement: Cards never nest visually

The system SHALL avoid card-on-card-on-card stacks. A bordered
`rounded-2xl` surface SHALL NOT contain another bordered `rounded-2xl`
surface unless visually separated by ≥16px gap.

#### Scenario: KPI strip on Status / Inbounds

- **WHEN** the page renders the KPI strip
- **THEN** each card SHALL sit directly on the page background (no wrapping card)
- **AND** the strip SHALL be a flat grid with `gap-3` or `gap-4`

### Requirement: Single accent color in normal states

The system SHALL NOT scatter rainbow color across KPI cards. The
`accent-*` palette is the only allowed coloring for KPI icon containers,
active sidebar items, primary CTAs (dark mode), and chart highlights.

#### Scenario: Status page KPI cards

- **WHEN** the Status page renders 4 KPI cards
- **THEN** each card's icon tile SHALL be `bg-accent-50 text-accent-600` (light) or `bg-accent-950/40 text-accent-300` (dark)
- **AND** semantic differentiation SHALL come from the icon glyph, not the background tint

### Requirement: Primary CTAs are ink-toned in light mode

The system SHALL render primary CTAs as `bg-ink-900` (cool near-black)
in light mode and `bg-accent-600` in dark mode (where ink would be
invisible).

#### Scenario: "Add node" button on Nodes page

- **WHEN** the page is in light mode
- **THEN** the button background SHALL be `bg-ink-900` with `hover:bg-ink-800`
- **AND** the text SHALL be white
- **AND** in dark mode, the same button SHALL be `bg-accent-600 hover:bg-accent-500`

### Requirement: Focus is always visible

The system SHALL render a 2px outlined focus ring on all keyboard-focused
interactive elements. The ring SHALL NOT be removed by `outline: none`.

#### Scenario: Tab through form

- **WHEN** the user presses Tab repeatedly to traverse inputs
- **THEN** each focused element SHALL show a 2px solid `accent-500` outline at 2px offset
- **AND** the rule SHALL be a global `:focus-visible` (defined in `style.css`)

### Requirement: prefers-reduced-motion is respected

The system SHALL disable all animations and transitions when the user
has `prefers-reduced-motion: reduce` set at the OS level.

#### Scenario: Reduced-motion user opens a modal

- **WHEN** the modal mounts and would normally `animate-scale-in`
- **THEN** the animation SHALL run with effectively 0 duration
- **AND** the final visual state SHALL be the same

## Out of scope

- OKLCH color expressions (deferred — Tailwind v3 has rough ergonomics
  for OKLCH+alpha; revisit when Tailwind v4 lands).
- Per-tenant white-labeling (operator can replace logo + brand name
  via env, but not the palette).
- Accessibility audit beyond focus rings + reduced motion (full WCAG
  2.2 contrast audit is a separate spec).
