# oidc-providers

Exposes the operator's configured OIDC identity provider so the login
page can render a "使用 <name> 登录" button at the bottom of the form.

## Purpose & boundaries

In v1 OIDC start/callback are 501 stubs — actual federation isn't
wired yet. This module covers ONLY the **discovery surface** that lets
the SPA decide whether to render the OIDC button row at all, and what
label/icon to show. When OIDC is properly configured, the same endpoint
will already be returning useful data, so adding real flow later is a
no-op for the UI.

## Configuration

New env vars (added to `config.OIDC`):

| Var | Purpose | Required for button to render |
|---|---|---|
| `OIDC_ISSUER` | Discovery base URL | Yes (already used by future flow) |
| `OIDC_CLIENT_ID` | OIDC client | Yes |
| `OIDC_CLIENT_SECRET` | OIDC secret | Yes |
| `OIDC_REDIRECT_URL` | Callback URL | Yes |
| `OIDC_DISPLAY_NAME` | Human label, e.g. "集换社" | No — falls back to issuer hostname |
| `OIDC_ICON_URL` | Icon shown on the button (URL or SVG path) | No — frontend falls back to generic globe |

`config.OIDC.Enabled() == (Issuer && ClientID && ClientSecret && RedirectURL)` — same as before; the new fields are pure UI hints.

## Endpoint

`GET /api/user/auth/oidc/providers` — public (no auth gate).

Response: JSON array. Empty array (not 404) when OIDC isn't configured.

```json
[
  {
    "name": "集换社",
    "icon": "https://cdn.example.com/jihuanshe.svg",
    "login_url": "/api/user/auth/oidc/start"
  }
]
```

Future expansion: multiple providers SHALL be permitted in the response
shape, even though the env vars only express one today. Frontend already
iterates `v-for`.

## Requirements

### Requirement: Providers endpoint reflects config

The system SHALL provide a public `GET /api/user/auth/oidc/providers`
endpoint that returns the configured OIDC provider as a single-element
array, or an empty array when OIDC isn't configured.

#### Scenario: OIDC fully configured with display name

- **GIVEN** `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` are all set
- **AND** `OIDC_DISPLAY_NAME=集换社`, `OIDC_ICON_URL=https://cdn/jhs.svg`
- **WHEN** a client GETs `/api/user/auth/oidc/providers`
- **THEN** the response SHALL be a JSON array with exactly one element:
  `{"name":"集换社","icon":"https://cdn/jhs.svg","login_url":"/api/user/auth/oidc/start"}`

#### Scenario: OIDC configured but display name missing

- **GIVEN** OIDC is `Enabled()` but `OIDC_DISPLAY_NAME` is empty
- **WHEN** providers is queried
- **THEN** `name` SHALL fall back to `url.Parse(Issuer).Host`, e.g. `idp.example.com`
- **AND** SHALL fall back to the literal `"OIDC"` if the issuer URL fails to parse

#### Scenario: OIDC not configured

- **GIVEN** any of the four required env vars is empty
- **WHEN** providers is queried
- **THEN** the response SHALL be `[]`
- **AND** the response status SHALL be `200 OK` (NOT 404 — the empty list is the truthful answer)

#### Scenario: Icon URL absent

- **GIVEN** `OIDC_ICON_URL` is empty
- **WHEN** providers is queried
- **THEN** the provider object SHALL omit the `icon` field (Go zero-value + `omitempty`)
- **AND** the frontend SHALL render a generic globe SVG in its place

### Requirement: Frontend renders providers below the form

The login page SHALL, on mount, fetch providers and render one button
per provider beneath the email/password form, separated by a divider.

#### Scenario: No providers — section hidden

- **GIVEN** `oidcProviders()` returned `[]`
- **WHEN** the login view renders
- **THEN** neither the divider ("或使用其他方式登录") nor any provider button SHALL appear

#### Scenario: Single provider — button rendered

- **GIVEN** providers returned `[{name:"集换社", login_url:"/api/user/auth/oidc/start"}]`
- **WHEN** the login view renders (login mode, not register)
- **THEN** the section SHALL show:
  - A horizontal divider with the text "或使用其他方式登录"
  - One button with the provider icon on the left and the literal label "使用 集换社 登录"
- **AND** clicking the button SHALL navigate the browser to
  `<login_url>?next=<current ?next or /portal>`

#### Scenario: Multiple providers (future-proofing)

- **GIVEN** the response array has more than one element
- **WHEN** the login view renders
- **THEN** each provider SHALL render as a separate button in array order
- **AND** the divider SHALL appear exactly once, above the first button

### Requirement: OIDC buttons hidden in register mode

The OIDC providers row SHALL only appear while the user is on the 登录 tab.

#### Scenario: User switches to register

- **GIVEN** OIDC is configured
- **WHEN** the user clicks the 注册 tab
- **THEN** the OIDC button row SHALL be hidden
- **AND** SHALL re-appear when they switch back to 登录

### Requirement: Providers endpoint is fail-soft on the client

The frontend SHALL treat any error fetching providers as "no providers",
so a backend without the route (older deploy) doesn't break the login page.

#### Scenario: Endpoint returns 404 on old deploy

- **WHEN** `portalAuthApi.oidcProviders()` rejects with a 404
- **THEN** the SPA SHALL set `oidcProviders.value = []` (no error toast)
- **AND** the login page SHALL render exactly as if OIDC were not configured

## Implementation notes

- `internal/handler/user/auth.go::OIDCProviders` is the handler; route
  is `GET` (not POST) so it can be cached / preflighted cleanly.
- The handler imports `internal/config` for `config.OIDC` and stays
  decoupled from the future `oidc` service (login flow). When OIDC
  actually lands, the `LoginURL` field can stay pointing at
  `/api/user/auth/oidc/start` since that's where the flow begins.
- TypeScript shape: `OIDCProvider` in `frontend/src/api/portal/auth.ts`.
- Login view mount hook: `onMounted(loadOIDC)` in `views/Login.vue`.

## Out of scope

- The OIDC start / callback / token-exchange flow itself (v1 stubs).
- PKCE, nonce, state parameter handling.
- Account-linking (creating a portal user when an OIDC subject doesn't
  match an existing email).
