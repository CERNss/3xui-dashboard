# frontend-rewrite-p7-cutover

P7 milestone. The single cutover: delete the Vue tree, rename
`frontend-react/` to `frontend/`, sweep Makefile/README/build
scripts, verify backend integration. After P7 the repo contains
exactly one frontend tree and the React app is the production
build.

**Entry criteria.** P6 (`frontend-rewrite-p6-tests`) is complete.
Full test pipeline is green on the React tree.

**Exit criteria.** Every requirement below holds. The OpenSpec
change `rewrite-frontend-react-antd` is then archivable.

## ADDED Requirements

### Requirement: Vue tree is deleted, React tree is renamed

The cutover SHALL be a single commit that removes `frontend/`
(the Vue tree) and renames `frontend-react/` to `frontend/`. The
Vue tree SHALL NOT be archived under any sibling path; it is
deleted outright per the pre-launch greenfield principle.

#### Scenario: Post-cutover tree-walk has no Vue files

- **GIVEN** the cutover commit has been merged
- **WHEN** the operator runs `find frontend -name '*.vue'`
- **THEN** the command SHALL produce no output
- **AND** `find frontend -name 'pinia*' -o -name '*.tsbuildinfo' -prune -o -path '*/node_modules' -prune` SHALL show no Pinia source files

#### Scenario: package.json has React deps only

- **GIVEN** the cutover commit has been merged
- **WHEN** the operator reads `frontend/package.json`
- **THEN** `vue`, `vue-router`, `vue-i18n`, `pinia`, `@vueuse/core`,
  `@vitejs/plugin-vue`, `@vue/test-utils`, `eslint-plugin-vue`,
  and `vue-tsc` SHALL NOT appear in either dependencies block
- **AND** `react`, `react-dom`, `antd`, `react-router-dom`,
  `zustand`, `@tanstack/react-query`, `react-i18next`, `i18next`,
  `@vitejs/plugin-react` SHALL appear

### Requirement: Backend `go:embed` continues to work without code change

The cutover SHALL preserve the backend's frontend-asset embed
contract. No Go file under `backend/` SHALL be modified by the
cutover commit; only the bundled assets at
`backend/internal/web/dist/` change (because the React build
overwrites them).

#### Scenario: Backend rebuild serves the React app

- **GIVEN** the cutover commit has been merged
- **WHEN** the operator runs `make build` (top-level) and starts
  the binary
- **THEN** `http://localhost:8080/` SHALL return HTML that loads
  the React bundle (recognizable by the `<div id="root">` mount
  point and the `vendor-react.*.js` script tag)
- **AND** no source file under `backend/` SHALL be touched by
  the cutover commit

#### Scenario: No `go:embed` directive changes

- **GIVEN** the cutover commit's diff
- **WHEN** the operator runs `git show <cutover-commit> -- 'backend/**/*.go'`
- **THEN** the output SHALL be empty

### Requirement: Makefile sweep removes parallel-tree targets

The post-cutover `Makefile` SHALL have a single `dev-frontend` /
`build-frontend` pair pointing at the renamed React tree. The
P0-introduced `dev-frontend-react` / `build-frontend-react`
targets SHALL be removed (they're now redundant). `make dev` and
`make build` SHALL continue to work transparently.

#### Scenario: Legacy parallel targets removed

- **GIVEN** the cutover sweep commit has been merged
- **WHEN** the operator runs `make -p 2>/dev/null | grep -E '^(dev|build)-frontend'`
- **THEN** the output SHALL list exactly `dev-frontend` and
  `build-frontend`
- **AND** SHALL NOT list `dev-frontend-react` or `build-frontend-react`

#### Scenario: `make dev` and `make build` work post-cutover

- **GIVEN** the cutover and sweep commits have both merged
- **WHEN** the operator runs `make build` from a clean checkout
- **THEN** the command SHALL produce `backend/internal/web/dist/`
  populated with React bundle output
- **AND** SHALL build the Go binary against that dist

### Requirement: Documentation references the React stack

The cutover sweep SHALL update `README.md`, any `docs/*.md` file that mentions the frontend stack, and `deploy/*` scripts to describe React + AntD as the frontend; Vue / Pinia / Tailwind-only references MUST be removed from descriptions of the current stack (historical context in changelogs MAY remain).

#### Scenario: README describes React + AntD

- **GIVEN** the cutover sweep has merged
- **WHEN** the operator opens `README.md`
- **THEN** the tech-stack section SHALL name React, AntD,
  Zustand, TanStack Query, react-i18next, Vite
- **AND** SHALL NOT name Vue, Pinia, vue-router, vue-i18n as the
  *current* stack

#### Scenario: Build/deploy docs reflect the rename

- **GIVEN** the cutover sweep has merged
- **WHEN** the operator greps `docs/` and `deploy/` for
  `frontend-react`
- **THEN** the only matches SHALL be in archived changelogs or
  this OpenSpec change itself
- **AND** no live build script SHALL reference `frontend-react`

### Requirement: OpenSpec change is archived

The OpenSpec change `rewrite-frontend-react-antd` SHALL be archived after the cutover sweep merges, so the `openspec/specs/` tree reflects the new platform contract going forward.

#### Scenario: Archive command succeeds

- **GIVEN** the cutover and sweep commits are merged
- **WHEN** the operator runs
  `openspec archive rewrite-frontend-react-antd`
- **THEN** the command SHALL move the change into the archived
  tree
- **AND** `openspec/specs/frontend-platform-react/spec.md` SHALL
  be present as the new canonical platform spec

#### Scenario: Milestone specs do NOT survive archival as live capabilities

- **GIVEN** the change is archived
- **WHEN** the operator lists `openspec/specs/`
- **THEN** the platform spec `frontend-platform-react` SHALL be
  present
- **AND** the seven milestone specs (`frontend-rewrite-p0-*` …
  `frontend-rewrite-p7-*`) SHALL NOT be present as live
  capabilities — they were rewrite-phase contracts and are no
  longer load-bearing after cutover
