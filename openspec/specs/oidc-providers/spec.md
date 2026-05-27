# oidc-providers

Exposes the operator's configured OIDC identity providers so the login
page can render SSO buttons and start the Authorization Code + PKCE
flow.

## Purpose & boundaries

OIDC is an active login surface. This module covers provider discovery,
start/callback, and account completion for callback results that cannot
be resolved automatically.

## Configuration

OIDC can be configured by environment variables and by DB-backed settings:

| Var | Purpose | Required for button to render |
|---|---|---|
| `OIDC_ISSUER` | Discovery base URL | Yes |
| `OIDC_CLIENT_ID` | OIDC client | Yes |
| `OIDC_CLIENT_SECRET` | OIDC secret | Yes |
| `OIDC_REDIRECT_URL` | Callback URL | Yes |
| `OIDC_DISPLAY_NAME` | Human label, e.g. "集换社" | No — falls back to issuer hostname |
| `OIDC_ICON_URL` | Icon shown on the button (URL or SVG path) | No — frontend falls back to generic globe |

`config.OIDC.Enabled() == (Issuer && ClientID && ClientSecret && RedirectURL)`.
Settings-backed values override or complement the environment where the OIDC
service reads them. The env/runtime provider is exposed as provider key
`default`.

## Endpoint

`GET /api/user/auth/oidc/providers` is public (no auth gate).
Response: JSON array. Empty array (not 404) when OIDC isn't configured.

```json
[
  {
    "key": "default",
    "name": "集换社",
    "icon": "https://cdn.example.com/jihuanshe.svg",
    "start_url": "/api/user/auth/oidc/start",
    "login_url": ""
  }
]
```

Multiple providers SHALL be permitted in the response shape. The frontend
starts a provider by POSTing to `/api/user/auth/oidc/start` with the returned
`key`, then navigates to the returned `authorize_url`.

## Requirements

### Requirement: Providers endpoint reflects config

The system SHALL provide a public `GET /api/user/auth/oidc/providers`
endpoint that returns enabled OIDC providers, or an empty array when
OIDC isn't configured.

#### Scenario: OIDC fully configured with display name

- **GIVEN** `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` are all set
- **AND** `OIDC_DISPLAY_NAME=集换社`, `OIDC_ICON_URL=https://cdn/jhs.svg`
- **WHEN** a client GETs `/api/user/auth/oidc/providers`
- **THEN** the response SHALL be a JSON array with exactly one element:
  `{"key":"default","name":"集换社","icon":"https://cdn/jhs.svg","start_url":"/api/user/auth/oidc/start","login_url":""}`

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
- **THEN** the provider object SHALL omit the `icon` field
- **AND** the frontend SHALL render a generic globe SVG in its place

### Requirement: Frontend renders providers below the form

The login page SHALL, on mount, fetch providers and render one button
per provider beneath the admin login form, separated by a divider.

#### Scenario: No providers — section hidden

- **GIVEN** `oidcProviders()` returned `[]`
- **WHEN** the login view renders
- **THEN** neither the divider ("或使用其他方式登录") nor any provider button SHALL appear

#### Scenario: Single provider — button rendered

- **GIVEN** providers returned `[{key:"default", name:"集换社", start_url:"/api/user/auth/oidc/start"}]`
- **WHEN** the login view renders
- **THEN** the section SHALL show:
  - A horizontal divider with the SSO label
  - One button labelled with the provider name
- **AND** clicking the button SHALL call `POST /api/user/auth/oidc/start`
  with `{provider_key:"default", redirect_after:<safe portal next path>}`
- **AND** the browser SHALL navigate to the returned `authorize_url`

#### Scenario: SSO defaults to portal redirect

- **GIVEN** the login view is opened without a portal `next` path
- **WHEN** a user starts OIDC from the shared login page
- **THEN** `redirect_after` SHALL default to `/portal/subscription`
- **AND** admin-only `next` paths SHALL NOT be passed into OIDC start

#### Scenario: Multiple providers

- **GIVEN** the response array has more than one element
- **WHEN** the login view renders
- **THEN** each provider SHALL render as a separate button in array order
- **AND** the divider SHALL appear exactly once, above the first button

### Requirement: Providers Endpoint Is Fail-Soft On The Client

The frontend SHALL treat any error fetching providers as "no providers",
so a transient backend/config error does not break the login page.

#### Scenario: Provider fetch rejects

- **WHEN** `portalAuthApi.oidcProviders()` rejects
- **THEN** the SPA SHALL set `oidcProviders.value = []` (no error toast)
- **AND** the login page SHALL render exactly as if OIDC were not configured

### Requirement: OIDC start and callback flow

The system SHALL support standard OIDC login through Authorization Code
flow with PKCE.

#### Scenario: Start returns authorize URL

- **WHEN** a client POSTs `/api/user/auth/oidc/start` with optional `provider_key` and `redirect_after`
- **THEN** the system SHALL generate `state` and PKCE verifier data, store them in the short-lived session store, and return `authorize_url`

#### Scenario: Callback returns a token for linked identities

- **WHEN** the frontend POSTs `code` and `state` to `/api/user/auth/oidc/callback`
- **THEN** the system SHALL exchange the code for tokens, validate the ID token signature, issuer, audience, and expiration, and read subject/email claims
- **AND** if the provider subject is already linked to a local user, the response SHALL be a user-audience JWT

#### Scenario: DB-backed provider callback does not require default OIDC

- **GIVEN** a provider row exists in `oidc_providers` and the env/runtime `default` provider is disabled
- **WHEN** callback state identifies that provider key
- **THEN** the callback SHALL resolve issuer/client/secret/redirect values from the provider row
- **AND** the token exchange SHALL complete without requiring default OIDC config

#### Scenario: Callback returns pending account completion

- **WHEN** the provider subject is not linked to a local user
- **THEN** the callback SHALL return `status:"pending"` with a short-lived `pending_token`, provider metadata, provider email, and existing-user metadata
- **AND** the system SHALL NOT expose `POST /api/user/auth/oidc/resolve`

#### Scenario: Bind existing requires local password

- **WHEN** the pending provider email belongs to an existing local account
- **THEN** completion SHALL use `POST /api/user/auth/oidc/bind-existing`
- **AND** the request SHALL include the existing local account password before the OIDC identity is linked or a JWT is issued

#### Scenario: Create account requires verified local email

- **WHEN** the user chooses to create a new local account from a pending OIDC callback
- **THEN** completion SHALL use `POST /api/user/auth/oidc/create-account`
- **AND** the handler SHALL require a display name, password, and email-verification token for `oidc_create_account`
- **AND** the service SHALL reject account creation when public registration is disabled

## Implementation notes

- `internal/handler/user/auth.go::OIDCProviders` is the handler; route
  is `GET` (not POST) so it can be cached / preflighted cleanly.
- TypeScript shape: `OIDCProvider` in `frontend/src/api/portal/auth.ts`.
- Login view hook: `useOidcStart` in `frontend/src/views/Login.tsx`.

## Out of scope

- Passwordless pending resolution.
- Recreate/reset of an existing email identity from OIDC callback.
- OIDC unlink actions in the portal profile.
