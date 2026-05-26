# Project notes for Claude Code sessions

## React frontend is live

The frontend rewrite has cut over. `frontend/` is now the single
production frontend tree: React 18 + TypeScript + Vite + AntD 5,
with Zustand for client state, TanStack Query for server state,
React Router for routing, and react-i18next/i18next for locale
handling.

The former rewrite plan lives in `docs/frontend-rewrite.md`; the
formal OpenSpec change was `rewrite-frontend-react-antd`
(`openspec/changes/`) until archival. Treat those artifacts as
historical context unless the user is explicitly asking about the
rewrite itself.

For new frontend work, edit `frontend/`. There is no parallel
frontend tree after cutover.

## Where the design intent lives

- `docs/frontend-rewrite.md` — historical narrative pitch +
  design summary for the React rewrite.
- `openspec/changes/rewrite-frontend-react-antd/` — formal
  rewrite spec until it is archived.
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
- Frontend: React 18 + TypeScript + Vite + AntD 5 + Zustand +
  TanStack Query + React Router + react-i18next.
- Frontend is embedded into the Go binary via `go:embed dist`
  and served as an SPA — same contract before and after
  cutover.
