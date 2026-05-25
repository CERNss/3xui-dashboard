# frontend-rewrite-p5-portal-views

P5 milestone. Ports the 5 portal views + `AlipayPayModal` to
React, completing end-user-facing surfaces.

**Entry criteria.** P3 (auth surface) is complete and `PortalLayout`
from P2 is wired. P4 (admin) may be in-flight; P5 does not depend
on it.

**Exit criteria.** Every requirement below holds. An end user can
log into the portal, view their subscription/usage, browse and
purchase a plan, see their orders, and edit their profile.

## ADDED Requirements

### Requirement: Subscription view exposes seven subscription formats

`Subscription.tsx` SHALL render seven format options matching the Vue tree's `portal/Subscription.vue` shape: `base64`, `clash`, `singbox`, `sip008`, `wireguard`, `wireguard-zip`, `json`. `base64` is the default and the URL has no `?format=` query; the other six append `?format=<key>`. `wireguard-zip` is `downloadOnly` — its UI MUST suppress the copy button and the QR code (it's a binary attachment, not a URL the user pastes).

#### Scenario: Base64 URL has no format query

- **GIVEN** the active format is `base64`
- **WHEN** the page computes the subscription URL
- **THEN** the URL SHALL be
  `<origin>/sub/<sub_id>` with no `?format=` query

#### Scenario: Switching to a non-default format appends `?format=`

- **GIVEN** the active format is changed to `clash`
- **WHEN** the page recomputes the URL
- **THEN** the URL SHALL be
  `<origin>/sub/<sub_id>?format=clash`
- **AND** the same pattern SHALL hold for `singbox`, `sip008`,
  `wireguard`, `wireguard-zip`, `json`

#### Scenario: wireguard-zip suppresses copy and QR

- **GIVEN** the active format is `wireguard-zip`
- **WHEN** the page renders
- **THEN** the copy-to-clipboard button SHALL be hidden (or
  disabled with a tooltip explaining why)
- **AND** the QR canvas SHALL be hidden
- **AND** a "Download ZIP" button SHALL be visible that issues
  a `GET` to the subscription URL and triggers a file save

#### Scenario: QR matches the displayed URL for non-zip formats

- **GIVEN** the user is on `/portal/subscription` with any
  non-zip format active
- **WHEN** the page renders
- **THEN** the QR SHALL encode exactly the URL string shown in
  the text field
- **AND** changing the active format SHALL regenerate the QR
  to match

#### Scenario: QR regeneration is race-safe

