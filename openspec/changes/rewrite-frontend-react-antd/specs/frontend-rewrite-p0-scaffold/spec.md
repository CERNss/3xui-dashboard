# frontend-rewrite-p0-scaffold

P0 milestone of the Vue → React/AntD rewrite. Defines the
acceptance contract for the bare scaffold of `frontend-react/`:
build/dev pipelines work, a hello-world page renders with brand
chrome, and zero impact on the live Vue tree until cutover.

This spec is **acceptance-only**. The HOW (which files to create,
which scripts to write) lives in `tasks.md` section 1; the WHY
(parallel rewrite, AntD theme tokens, port allocation) lives in
`design.md` (D2, D5, D9). Use this spec to know when P0 is done.

**Entry criteria.** Approved `proposal.md`, `design.md`, and
`specs/frontend-platform-react/spec.md` exist. No file under
`frontend/` is modified.

**Exit criteria.** Every requirement below has its WHEN/THEN
scenario observably true on a fresh checkout. P0 is then complete
and P1 unblocks.

## ADDED Requirements

### Requirement: Parallel `frontend-react/` directory exists

The repository SHALL contain a `frontend-react/` directory at the
repo root, populated with a Vite + React + TypeScript scaffold,
without modifying any file under the existing `frontend/` Vue
tree.

#### Scenario: Fresh checkout has both trees

- **GIVEN** a fresh clone of the repository after P0 lands
- **WHEN** the operator runs `ls -d frontend frontend-react`
- **THEN** both directories SHALL exist
- **AND** `frontend/package.json` SHALL still list `vue` in its
  dependencies (the Vue tree is untouched)
- **AND** `frontend-react/package.json` SHALL list `react`,
  `react-dom`, `antd`, `react-router-dom`, `zustand`,
  `@tanstack/react-query`, `react-i18next`, and `i18next` in its
  dependencies

#### Scenario: Vue tree's working tree is untouched

- **GIVEN** P0 is implemented in a single commit
- **WHEN** the operator runs `git show --stat <p0-commit> -- frontend/`
- **THEN** the output SHALL be empty (no file under `frontend/`
  was added, modified, or deleted by P0)

### Requirement: `npm install && npm run dev` boots the dev server on port 5174

The React tree SHALL run its dev server on port 5174 so it can
coexist with the Vue tree's port 5173 during the rewrite window.
The dev server SHALL proxy `/api`, `/sub`, and `/uploads` to the
backend on `localhost:8080`, matching the Vue tree's proxy.

#### Scenario: Dev server boots and serves a hello page

- **GIVEN** the operator has run `npm install` inside
  `frontend-react/`
- **WHEN** the operator runs `npm run dev`
- **THEN** Vite SHALL listen on `http://localhost:5174`
- **AND** a GET to `/` SHALL return HTML that loads the React
  bundle
- **AND** the rendered page SHALL show a placeholder title that
  identifies the tree (so an operator opening both 5173 and 5174
  can tell them apart)

#### Scenario: Dev proxy forwards API calls

- **GIVEN** the backend is running on `localhost:8080`
- **AND** the React dev server is running on `localhost:5174`
- **WHEN** the browser issues `GET http://localhost:5174/api/public/branding`
- **THEN** Vite SHALL proxy to `http://localhost:8080/api/public/branding`
- **AND** the same SHALL hold for any path under `/sub` or `/uploads`

#### Scenario: Both dev servers run side-by-side

- **GIVEN** `make dev-frontend` (Vue, port 5173) is already running
- **WHEN** the operator runs the equivalent React command in
  another terminal
- **THEN** the React dev server SHALL bind successfully on 5174
- **AND** neither dev server SHALL crash or steal the other's port

### Requirement: `npm run build` produces the backend embed contract

The production build SHALL emit assets at
`backend/internal/web/dist/` so the Go `go:embed dist` directive
in `backend/internal/web/embed.go` can consume them without code
change. The build SHALL also produce vendor chunks separately so
the initial bundle does not include all of AntD on one request.

#### Scenario: Fresh build produces dist with index.html and assets

- **GIVEN** the operator has run `npm install` inside
  `frontend-react/`
