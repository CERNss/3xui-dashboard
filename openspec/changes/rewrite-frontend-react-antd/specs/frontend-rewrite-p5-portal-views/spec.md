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

### Requirement: Profile supports email, password, OIDC linking

`Profile.tsx` SHALL render the same three sections the Vue tree
`portal/Profile.vue` exposes: email change, password change,
linked OIDC providers. Each section SHALL use AntD `<Form>`.

#### Scenario: Email change requires current password

- **GIVEN** the operator is on `/portal/profile`
- **WHEN** the operator submits the email-change form with a new
  email but blank password
- **THEN** the form SHALL block submission with a validation error
  on the password field

#### Scenario: Password change validates min length

- **WHEN** the operator types a new password shorter than 8 chars
- **THEN** the password field SHALL show a min-length error
- **AND** SHALL NOT allow submission

#### Scenario: OIDC unlink uses confirmation

- **GIVEN** the operator has at least one linked OIDC provider
- **WHEN** the operator clicks "Unlink" on a provider
- **THEN** an AntD `Modal.confirm` SHALL ask for confirmation
- **AND** only on confirm SHALL the unlink mutation fire

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
| Profile | `portal.profile.*` | Email-change / password-change / OIDC unlink sections; `auth.*` reused for OIDC provider names |
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
