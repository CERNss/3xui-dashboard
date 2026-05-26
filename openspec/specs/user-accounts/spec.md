# user-accounts

End-user account lifecycle: register, login, change password, verified
email change, OIDC linkage, admin moderation, and the client→user ownership
mapping that vanilla 3x-ui lacks.

## Purpose & boundaries

This is the canonical surface for everything that happens to a row in
the `users` table. Adjacent modules:

- **`admin-auth`** — the single administrator (never in this table).
- **`email-verification`** — the 6-digit code service that gates register.
- **`unified-login`** — the SPA chrome that presents login + register.
- **`oidc-providers`** — provider listing, OIDC start/callback, and
  account-completion endpoints.
- **`client-provisioning`** — owns the `client_ownerships` table on the
  fleet side; this module owns the user side of that relation.

## Requirements

### Requirement: Email/Password Registration

The system SHALL allow end users to register a portal account with an
email and password, subject to the public-registration and
email-domain controls, AND (when SMTP is enabled) a valid 6-digit
verification code obtained via the `email-verification` flow.

#### Scenario: Successful registration with verification code (SMTP enabled)

- **GIVEN** `cfg.SMTP.Enabled() == true`
- **AND** the user has previously called `POST /api/user/auth/send-code` and received an email containing a 6-digit code
- **WHEN** the user POSTs `{email, password, code}` to `/api/user/auth/register` and the code matches the latest unconsumed unexpired row
- **AND** registration is permitted by public-registration and domain controls
- **THEN** the verification code SHALL be consumed
- **THEN** the system SHALL create a `users` row, store a bcrypt password hash, and issue a user-audience JWT
- **AND** publish a `user.registered` event on the internal bus

#### Scenario: Register without code in dev mode (SMTP disabled)

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** a client POSTs `/api/user/auth/register` with `code` omitted or any value
- **THEN** the system SHALL skip code verification entirely
- **AND** create the account as if registration were unprotected

#### Scenario: Register missing or wrong code (SMTP enabled)

- **WHEN** `req.Code` is empty
- **THEN** the system SHALL respond HTTP 400 with `{"error":"缺少邮箱验证码"}`

- **WHEN** `req.Code` is wrong
- **THEN** the system SHALL respond HTTP 400 with `{"error":"验证码不正确"}`
- **AND** the row's `attempts` counter SHALL increment (≥5 burns it)

#### Scenario: Duplicate email

- **WHEN** registration is attempted with an email that already belongs to a `users` row
- **THEN** the system SHALL respond HTTP 409 (conflict)
- **AND** no second account SHALL be created

#### Scenario: Weak password rejected

- **WHEN** a submitted password is shorter than 8 characters
- **THEN** registration SHALL be rejected with `ErrPasswordTooShort` surfaced as HTTP 400
- **AND** no account SHALL be created

### Requirement: Public Registration Control

The system SHALL provide a configuration switch
(`public_registration_enabled` setting, falling back to env
`PUBLIC_REGISTRATION`) that enables or disables public self-service
registration.

#### Scenario: Registration enabled

- **WHEN** public registration is enabled
- **THEN** the registration endpoint accepts the flow described above
- **AND** the portal's "注册" tab SHALL be available in the unified login UI

#### Scenario: Registration disabled

- **WHEN** public registration is disabled and a client calls `/api/user/auth/register`
- **THEN** the system SHALL respond HTTP 403 with `ErrRegistrationOff`
- **AND** the portal SHALL hide the "注册" tab (or surface a "registration closed" message)

#### Scenario: Existing accounts can still log in

- **WHEN** public registration is disabled
- **THEN** the switch SHALL affect only account creation
- **AND** existing users SHALL still be able to log in via email/password and OIDC normally

#### Scenario: OIDC account creation follows the switch

- **WHEN** public registration is disabled
- **THEN** OIDC login for already linked identities and bind-existing completion SHALL still work
- **AND** OIDC create-account completion SHALL be rejected with HTTP 403

### Requirement: Email Domain Allowlist

The system SHALL support restricting account email addresses to a
configurable set of allowed domain suffixes (env
`EMAIL_DOMAIN_ALLOWLIST` and/or the `email_domain_allowlist` setting).

#### Scenario: Allowlist configured

- **WHEN** an allowlist is configured (e.g. `company.com,edu.cn`) and a visitor registers or changes to an email whose `@<domain>` is in the allowlist (case-insensitive)
- **THEN** the operation SHALL be permitted

#### Scenario: Disallowed domain rejected

- **WHEN** the allowlist is configured and an email's domain is not in it
- **THEN** registration or email change SHALL be rejected with `ErrDomainNotAllowed` surfaced as HTTP 403
- **AND** the error message SHALL name the allowed domains so the user knows which to use

#### Scenario: Allowlist empty means unrestricted

- **WHEN** the allowlist is empty or unset
- **THEN** any syntactically valid email domain SHALL be accepted

#### Scenario: Allowlist applies to OIDC emails

- **WHEN** an OIDC login presents an email whose domain is not in a configured allowlist
- **THEN** the system SHALL reject the login (or provision the account without an email) rather than storing a disallowed email

### Requirement: Verified Email Change

The system SHALL allow an authenticated user to change their local
login email only after proving ownership through the email-verification
flow.

#### Scenario: Change email with verification token

- **WHEN** an authenticated user confirms an email-verification code for `purpose="change_email"` and submits the returned verification token to `/api/user/change-email`
- **AND** the domain passes the allowlist and the email is not already used by another account
- **THEN** the system SHALL update `users.email`
- **AND** mark `email_verified=true`

#### Scenario: Email already in use

- **WHEN** a user tries to change to an email that already belongs to another account
- **THEN** the change SHALL be rejected with HTTP 409

