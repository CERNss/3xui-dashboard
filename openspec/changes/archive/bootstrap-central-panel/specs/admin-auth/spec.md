## ADDED Requirements

### Requirement: Environment-Supplied Admin Credential

The system SHALL recognize exactly one administrator whose username and password
are supplied via environment configuration (`.env`), and SHALL NOT store the
administrator in the database.

#### Scenario: Admin credential loaded from environment

- **WHEN** the backend starts with `ADMIN_USERNAME` and `ADMIN_PASSWORD` set
- **THEN** those values define the only administrator, and no `admins` table or
  admin database record is created

#### Scenario: Missing admin credential

- **WHEN** the backend starts without `ADMIN_USERNAME` or `ADMIN_PASSWORD` set
- **THEN** the system SHALL refuse to start and log a clear configuration error

#### Scenario: No admin registration

- **WHEN** any request attempts to register or create an admin account
- **THEN** the system SHALL reject it — there is no admin registration flow

### Requirement: Admin Console Login

The system SHALL provide a dedicated admin login endpoint, separate from the
user portal, that authenticates against the environment credential.

#### Scenario: Valid admin login

- **WHEN** a request to `POST /api/admin/auth/login` carries the configured
  `ADMIN_USERNAME` and `ADMIN_PASSWORD`
- **THEN** the system returns HTTP 200 with a JWT whose claims mark it as an
  admin token (audience `admin`)

#### Scenario: Invalid admin login

- **WHEN** the submitted admin username or password does not match the
  environment credential
- **THEN** the system returns HTTP 401 with a generic error

#### Scenario: Constant-time password comparison

- **WHEN** the submitted admin password is checked
- **THEN** the comparison SHALL be constant-time to avoid timing disclosure

### Requirement: Separated Admin and User Auth Domains

Admin and user authentication SHALL be fully isolated: separate endpoints,
separate JWT audiences, and no cross-acceptance of tokens.

#### Scenario: User token rejected on admin routes

- **WHEN** a token issued by the user portal (audience `user`) is presented to
  any `/api/admin/*` route
- **THEN** the system returns HTTP 403 regardless of the token's validity

#### Scenario: Admin token rejected on user routes

- **WHEN** an admin token (audience `admin`) is presented to a `/api/user/*` route
- **THEN** the system returns HTTP 403

#### Scenario: Admin login endpoint isolated from OIDC

- **WHEN** the admin login endpoint is called
- **THEN** it SHALL NOT invoke any OIDC or email/password user-auth path —
  admin auth depends only on the environment credential

### Requirement: Admin Session Lifetime

Admin JWTs SHALL carry an expiry and be re-issued only through admin login.

#### Scenario: Expired admin token

- **WHEN** an admin presents an expired admin JWT to a protected route
- **THEN** the system returns HTTP 401 and the admin must log in again

#### Scenario: Admin credential rotation

- **WHEN** the `ADMIN_PASSWORD` environment value is changed and the backend restarts
- **THEN** previously issued admin tokens remain valid only until their own
  expiry, and new logins require the new password
