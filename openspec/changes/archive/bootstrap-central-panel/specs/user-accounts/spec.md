## ADDED Requirements

### Requirement: Email/Password Registration

The system SHALL allow end users to register a portal account with an email and
password, subject to the public-registration and email-domain controls.

#### Scenario: Successful registration

- **WHEN** a visitor submits a unique email and a password meeting the strength
  policy to `POST /api/user/auth/register`, and registration is permitted by the
  public-registration and email-domain controls
- **THEN** the system creates a `user` record, stores a bcrypt password hash,
  and returns a user-audience JWT

#### Scenario: Duplicate email

- **WHEN** a visitor registers with an email that already belongs to a user account
- **THEN** the system returns a conflict error and does not create a second account

#### Scenario: Weak password rejected

- **WHEN** a submitted password fails the configured strength policy
- **THEN** registration is rejected with a validation error and no account is created

### Requirement: Public Registration Control

The system SHALL provide a configuration switch that enables or disables public
self-service registration.

#### Scenario: Registration enabled

- **WHEN** public registration is enabled
- **THEN** the registration endpoint and the portal's "Sign up" UI are available

#### Scenario: Registration disabled

- **WHEN** public registration is disabled and a visitor calls the registration endpoint
- **THEN** the system returns HTTP 403 with a "registration closed" message, and
  the portal hides the "Sign up" UI

#### Scenario: Existing accounts can still log in

- **WHEN** public registration is disabled
- **THEN** the switch SHALL affect only account creation — existing users SHALL
  still be able to log in via email/password and OIDC normally

#### Scenario: OIDC unaffected by the switch

- **WHEN** public registration is disabled
- **THEN** OIDC login SHALL still be able to provision a first-time account,
  unless OIDC auto-provisioning is itself separately disabled

### Requirement: Email Domain Allowlist

The system SHALL support restricting account email addresses to a configurable
set of allowed domain suffixes.

#### Scenario: Allowlist configured

- **WHEN** an email-domain allowlist is configured (e.g. `company.com`, `edu.cn`)
  and a visitor registers or binds an email whose domain matches the allowlist
- **THEN** the operation is permitted

#### Scenario: Disallowed domain rejected

- **WHEN** the allowlist is configured and an email's domain is not in it
- **THEN** registration or email binding is rejected with a validation error
  naming the allowed domains

#### Scenario: Allowlist empty means unrestricted

- **WHEN** the email-domain allowlist is empty or unset
- **THEN** any syntactically valid email domain is accepted

#### Scenario: Allowlist applies to OIDC emails

- **WHEN** an OIDC login presents an email whose domain is not in a configured allowlist
- **THEN** the system SHALL reject the login (or provision the account without an
  email) rather than storing a disallowed email

### Requirement: Email Address Binding

The system SHALL allow an authenticated user to bind an email address to their
account — in particular an OIDC-provisioned account that has no email, or a user
adding a secondary verified email.

#### Scenario: Bind an email

- **WHEN** an authenticated user submits an email address to the bind endpoint,
  the domain passes the allowlist, and the email is not already used by another account
- **THEN** the system associates the email with the user's account

#### Scenario: Bind requires verification when SMTP is enabled

- **WHEN** email delivery is enabled and a user requests to bind an email
- **THEN** the system sends a verification message and marks the email `verified`
  only after the user confirms the verification token

#### Scenario: Bind without verification when SMTP is disabled

- **WHEN** email delivery is disabled and a user binds an email
- **THEN** the system records the email as `unverified` and completes the bind
  without sending a message

#### Scenario: Email already in use

- **WHEN** a user tries to bind an email that already belongs to another account
- **THEN** the bind is rejected with a conflict error

### Requirement: Email Delivery via SMTP

The system SHALL provide an optional SMTP integration for outbound email
(verification, notifications) that is configured separately and may be left
disabled without blocking core functionality.

#### Scenario: SMTP configuration slot

- **WHEN** the backend loads configuration
- **THEN** it SHALL read an SMTP section (host, port, username, password,
  from-address, TLS mode, enabled flag) from environment config, defaulting to disabled

#### Scenario: SMTP disabled

- **WHEN** SMTP is disabled
- **THEN** flows that would send email SHALL degrade gracefully — email
  verification is skipped and addresses are stored as `unverified` — and no
  core feature (login, registration, provisioning) is blocked

#### Scenario: SMTP enabled

- **WHEN** SMTP is enabled and correctly configured
- **THEN** the system SHALL send verification and notification emails through
  the configured server

#### Scenario: SMTP send failure is non-fatal

- **WHEN** SMTP is enabled but a send attempt fails
- **THEN** the system SHALL log the failure and surface a retryable error to the
  user without corrupting the account state

### Requirement: Email/Password Login