### Requirement: Email/Password Login

The system SHALL authenticate users by email and password at a
user-only endpoint.

#### Scenario: Valid user login

- **WHEN** a user POSTs correct `{email, password}` to `/api/user/auth/login`
- **THEN** the system SHALL respond HTTP 200 with a JWT whose audience is `user`, plus response fields `user_id` and `email`

#### Scenario: Invalid user login

- **WHEN** the email is unknown or the password is wrong
- **THEN** the system SHALL respond HTTP 401 with a generic error that does NOT reveal whether the email exists

#### Scenario: Suspended user

- **WHEN** the credentials are correct but the user's status is `suspended`
- **THEN** the system SHALL respond HTTP 403 with `ErrUserSuspended` surfaced
- **AND** no JWT SHALL be issued

### Requirement: Standard OIDC Login

The system SHALL support end-user login through a standard OIDC
provider using the Authorization Code flow with PKCE.

#### Scenario: OIDC configured from environment

- **WHEN** `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` are all set
- **THEN** the system SHALL expose the OIDC login button via `GET /api/user/auth/oidc/providers` (see `oidc-providers`)

#### Scenario: Authorization request

- **WHEN** a user starts OIDC login at `POST /api/user/auth/oidc/start`
- **THEN** the system SHALL generate `state` and PKCE values, store them in the server-side short-lived session store, and return JSON containing `authorize_url`
- **AND** the frontend SHALL navigate the browser to that URL

#### Scenario: Callback exchanges code

- **WHEN** the provider redirects back with a matching `state` and an authorization code
- **THEN** the system SHALL exchange the code (with the PKCE verifier) for tokens, validate the ID token signature against JWKS and its issuer/audience/expiry, and read the subject and email claims

#### Scenario: First OIDC login provisions an account

- **WHEN** a user completes OIDC login with an email claim and no account exists for that email
- **THEN** the callback SHALL return a short-lived pending completion token
- **AND** the user SHALL complete `POST /api/user/auth/oidc/create-account` with display name, password, and a verified local email token before a `users` row is created
- **AND** account creation SHALL respect public-registration and email-domain controls

#### Scenario: Returning OIDC user

- **WHEN** a user completes OIDC login and the OIDC subject is already linked to a local account
- **THEN** the system SHALL log them into the existing account without creating a duplicate

#### Scenario: Existing email requires account decision

- **WHEN** an OIDC login presents an email that already belongs to an account not linked to that OIDC subject
- **THEN** the callback SHALL return a short-lived pending decision rather than silently linking or creating a duplicate
- **AND** the frontend SHALL ask whether to bind the OIDC login to the existing account or create a separate local account
- **AND** binding the existing account SHALL require the existing local password through `POST /api/user/auth/oidc/bind-existing`
- **AND** the system SHALL NOT expose a passwordless `/api/user/auth/oidc/resolve` endpoint

#### Scenario: Missing OIDC email is rejected

- **WHEN** an OIDC login succeeds at the provider but the ID token does not include an email claim
- **THEN** the dashboard SHALL reject the login because email is the unique user identity

### Requirement: User Password Management

Users with an email/password credential SHALL be able to change their
password.

#### Scenario: Passwords hashed

- **WHEN** a user password is set at registration or change
- **THEN** the system SHALL store only a bcrypt hash and never the plaintext

#### Scenario: Self-service password change

- **WHEN** an authenticated user submits their current and a new password
- **THEN** the system SHALL verify the current password and update the stored hash

#### Scenario: OIDC-created account has a password

- **WHEN** a user completes OIDC create-account
- **THEN** the account SHALL store a local bcrypt password hash
- **AND** future password changes SHALL require the current password

### Requirement: User Token Audience

User portal JWTs SHALL be scoped to the user domain and SHALL NOT
grant admin access.

#### Scenario: User token audience

- **WHEN** any user-portal JWT is issued (email/password or OIDC)
- **THEN** the JWT claims SHALL include `sub` set to the user id string, audience `user`, and an `exp` timestamp
- **AND** the response body SHALL include `user_id` for frontend state

#### Scenario: Protected user route

- **WHEN** a request to `/api/user/*` omits or carries an invalid/expired user JWT
- **THEN** the system SHALL respond HTTP 401
- **AND** the axios interceptor SHALL redirect to `/login`

### Requirement: Admin Administration Of User Accounts

The administrator SHALL be able to list, edit, suspend, and delete
user accounts.

#### Scenario: Admin lists users

- **WHEN** the admin calls `GET /api/admin/users`
- **THEN** the system SHALL return a paginated list with id, email, `oidc_linked`, balance, linked client identity, and status

#### Scenario: Admin suspends a user

- **WHEN** the admin suspends a user account
- **THEN** that user's existing tokens SHALL be rejected (via status check on the protected route)
- **AND** new logins SHALL respond with `ErrUserSuspended` until reactivated

#### Scenario: Admin deletes a user

- **WHEN** the admin deletes a user account
- **THEN** the portal account SHALL be removed
- **AND** the underlying Xray client on the node SHALL NOT be deleted unless the admin explicitly requests it

### Requirement: Client Ownership Mapping

The system SHALL maintain a mapping between a user account and the
Xray client(s) it owns across the node fleet, since vanilla 3x-ui has
no such concept.

#### Scenario: Admin links a client to a user

- **WHEN** the admin associates a `(node_id, inbound_tag, client_email/uuid)` triple with a user account
- **THEN** the mapping SHALL be persisted in `client_ownerships`
- **AND** the user SHALL thereafter see that client's subscription and traffic

#### Scenario: User sees only owned clients

- **WHEN** an authenticated user requests their subscription or traffic
- **THEN** the response SHALL contain only clients mapped to that user
