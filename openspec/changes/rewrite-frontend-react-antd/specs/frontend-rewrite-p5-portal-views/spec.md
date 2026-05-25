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

### Requirement: Subscription view shows URL, QR, and copy controls

`Subscription.tsx` SHALL display the user's subscription URL,
render a QR code of that URL, and surface copy-to-clipboard
controls — matching the Vue tree's `portal/Subscription.vue`
content.

#### Scenario: QR matches the displayed URL

- **GIVEN** the user is on `/portal/subscription`
- **WHEN** the page renders
- **THEN** the QR SHALL encode exactly the URL string shown in
  the text field
- **AND** scanning the QR SHALL yield the same string the user
  can copy

#### Scenario: Copy button writes to clipboard

- **GIVEN** the page is rendered with a subscription URL
- **WHEN** the operator clicks the copy button
- **THEN** the clipboard SHALL contain the URL
- **AND** a transient toast/message SHALL confirm the copy

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

### Requirement: P5 portal specs pass with parity

Every view ported in P5 SHALL ship a `.spec.tsx` whose `it(...)`
count meets or exceeds the corresponding Vue `.spec.ts`.

#### Scenario: P5 test suite passes

- **WHEN** the operator runs `npm run test -- src/views/portal`
- **THEN** all specs SHALL pass
- **AND** each spec SHALL have at least as many `it(...)` blocks
  as the corresponding Vue spec