The system SHALL authenticate users by email and password at a user-only endpoint.

#### Scenario: Valid user login

- **WHEN** a user submits correct email/password to `POST /api/user/auth/login`
- **THEN** the system returns HTTP 200 with a JWT whose audience is `user` and
  the user's profile

#### Scenario: Invalid user login

- **WHEN** the email is unknown or the password is wrong
- **THEN** the system returns HTTP 401 with a generic error that does not reveal
  whether the email exists

### Requirement: Standard OIDC Login

The system SHALL support end-user login through a standard OIDC provider using
the Authorization Code flow with PKCE.

#### Scenario: OIDC configured from environment

- **WHEN** the backend starts with OIDC enabled and an issuer/discovery URL,
  client id, and client secret supplied via environment
- **THEN** the system SHALL resolve the provider's authorize, token, userinfo,
  and JWKS endpoints (via the discovery document when available) and expose an
  OIDC login option in the user portal

#### Scenario: Authorization request

- **WHEN** a user starts OIDC login at `GET /api/user/auth/oidc/start`
- **THEN** the system generates a `state` and PKCE `code_verifier`/`code_challenge`,
  stores them in short-lived cookies, and redirects the browser to the
  provider's authorize endpoint with `openid` scope

#### Scenario: Callback exchanges code

- **WHEN** the provider redirects back to the callback with a matching `state`
  and an authorization code
- **THEN** the system exchanges the code (with the PKCE verifier) for tokens,
  validates the ID token signature against JWKS and its issuer/audience/expiry,
  and reads the subject and email claims

#### Scenario: State mismatch rejected

- **WHEN** the callback `state` does not match the stored cookie value, or the
  PKCE verifier is missing
- **THEN** the system aborts the login with an error and issues no token

#### Scenario: First OIDC login provisions an account

- **WHEN** a user completes OIDC login and no account exists for that provider
  subject
- **THEN** the system creates a `user` record linked to the OIDC subject (and
  email when present) and issues a user-audience JWT

#### Scenario: Returning OIDC user

- **WHEN** a user completes OIDC login and an account already exists for that
  provider subject
- **THEN** the system logs them into the existing account without creating a duplicate

#### Scenario: OIDC account links to existing email

- **WHEN** an OIDC login presents a verified email that already belongs to an
  email/password account
- **THEN** the system SHALL link the OIDC identity to that existing account
  rather than creating a separate one

### Requirement: User Password Management

Users with an email/password credential SHALL be able to change their password.

#### Scenario: Passwords hashed

- **WHEN** a user password is set at registration or change
- **THEN** the system stores only a bcrypt hash and never the plaintext

#### Scenario: Self-service password change

- **WHEN** an authenticated user submits their current and a new password
- **THEN** the system verifies the current password and updates the stored hash

#### Scenario: OIDC-only account has no password

- **WHEN** a user who registered solely through OIDC opens password settings
- **THEN** the portal SHALL offer to set an initial password rather than asking
  for a current one

### Requirement: User Token Audience

User portal JWTs SHALL be scoped to the user domain and SHALL NOT grant admin access.

#### Scenario: User token audience

- **WHEN** any user-portal JWT is issued (email/password or OIDC)
- **THEN** its claims SHALL include the user id, audience `user`, and an expiry

#### Scenario: Protected user route

- **WHEN** a request to a `/api/user/*` route omits or carries an invalid/expired
  user JWT
- **THEN** the system returns HTTP 401

### Requirement: Admin Administration of User Accounts

The administrator SHALL be able to list, edit, suspend, and delete user accounts.

#### Scenario: Admin lists users

- **WHEN** the admin calls `GET /api/admin/users`
- **THEN** the system returns a paginated list with id, email, auth method
  (oidc / password / both), balance, linked client identity, and status

#### Scenario: Admin suspends a user

- **WHEN** the admin suspends a user account
- **THEN** that user's existing tokens SHALL be rejected and new logins refused
  until the account is reactivated

#### Scenario: Admin deletes a user

- **WHEN** the admin deletes a user account
- **THEN** the portal account is removed, and the underlying Xray client on the
  node SHALL NOT be deleted unless the admin explicitly requests it

### Requirement: Client Ownership Mapping

The system SHALL maintain a mapping between a user account and the Xray
client(s) it owns across the node fleet, since vanilla 3x-ui has no such concept.

#### Scenario: Admin links a client to a user

- **WHEN** the admin associates a node id + inbound tag + client email/UUID with
  a user account
- **THEN** the mapping is persisted and the user can thereafter see that client's
  subscription and traffic

#### Scenario: User sees only owned clients

- **WHEN** an authenticated user requests their subscription or traffic
- **THEN** the system SHALL return data only for clients mapped to that user
