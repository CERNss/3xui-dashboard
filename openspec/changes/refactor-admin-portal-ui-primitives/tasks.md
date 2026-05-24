# Tasks — refactor-admin-portal-ui-primitives

Each task is a self-contained commit. Run `npx vitest run` + visual
diff after every commit; do not batch.

## 1. UiSwitch primitive

- [ ] 1.1 Author `frontend/src/components/common/UiSwitch.vue` with
  props `modelValue: boolean`, `loading?: boolean`, `disabled?: boolean`,
  `label?: string`, `ariaDescribedby?: string`. Emits
  `update:modelValue`. Owns the bg / focus-ring / disabled-opacity
  styling that is currently hand-copied.
- [ ] 1.2 Spec `UiSwitch.spec.ts`: emits on click, no emit while
  `loading`, `aria-checked` reflects state, `role="switch"`.
- [ ] 1.3 Migrate call sites one commit at a time:
  - `views/admin/Users.vue:1032-63`
  - `views/admin/Nodes.vue:304-23, 459-75`
  - `views/admin/Settings.vue` (all `.settings-toggle` buttons)
  - `views/admin/Inbounds.vue:807`
- [ ] 1.4 Delete `.settings-switch` / `.settings-toggle` CSS once
  Settings is migrated.

## 2. ResponsiveListTable primitive

- [ ] 2.1 Author `frontend/src/components/common/ResponsiveListTable.vue`
  with named slots `card` (mobile, default), `header` (desktop
  thead), `row` (desktop tbody tr). Props: `rows`, `empty?`,
  `loading?`, `mobileBreakpoint?` (default `md`).
- [ ] 2.2 Spec asserts ARIA-correct table on desktop, card list on
  mobile (via `matchMedia` mock).
- [ ] 2.3 Migrate `views/admin/Users.vue`. Verify
  `views/admin/Users.spec.ts` still passes unchanged.
- [ ] 2.4 Migrate `views/admin/Nodes.vue`. Verify
  `views/admin/Nodes.spec.ts` still passes unchanged.

## 3. AdminToolbar primitive

- [ ] 3.1 Author `frontend/src/components/common/AdminToolbar.vue`.
  Slots: `search`, `filters`, `actions`. Owns layout +
  border-bottom + padding spacing.
- [ ] 3.2 Spec covers slot rendering + sticky / shadow on scroll.
- [ ] 3.3 Migrate `views/admin/Users.vue`, `Inbounds.vue`,
  `AuditLog.vue`, `Webhooks.vue` one commit each.

## 4. Tailwind components layer

- [ ] 4.1 In `frontend/src/style.css` (or wherever the app-level
  Tailwind layer lives) add `@layer components`:
  - `.input-admin` — full focus-ring + dark-mode field style
  - `.input-admin-mono` — same, monospace variant for ports/IDs
  - `.btn-admin-primary` — accent CTA
  - `.btn-admin-ghost` — secondary border-only
  - `.filter-chip` — toolbar filter pill
- [ ] 4.2 Migrate `Plans.vue` form fields (one commit).
- [ ] 4.3 Migrate `ProvisioningPools.vue` form fields (one commit).
- [ ] 4.4 Migrate `Settings.vue` `.settings-input` /
  `.settings-secondary-button` → new classes; drop the old
  scoped utilities.

## 5. Branding store: repoUrl

- [ ] 5.1 Add `repoUrl` to `stores/branding.ts`, env-backed
  (`VITE_REPO_URL`) with the existing default.
- [ ] 5.2 `AdminLayout.vue` + `PortalLayout.vue` consume from store.
  Drop the module-level `REPO_URL` const.

## 6. i18n number formatting

- [ ] 6.1 Filter-chip count badges in `Inbounds.vue` use `$n(count)`
  with the active locale.
- [ ] 6.2 Audit `Users.vue` / `Nodes.vue` / `AuditLog.vue` for
  raw number rendering inside `{{ }}` interpolations — wrap with
  `$n()` where it represents a localizable count.

## 7. Cleanup pass

- [ ] 7.1 Grep for `focus:ring-2 focus:ring-accent-500/40` —
  confirm zero raw occurrences remain outside the new utility
  classes.
- [ ] 7.2 Grep for `relative inline-flex h-` — confirm no hand-rolled
  toggles remain.
- [ ] 7.3 Run full `npx vitest run`; all existing specs unchanged.
- [ ] 7.4 Build + visual-diff: dashboard / admin / portal screenshots
  at desktop + mobile breakpoints, light + dark.
