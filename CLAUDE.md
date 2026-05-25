# Project notes for Claude Code sessions

## Active rewrite window: Vue → React + AntD

The frontend is mid-rewrite. The plan lives in
`docs/frontend-rewrite.md`; the formal spec is the OpenSpec
change `rewrite-frontend-react-antd` (`openspec/changes/`).

**While the rewrite window is open, the Vue tree
(`frontend/`) is on a strict diet:**

- **Allowed:** bugfixes, security patches, dependency security
  bumps.
- **Forbidden:** new views, new features, new endpoints that
  the Vue tree wires up first.
- **Allowed but with cost:** if a new feature is urgent enough
  to ship before cutover, the same PR ports it to the React
  tree under `frontend-react/`. No Vue-only features merge.

This policy is enforced socially (one author, one main branch).
If it breaks down, every Vue-only feature becomes catch-up work
in the React tree and the cutover date slides. See design.md
D12 for the long form.

**For Claude in particular:** if a user asks you to add a new
feature, view, or endpoint to `frontend/` during the rewrite
window, push back and ask whether it should live in
`frontend-react/` instead. Bugfix-shaped changes (one-line
fixes, dependency bumps, lint cleanup) are fine.

**To check if we're still in the window:** look for the
`frontend-react/` directory at the repo root. If it exists and
`frontend/` still has `*.vue` files, the window is open. After
cutover (P7 of the OpenSpec change), `frontend/` will be the
React tree and this section becomes historical context.

## Where the design intent lives

- `docs/frontend-rewrite.md` — narrative pitch + design summary
  for the React rewrite.
- `openspec/changes/rewrite-frontend-react-antd/` — formal spec
  (proposal / design / 9 capability specs / tasks).
- `docs/3xui-node-reference.md` — node-side API + data-shape
  reference. Consult before touching any node-facing code.
- `docs/operator/` — runbook-style docs (OIDC setup,
  WireGuard, node contract).

## Reference projects

- `/Users/cern/LocalDisk/D/Repo/infra/cern-3x-ui` — the
  upstream 3x-ui fork. Reference, not dependency. Don't import
  from it; extract concepts and re-implement.
- `/Users/cern/LocalDisk/D/Repo/infra/cern-sub2api` — sibling
  project, similar code style/layout conventions.

## Stack at a glance

- Backend: Go 1.26, Gin, GORM, PostgreSQL, JWT.
- Frontend (current, Vue tree): Vue 3 + TypeScript + Vite +
  Tailwind + Pinia + vue-router.
- Frontend (target, after cutover): React 18 + TypeScript +
  Vite + AntD 5 + Zustand + TanStack Query + react-i18next.
- Frontend is embedded into the Go binary via `go:embed dist`
  and served as an SPA — same contract before and after
  cutover.