- **GIVEN** the user rapidly toggles between formats (faster
  than the QR generator's promise resolves)
- **WHEN** an older format's `QRCode.toDataURL` call resolves
  after the user has already moved to a newer format
- **THEN** the stale result SHALL NOT overwrite the current QR
- **AND** the React tree SHALL achieve this via a monotonic
  token (matching the Vue tree's `qrToken` ref pattern) or an
  equivalent AbortController flow

#### Scenario: Copy button writes to clipboard

- **GIVEN** the page is rendered with a non-zip format active
- **WHEN** the operator clicks the copy button
- **THEN** the clipboard SHALL contain the URL
- **AND** a transient AntD `message.success` SHALL confirm the
  copy

#### Scenario: Format labels and hints come from i18n

- **WHEN** the page renders the 7 format chips
- **THEN** each chip's label SHALL be the hard-coded literal
  (`Base64`, `Clash`, `Sing-box`, `SIP008`, `WireGuard`,
  `WG (ZIP)`, `JSON`)
- **AND** each chip's hint and supported-apps subtitle SHALL
  come from `portal.subscription.formats.<key>.hint` and
  `portal.subscription.formats.<key>.apps` (Vue tree's exact
  i18n key shape)

### Requirement: Usage view renders traffic stats

`Usage.tsx` (registered at `/portal/usage`) SHALL render the same
traffic stats and progress bars the Vue tree's
`portal/Dashboard.vue` shows.

#### Scenario: Stats match the API response

- **GIVEN** the backend returns traffic stats for the current user
- **WHEN** the page renders
- **THEN** the rendered numbers (up / down / limit / reset date)
  SHALL match the API response exactly
- **AND** the progress bar percentage SHALL be
  `min(100, used/limit*100)` rounded to one decimal

### Requirement: Plans purchase flow lands an order

`Plans.tsx` SHALL list available plans (filtered to enabled
ones), surface a "Buy" CTA per plan, and on submit SHALL open
the appropriate payment-gateway flow (Alipay or Stripe per
backend config).

#### Scenario: Alipay flow opens the modal

- **GIVEN** the backend gateway is configured for Alipay
- **WHEN** the user clicks Buy on a plan
- **THEN** the page SHALL POST to the order-create endpoint
- **AND** SHALL open `AlipayPayModal` with the returned payment
  URL/QR

#### Scenario: Stripe flow redirects to checkout

- **GIVEN** the backend gateway is configured for Stripe
- **WHEN** the user clicks Buy on a plan
- **THEN** the page SHALL POST to the order-create endpoint
- **AND** SHALL redirect the browser to the Stripe Checkout URL
  returned by the backend

#### Scenario: Disabled plans are hidden

- **GIVEN** the backend lists 3 plans where 1 has `enabled: false`
- **WHEN** the page renders
- **THEN** only 2 plan cards SHALL appear
- **AND** the disabled plan SHALL NOT have its Buy button rendered
  anywhere

### Requirement: Orders view lists order history

`Orders.tsx` (at `/portal/orders`) SHALL render the user's order
history with the same columns the Vue tree shows: order id,
plan name, amount, status, created at.

#### Scenario: Order list matches backend

- **GIVEN** the backend returns N orders for the current user
- **WHEN** the page renders
- **THEN** the table SHALL show exactly N rows
- **AND** each row's status SHALL render with the same color
  badge logic as the Vue tree (completed→accent, failed→red,
  pending→neutral)

### Requirement: Profile manages display name, email, password, and OIDC provider links

`Profile.tsx` SHALL render four account-security sections:
display name, email, password, and connected OIDC providers. Each
editable section SHALL use AntD `<Form>`. Email remains the only
local login identifier; display name is profile metadata only and
MUST NOT participate in login or uniqueness checks.

The Vue tree's older Profile page is not the complete contract for
P5. During this pre-launch window the backend/API/database MAY be
changed to support the account model below; no compatibility with
the current single-provider `users.oidc_subject` field is required.
OIDC unlink is out of scope for P5.

#### Scenario: Display name is profile metadata

- **GIVEN** the user opens `/portal/profile`
- **WHEN** the profile response contains `display_name`
- **THEN** the Profile page SHALL render it in an editable AntD form
- **AND** saving SHALL call a profile-update mutation that updates
  `display_name` only
- **AND** login SHALL continue to use `email` exclusively
- **AND** no uniqueness validation SHALL run for `display_name`

#### Scenario: Email change requires a verification code

- **GIVEN** the user opens `/portal/profile`
- **WHEN** the user enters a new email address
- **THEN** the page SHALL expose a "send verification code" action
  that calls the email-verification start endpoint with
  `purpose = "change_email"`
- **AND** the confirmation form SHALL require the new email and
  verification code before submitting the email-change mutation
- **AND** the mutation SHALL mark the new email verified on success

#### Scenario: Password change validates old password and min length

- **WHEN** the user submits the password form
- **THEN** the form SHALL require current password, new password,
  and confirmation
- **AND** a new password shorter than 8 characters SHALL show a
  field-level validation error
- **AND** mismatched confirmation SHALL block submission
- **AND** a successful mutation SHALL clear all password fields

#### Scenario: Connected OIDC providers list is multi-provider

- **GIVEN** multiple OIDC providers are configured
- **WHEN** the user opens `/portal/profile`
- **THEN** the OIDC section SHALL render one row per provider
  with provider display name, icon, and linked/unlinked state
- **AND** clicking "Connect" on an unlinked provider SHALL start
  that provider's OIDC link flow
- **AND** linked providers SHALL be shown as connected
- **AND** no "Unlink" action SHALL be rendered in P5

### Requirement: OIDC uses multi-provider account identities

The backend SHALL replace the single-provider account model with a
multi-provider identity model. The database SHALL include an
`oidc_providers` table for provider configuration and a
`user_oidc_identities` table for linked identities. `users.email`
remains the unique local login identifier and `users.password_hash`
MUST NOT be nullable. OIDC-created users MUST set a local password
during account completion.

The OIDC provider's verified email and the user's local login email
MAY differ. The provider email SHALL be stored on the OIDC identity
for audit/display; the local login email SHALL live on `users.email`
and SHALL be verified through the dashboard email-verification flow.

#### Scenario: Provider list returns all enabled providers

- **GIVEN** two OIDC providers are enabled
- **WHEN** the login page or Profile page fetches OIDC providers
- **THEN** the API SHALL return both providers with stable provider
  keys, display names, optional icons, and start URLs
- **AND** the frontend SHALL render both providers

#### Scenario: OIDC callback for an already-linked identity logs in

- **GIVEN** a `user_oidc_identities` row exists for
  `(provider_key, subject)`
- **WHEN** the OIDC callback completes for the same provider and
  subject
- **THEN** the backend SHALL issue the portal token for the linked
  user
- **AND** the frontend SHALL store the session and navigate to the
  requested `redirect_after` path

#### Scenario: OIDC callback with provider email matching an existing user requires password binding

- **GIVEN** the provider returns a verified email that matches an
  existing `users.email`
- **AND** no identity row exists for `(provider_key, subject)`
- **WHEN** the OIDC callback completes
- **THEN** the backend SHALL return a pending account-completion
  response containing a short-lived pending token, provider
  metadata, the provider email, and `existing_user = true`
- **AND** the frontend SHALL render a completion page that offers
  "Bind existing account"
- **AND** binding SHALL require the existing account password
- **AND** the backend SHALL only link the OIDC identity and issue a
  portal token after that password check succeeds

#### Scenario: OIDC account completion can create a new user with a different verified email

- **GIVEN** an OIDC callback returned a pending token
- **WHEN** the user chooses "Create a new account"
- **THEN** the completion page SHALL require display name,
  password, local login email, and verification code
- **AND** the verification start endpoint SHALL be called with
  `purpose = "oidc_create_account"`
- **AND** the local login email MAY differ from the provider email
- **AND** the backend SHALL reject local emails that are already in
  use
- **AND** on success the backend SHALL create the user, persist the
  OIDC identity with provider email, and issue a portal token

#### Scenario: OIDC callback rejects providers without verified email

- **GIVEN** an OIDC provider callback does not include a verified
  email claim
- **WHEN** the callback is processed
- **THEN** the backend SHALL reject the login with a typed error
- **AND** the frontend SHALL explain that this provider must be
  configured to return a verified email before it can be used

### Requirement: Email verification endpoints support profile and OIDC completion flows

The backend SHALL expose email verification APIs that can be used
by Profile email change and OIDC create-account completion. The
flow SHALL enforce a 10-minute code TTL, a 60-second resend
cooldown per email/purpose, and at most 5 failed confirmation
attempts per issued code.

#### Scenario: Verification start returns resend timing

- **WHEN** the frontend starts verification for
  `purpose = "change_email"` or `purpose = "oidc_create_account"`
- **THEN** the backend SHALL send a code to the requested email
- **AND** the response SHALL include cooldown/expires metadata so
  the frontend can disable the resend button until allowed

#### Scenario: Verification confirm returns a short-lived token

- **GIVEN** the user enters the correct code before expiry
- **WHEN** the frontend confirms the code
- **THEN** the backend SHALL return a short-lived verification token
  scoped to the email and purpose
- **AND** Profile email-change and OIDC create-account mutations
  SHALL require that token instead of accepting a raw code directly

### Requirement: `AlipayPayModal` renders the Alipay QR

`components/portal/AlipayPayModal.tsx` SHALL be an AntD `<Modal>`
that displays the Alipay payment QR returned by the backend,
together with the order amount and a polling indicator that
reflects payment status.

#### Scenario: Modal polls payment status

- **GIVEN** the modal is open with an active order id
- **WHEN** the order's status changes from `pending` to `paid`
  on the backend
- **THEN** the modal SHALL detect the change within 5 seconds
- **AND** SHALL replace the QR with a success state
- **AND** SHALL auto-close after a brief delay or expose a "View
  order" CTA

#### Scenario: Modal close cancels polling

- **GIVEN** the modal is open and polling
- **WHEN** the operator closes the modal
- **THEN** all polling timers / TanStack Query refetch loops SHALL
  stop
- **AND** no further requests SHALL be issued for that order's
  status

### Requirement: Each portal view owns a documented i18n prefix

Every React portal view SHALL consume i18n keys exclusively from its documented prefix (plus shared `common.*` / `nav.*` / `app.*` / `auth.*` keys for cross-cutting concerns).

| View | Owned prefix | Notes |
|---|---|---|
| Subscription | `portal.subscription.*` | Format chip metadata under `portal.subscription.formats.<key>.{hint,apps}` for the 7 formats spec'd above |
| Usage (Dashboard) | `portal.dashboard.*` | Keep the `dashboard` prefix even though the route is `/portal/usage` — preserve key set 1:1 |
| Plans | `portal.plans.*` | Purchase confirmation under `portal.plans.confirm*`, payment method labels under `portal.plans.method.*` |
| Orders | `portal.orders.*` | Continue-payment + paid-state strings |
| Profile | `portal.profile.*` | Display-name / email-verification / password-change / multi-provider OIDC link sections; `auth.*` reused for OIDC callback and provider-entry strings |
| AlipayPayModal | `portal.alipayPay.*` | Polling status + countdown strings |

#### Scenario: Portal view only references its owned prefix

- **GIVEN** the React Subscription view source
- **WHEN** grepped for `t(['"]`
- **THEN** every matched key SHALL start with
  `portal.subscription.`, `common.`, `nav.`, `app.`, or `auth.`
- **AND** no `portal.plans.*` or `portal.profile.*` key SHALL
  leak in (the Vue tree had a few cross-references during the
  dashboard-user-panel batch; the rewrite is a chance to clean
  those up)

#### Scenario: All 7 subscription format keys are preserved

- **GIVEN** the Vue tree's `zh.ts` contains seven
  `portal.subscription.formats.<key>.{hint,apps}` pairs
  (`base64`, `clash`, `singbox`, `sip008`, `wireguard`,
  `wireguardZip`, `json`)
- **WHEN** the React tree's locale files are inspected
- **THEN** all 14 keys SHALL be present with identical values

### Requirement: P5 portal specs pass with parity

Every view ported in P5 SHALL ship a `.spec.tsx` whose `it(...)`
count meets or exceeds the corresponding Vue `.spec.ts`.

#### Scenario: P5 test suite passes

- **WHEN** the operator runs `npm run test -- src/views/portal`
- **THEN** all specs SHALL pass
- **AND** each spec SHALL have at least as many `it(...)` blocks
  as the corresponding Vue spec
