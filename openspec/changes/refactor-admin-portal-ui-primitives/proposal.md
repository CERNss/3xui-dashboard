# refactor-admin-portal-ui-primitives

## Why

The dashboard-user-panel batch added card-mobile / table-desktop
double views across every admin surface (Users, Nodes, Inbounds,
Webhooks, Orders, Plans, ProvisioningPools) plus a portal-side
Profile rewrite. Pattern surface area grew faster than the
abstractions, so we now have:

- The same `<button class="settings-toggle">` switch hand-rolled ~5
  times (Users:1032, Nodes:304+459, Settings:604+625, Inbounds:807)
  with subtly drifting focus-ring / disabled / size styles.
- The `card-mobile / table-desktop` slot pair re-implemented in
  Users.vue and Nodes.vue at ~150 lines each, ~80% identical
  template.
- The toolbar header (search input + filter chip row + reload button)
  re-implemented in Users / Inbounds / AuditLog / Webhooks four
  times.
- Input/select Tailwind classes (`focus:ring-2 ring-accent-500/40
  dark:bg-surface-800/60 ...`) repeated raw across ~40+ form fields
  in Plans / ProvisioningPools / Settings / Inbounds.
- `REPO_URL = 'https://github.com/cern/3xui-dashboard'` hard-coded
  in both AdminLayout.vue and PortalLayout.vue.
- Filter-chip number badges in Inbounds (`{{ filtered.length }} /
  {{ data.inbounds.length }}`) bypass i18n number formatting.

None of this is incorrect today, but the churn from the next product
push (admin: client-ownership table, portal: balance log) will
multiply each of these by another 2-3x unless the primitives are
extracted first. Removing the duplication also makes the recent
ring-2/40 + dark-mode polish actually consistent — right now
hand-copies have already drifted (Settings.vue:1551 had `ring-4/20`
until the dashboard-user-panel batch).

## What Changes

This is a non-product cleanup. No new features, no API surface
changes, no migration. All commits should be diff-only refactors
that the screenshot-diff suite can sign off on.

### New shared primitives

- **`components/common/UiSwitch.vue`** — owns the visual states
  (on / off / disabled / loading), focus ring, ARIA `role="switch"`.
  Accepts `modelValue` + `loading` + `disabled` + `label` props.
  Emits `update:modelValue`.
- **`components/common/ResponsiveListTable.vue`** — accepts `rows`,
  `card` slot (mobile), `columns` config (desktop). Hides the right
  half via `md:` breakpoint, ARIA-wires the visible variant only.
- **`components/common/AdminToolbar.vue`** — search input slot +
  filter-chip slot + trailing-actions slot. Owns the focus ring,
  spacing, and i18n placeholder hookup.
- **`tailwind.css` components layer** — `.input-admin`,
  `.btn-admin-primary`, `.btn-admin-ghost`, `.filter-chip` for the
  ~40+ hand-styled form fields. Replaces inline class strings.

### Config / i18n cleanup

- **`branding` store** — surface `repoUrl` (env-backed, default
  `https://github.com/cern/3xui-dashboard`). AdminLayout +
  PortalLayout consume from store instead of a module-level const.
- **Filter-chip badges** — route through `$n(count)` so locales
  apply their own thousands separator.

### Migration plan per file

1. Users.vue, Nodes.vue → drop in `<ResponsiveListTable>`, switch to
   `<UiSwitch>`.
2. Inbounds.vue, Webhooks.vue, AuditLog.vue → `<AdminToolbar>`.
3. Plans.vue, ProvisioningPools.vue, Settings.vue forms → swap raw
   Tailwind class strings for the new utility classes.
4. Settings.vue + Inbounds.vue → `<UiSwitch>`.
5. AdminLayout.vue + PortalLayout.vue → branding-store `repoUrl`.

Each step is its own commit so the screenshot-diff suite can pass
incrementally and bisect cleanly if a regression slips through.

### Tests

- Add unit specs for `UiSwitch`, `ResponsiveListTable`,
  `AdminToolbar` covering: a11y attributes, slot rendering, basic
  emit + keyboard.
- Existing screen specs (Nodes.spec, Users.spec, Settings.spec,
  Webhooks.spec, Inbounds.spec) should remain green without
  modification — if they break, the new primitive's API is wrong.

## Out of scope

- No backend changes.
- No portal-side `<ResponsiveListTable>` migration in this batch
  (Subscription / Orders / Plans portal views) — separate change
  after the admin migration locks the API.
- No `<UiSwitch>` migration in legacy 3x-ui forked views — they're
  going to be re-skinned in a different change.
