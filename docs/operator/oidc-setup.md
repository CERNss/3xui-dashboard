# OIDC login setup

The dashboard's portal supports OIDC (OpenID Connect) login as an
alternative to email + password. Verified against self-hosted
**Zitadel** and **Keycloak**; should work against any spec-
compliant IDP that uses RS256-signed ID tokens.

## What you need

1. An OIDC application registered with your IDP
2. The application's client ID
3. The application's client secret (or none, if using PKCE only)
4. The IDP's issuer URL (e.g. `https://auth.example.com`)

## Configure the dashboard

Set these env vars (or `.env` entries):

```env
OIDC_ISSUER=https://auth.example.com
OIDC_CLIENT_ID=3xui-dashboard
OIDC_CLIENT_SECRET=<from IDP — empty string if PKCE-only public client>
OIDC_REDIRECT_URL=https://dashboard.example.com/oidc/callback
OIDC_SCOPES=openid,profile,email          # comma-separated
OIDC_DISPLAY_NAME=My Company SSO          # shown on the login button
OIDC_ICON_URL=https://cdn.example.com/logo.svg   # optional
```

Optional explicit endpoint overrides (skip discovery when set):

```env
OIDC_AUTH_URL=https://auth.example.com/oauth/v2/authorize
OIDC_TOKEN_URL=https://auth.example.com/oauth/v2/token
OIDC_JWKS_URL=https://auth.example.com/oauth/v2/keys
OIDC_USERINFO_URL=https://auth.example.com/oauth/v2/userinfo
```

When the explicit overrides are missing, the dashboard fetches
`<issuer>/.well-known/openid-configuration` on startup (lazy, on
first OIDC login) and caches it for 24 h.

## Zitadel-specific notes

1. Create a **Web** application (not Native, not API)
2. **Authentication Method**: choose "PKCE" if you want to skip
   the client secret, otherwise "Basic" + paste the secret
3. **Redirect URIs**: add `https://dashboard.example.com/oidc/callback`
4. **Post Logout URIs**: optional
5. **Token Settings**:
   - Auth Token Type: **JWT**
   - ID Token contains user info: **Yes** (so the dashboard can
     skip a separate `/userinfo` call when email is in the ID token)

## Keycloak-specific notes

1. Realm → Clients → Create
2. Client Type: **OpenID Connect**, Client ID: `3xui-dashboard`
3. Capability config:
   - **Standard flow**: ON (authorization code)
   - **Direct access grants**: OFF
   - **Service accounts**: OFF
4. Valid redirect URIs: `https://dashboard.example.com/oidc/callback`
5. Credentials tab → copy the **Client secret**
6. Client scopes — confirm `openid`, `profile`, `email` are in the
   defaults (Keycloak ships them by default)

## Flow

```
User                Frontend          Backend           IDP
 │                    │                 │                │
 ├──click "登录"─────►│                 │                │
 │                    ├─POST /oidc/start─►                │
 │                    │  with redirect_after             │
 │                    │◄──{authorize_url}─                │
 │                    ├──navigate browser──────────────►│
 │                    │                 │                ├─auth user
 │                    │                 │                ├─consent
 │                    │◄────────redirect ?code=&state=───┤
 │  /oidc/callback   │                 │                │
 │                    ├─POST /oidc/callback─►            │
 │                    │  {code, state}                   │
 │                    │              ├─token exchange──►│
 │                    │              │◄──id_token───────┤
 │                    │              ├─verify via JWKS  │
 │                    │              ├─resolve user by email
 │                    │◄──{token} or {pending decision}─ │
 │  /portal/...      │                 │                │
```

## How the dashboard verifies the ID token

1. Parse JWT header → extract `kid`
2. Fetch JWKS doc from `OIDC_JWKS_URL` (or discovered URL); cache
   1 hour in-process
3. Look up the matching public key by `kid`; refetch JWKS once on
   miss to handle key rotation
4. Verify signature (RS256 / RS384 / RS512 only — EC tokens are
   rejected; most IDPs default to RS256)
5. Validate standard claims: `iss == OIDC_ISSUER`,
   `aud contains OIDC_CLIENT_ID`, `exp > now`
6. Extract `email` + `sub` claims. Email is the canonical identity;
   `sub` is stored only as an OIDC login credential on that email row.

## Email-first account resolution

- New OIDC email: create one user row with that email and OIDC subject.
- Returning OIDC subject with the same email: log into the existing row.
- OIDC email already exists but is not linked to this subject: the
  callback returns a short-lived pending decision. The browser asks the
  user whether to bind this OIDC login to the existing email account or
  recreate/reset that email identity.
- Missing email claim: rejected. Configure the IDP scopes/claims so
  `email` is present; the dashboard intentionally avoids email-less
  users because email is the unique user identity.

## Limitations

- **PKCE is always sent**. Even when a client secret is also set
  the dashboard includes the verifier — no harm and supports
  IDPs configured for PKCE-only.
- **Single IDP at a time**. The config has one OIDC block; if
  you need multi-provider login, file an issue.
- **State store is in-memory**. Restart the dashboard during an
  in-flight login and the user gets `state mismatch or expired`
  + has to re-click login. This is fine for single-instance
  deployments; multi-instance needs a DB-backed store (not yet
  implemented).
- **Email verification claim is honored**. If the ID token has
  `email_verified: true` the dashboard marks the user's email
  as verified. Without it, the email column is populated but
  `email_verified=false`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `OIDC not configured` 501 | One of OIDC_ISSUER / CLIENT_ID / REDIRECT_URL is empty. Check `cfg.OIDC.Enabled()` (all four required by config.go) |
| `state mismatch or expired` 400 | User took >10min, or browser blocked cookies, or the dashboard restarted mid-flow |
| `id_token verification failed` 401 | JWKS fetch failing, or token's iss/aud doesn't match config, or signing method is not RS{256,384,512} |
| `jwks: no key for kid=…` | IDP rotated keys faster than the 1h cache; refetch retry should fire once — if you see this twice, the kid is genuinely missing from JWKS |
| Login button doesn't appear | `OIDC_DISPLAY_NAME` empty AND issuer URL doesn't parse — backend returns `[]` to `/auth/oidc/providers` |