- **WHEN** the operator runs `npm run build`
- **THEN** the command SHALL exit with code 0
- **AND** `backend/internal/web/dist/index.html` SHALL exist
- **AND** `backend/internal/web/dist/assets/` SHALL contain at
  least one `*.js` file and one `*.css` file with content-hash
  filenames

#### Scenario: Vendor chunks are split by library

- **GIVEN** a successful build
- **WHEN** the operator inspects the filenames under
  `backend/internal/web/dist/assets/`
- **THEN** there SHALL be a separate chunk for React
  (`vendor-react.*.js`)
- **AND** a separate chunk for AntD (`vendor-antd.*.js`)
- **AND** a separate chunk for TanStack Query (`vendor-query.*.js`)

#### Scenario: Backend rebuild picks up the React bundle

- **GIVEN** the React build has just produced `dist/`
- **WHEN** the operator runs `make build-backend` and starts the
  binary
- **THEN** opening `http://localhost:8080/` SHALL return the React
  hello-world page (not the Vue page) — i.e. the embed pipeline
  works end-to-end with the React output

### Requirement: AntD theme renders brand-colored primary affordances

The hello-world page SHALL be wrapped in a single
`<ConfigProvider>` carrying the brand-derived theme tokens. A
visible primary AntD `<Button type="primary">` SHALL be present
on the page so the brand color is observable.

#### Scenario: Primary button uses the brand indigo

- **GIVEN** the dev server is running
- **WHEN** the operator opens `http://localhost:5174/`
- **THEN** the page SHALL render an AntD `<Button type="primary">`
- **AND** the button's computed background color SHALL be the
  brand primary `#4f46e5` (Tailwind `primary-600`), NOT AntD's
  default `#1677ff` blue

#### Scenario: Geist Sans is the default UI font

- **GIVEN** the dev server is running and `@fontsource/geist-sans`
  has loaded
- **WHEN** the operator inspects the page's body element in the
  browser devtools
- **THEN** `getComputedStyle(document.body).fontFamily` SHALL begin
  with `"Geist"` (matching the Vue tree's font choice)

### Requirement: Typecheck, lint, and test scripts all pass on an empty scaffold

The scaffold SHALL ship the same script vocabulary as the Vue
tree (`typecheck`, `lint`, `test`, `dev`, `build`, `preview`) so
existing CI / Makefile patterns translate without surprise. All
scripts SHALL succeed on the empty hello-world scaffold.

#### Scenario: `npm run typecheck` passes

- **GIVEN** a fresh `npm install` has run
- **WHEN** the operator runs `npm run typecheck`
- **THEN** the command SHALL exit with code 0 and emit no
  diagnostics

#### Scenario: `npm run lint` passes

- **GIVEN** a fresh `npm install` has run
- **WHEN** the operator runs `npm run lint`
- **THEN** the command SHALL exit with code 0

#### Scenario: `npm run test` runs with zero specs and exits clean

- **GIVEN** the scaffold has not yet ported any view-level spec
- **WHEN** the operator runs `npm run test`
- **THEN** vitest SHALL exit with code 0
- **AND** the output SHALL report "0 tests" or equivalent (no
  failure, no hang)

### Requirement: Root Makefile exposes parallel-tree build/dev targets

The root `Makefile` SHALL gain two new targets — `dev-frontend-react`
and `build-frontend-react` — that operate on the new tree, without
removing or modifying the existing `dev-frontend` / `build-frontend`
targets that operate on the Vue tree. The default `make dev` and
`make build` SHALL continue to drive the Vue tree until P7
cutover.

#### Scenario: New targets are reachable from `make`

- **WHEN** the operator runs `make -n dev-frontend-react`
- **THEN** the dry-run output SHALL include
  `cd .../frontend-react && npm install` and `npm run dev`
- **AND** `make -n build-frontend-react` SHALL include the
  matching `npm run build` invocation

#### Scenario: Legacy Vue targets unchanged

- **WHEN** the operator runs `make -n dev-frontend`
- **THEN** the dry-run output SHALL still reference
  `cd .../frontend && npm install` and `npm run dev`
- **AND** `make -n build` SHALL continue to call `build-frontend`
  (Vue), not `build-frontend-react`
