## ADDED Requirements

### Requirement: OIDC Providers Listing Endpoint

The system SHALL expose a public endpoint that returns the operator's
configured OIDC identity provider(s) so the login UI can render a
"使用 X 登录" button. The endpoint SHALL be safe to call without
authentication and SHALL return an empty list (not 404) when OIDC is
not configured.

#### Scenario: OIDC configured with display name and icon

- **GIVEN** `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` are all set (i.e. `cfg.OIDC.Enabled() == true`)
- **AND** `OIDC_DISPLAY_NAME=集换社`
- **AND** `OIDC_ICON_URL=https://cdn.example.com/jhs.svg`
- **WHEN** a client GETs `/api/user/auth/oidc/providers`
- **THEN** the system SHALL respond HTTP 200
- **AND** the body SHALL be a JSON array with exactly one element: `{"name":"集换社","icon":"https://cdn.example.com/jhs.svg","login_url":"/api/user/auth/oidc/start"}`

#### Scenario: Display name fallback to issuer host

- **GIVEN** OIDC `Enabled() == true` but `OIDC_DISPLAY_NAME` is empty
- **AND** `OIDC_ISSUER=https://idp.example.com`
- **WHEN** providers is queried
- **THEN** the response element's `name` SHALL be `idp.example.com`

#### Scenario: Display name fallback to literal "OIDC"

- **GIVEN** `OIDC_DISPLAY_NAME` is empty AND `OIDC_ISSUER` fails URL parsing or has no host
- **WHEN** providers is queried
- **THEN** the response element's `name` SHALL be the literal `"OIDC"`

#### Scenario: Icon URL omitted when not set

- **GIVEN** `OIDC_ICON_URL` is empty
- **WHEN** providers is queried
- **THEN** the response element SHALL omit the `icon` key entirely (Go zero-value + `omitempty`)
- **AND** the frontend SHALL render a generic globe SVG in its place

#### Scenario: OIDC not configured

- **GIVEN** any of the four required OIDC env vars is empty (`cfg.OIDC.Enabled() == false`)
- **WHEN** providers is queried
- **THEN** the system SHALL respond HTTP 200 (NOT 404)
- **AND** the body SHALL be `[]`

### Requirement: Frontend Renders Providers Below Login Form

The login SPA SHALL fetch the providers list on mount and render one
labeled button per provider beneath the email/password form. The button
row SHALL appear only on the 登录 tab — never in 注册 mode.

#### Scenario: No providers — button row hidden

- **GIVEN** the providers endpoint returned `[]`
- **WHEN** the login view renders (login mode)
- **THEN** neither the divider ("或使用其他方式登录") nor any provider button SHALL appear

#### Scenario: Single provider — labeled button

- **GIVEN** the response contains `[{name:"集换社", icon:"…/jhs.svg", login_url:"/api/user/auth/oidc/start"}]`
- **WHEN** the login view renders in login mode
- **THEN** the page SHALL show a horizontal divider with text "或使用其他方式登录"
- **AND** one button with the provider's icon image and the literal label "使用 集换社 登录"
- **AND** clicking the button SHALL navigate the browser to `<login_url>?next=<current ?next or /portal>`

#### Scenario: Hidden in register mode

- **GIVEN** OIDC is configured and providers is non-empty
- **WHEN** the user clicks the 注册 tab
- **THEN** the OIDC button row SHALL be hidden
- **AND** SHALL re-appear when the user returns to the 登录 tab

#### Scenario: Endpoint error — fail-soft

- **WHEN** the providers fetch rejects (network, 404 on older backend, 5xx)
- **THEN** the SPA SHALL set the providers list to `[]`
- **AND** SHALL NOT surface a toast or error UI
- **AND** the login page SHALL render exactly as if OIDC were not configured

### Requirement: New OIDC UI-Hint Env Vars

The system SHALL accept two new optional environment variables for
operator-side branding of the OIDC button. Neither SHALL affect
`OIDC.Enabled()` — they are pure UI hints.

#### Scenario: OIDC_DISPLAY_NAME read on startup

- **WHEN** the operator sets `OIDC_DISPLAY_NAME=集换社` in `.env`
- **THEN** `cfg.OIDC.DisplayName` SHALL equal `"集换社"` after `config.Load()`
- **AND** the providers endpoint SHALL use that value as the button label

#### Scenario: OIDC_ICON_URL read on startup

- **WHEN** the operator sets `OIDC_ICON_URL=https://cdn.example.com/jhs.svg`
- **THEN** `cfg.OIDC.IconURL` SHALL equal that value
- **AND** the providers endpoint SHALL include it in the response
