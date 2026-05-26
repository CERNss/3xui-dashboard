# OIDC T2 notes — full browser chain (2026-05-21)

Follow-up to `oidc-t1-notes.md`. T1 verified the headless half
(discovery doc, authorize URL construction, JWKS shape); T2 closes
the remaining gap by driving a real Chromium through the full
code → token → JWKS verify → users upsert chain against the same
Zitadel v2.71.10 stack.

## Stack

- Dashboard: locally-built `./3xui-dashboard` on `:8080`, fresh
  `dashboard_t2` db on `pg-demo` (port 5495).
- Zitadel: `ghcr.io/zitadel/zitadel:v2.71.10` on `:8088`, same
  stack as T1 (`/tmp/zitadel-stack/docker-compose.yml`).
- OIDC client app reused from T1 (client_id `373880820596801539`,
  redirect_uri `http://localhost:8080/oidc/callback`, devMode=true).
- Driver: `playwright@latest` + headless Chromium (one-shot,
  install at `/tmp/oidc-t2-pw/`, NOT committed).

Dashboard env (`/tmp/dashboard-t2.env`):

```ini
LISTEN_ADDR=:8080
DATABASE_URL=postgres://postgres:demo@localhost:5495/dashboard_t2?sslmode=disable
OIDC_ISSUER=http://localhost:8088
OIDC_CLIENT_ID=373880820596801539
OIDC_CLIENT_SECRET=<from /tmp/zitadel-stack/oidc.env>
OIDC_REDIRECT_URL=http://localhost:8080/oidc/callback
OIDC_SCOPES=openid,profile,email
OIDC_DISPLAY_NAME=Zitadel T2
```

## What the spec exercises

`/tmp/oidc-t2-pw/oidc-flow.mjs` runs the same flow twice in fresh
browser contexts:

1. `GET /login` — dashboard renders the SSO button list from
   `/api/user/auth/oidc/providers`.
2. Click "Zitadel T2" → `POST /api/user/auth/oidc/start` →
   navigate to the returned `authorize_url`.
3. Zitadel login UI (v2.71.10 is a 2-step form):
   - Page 1 `/ui/login/login` → fill `loginName` → POST
   - Page 2 `/ui/login/password` → fill password → POST
4. **MFA prompt** at `/ui/login/mfa/prompt`: Zitadel default
   policy forces MFA setup on first login. The spec clicks the
   page's own **Skip** button (`button[name="skip"]`) to bypass.
   On the second run the prompt doesn't reappear because Zitadel
   remembers the dismissal.
5. Redirect back to dashboard `/oidc/callback?code=&state=` →
   the SPA's OIDC callback view calls
   `POST /api/user/auth/oidc/callback` to exchange the code.
6. Server-side: `oidcCallbackImpl` → token exchange →
   `verifyIDToken` (JWKS fetched + RS256 verify) → users upsert
   keyed on `oidc_subject` → portal JWT minted → returned to SPA.
7. SPA stashes the JWT in `localStorage` under
   `dashboard.portal.token` and navigates to `/portal`.

## Verified ✅

- **Code exchange round-trip**: dashboard's
  `POST /oauth/v2/token` with `code` + `code_verifier` returned a
  valid id_token. The JWKS verifier accepted the RS256 signature.
- **users upsert by oidc_subject**: after two parallel logins
  (fresh browser context each, same Zitadel account), exactly
  one row exists in `users`:

  ```
   id |           email           |    oidc_subject    | status
   ---+---------------------------+--------------------+--------
    1 | zitadel-admin@example.com | 373880778671194115 | active
  ```

  Second login found the existing row by `oidc_subject` — no
  duplicate inserted, the unique partial index held.
- **Portal JWT issued + localStorage hydrated**: 192-byte HS256
  token under `dashboard.portal.token`, recognized by the auth
  guard which then allowed `/portal/subscription` to render.
- **State + PKCE cookie / param round-trip**: the same browser
  context that received `state` from `/oidc/start` returned it
  intact to `/oidc/callback`; the in-memory state store on the
  server consumed it correctly. No CSRF errors in Zitadel's log
  during the runs.

## Non-issues observed but mitigated

- **First-login MFA enforcement**: Zitadel's default policy
  forces `2-Factor Setup`. The login UI ships a Skip button that
  ends the auth request as if MFA were not required. Operators
  rolling this out to real users SHOULD either disable the MFA
  prompt at the org-policy level OR pre-enroll users with a
  factor. The dashboard side doesn't care either way — its
  callback handler runs identically regardless.
- **`Errors.AuthRequest.NotFound`** seen in Zitadel logs during
  earlier curl-based T1 attempts: those were the result of CSRF
  / cookie context loss across curl invocations. Browser-driven
  runs do NOT produce this error because the `gorilla.csrf.Token`
  hidden field + session cookie travel together inside the same
  context.

## Cleanup

```bash
pkill -9 dashboard
docker exec pg-demo psql -U postgres -c "DROP DATABASE dashboard_t2"
# /tmp/oidc-t2-pw is ephemeral — left in place for re-runs.
# /tmp/zitadel-stack stays up for re-use; tear down with:
#   docker compose -f /tmp/zitadel-stack/docker-compose.yml down -v
```

## What still isn't automated

The spec drives one user. It does NOT exercise:

- **Multiple distinct OIDC identities sharing an email**: the
  dashboard is now email-first. A different OIDC identity claiming
  the same email receives a pending decision in the callback; the
  browser asks whether to bind or recreate/reset that email identity.
- **id_token claim mismatches**: bad `iss`, missing `aud`,
  expired tokens — these are covered by Go unit tests in
  `oidc_test.go`, not by this spec.
- **Provider misconfiguration**: invalid `OIDC_REDIRECT_URL`,
  missing scopes, etc. Covered by the cfg.OIDC.Enabled() boot
  check and the user-facing error in the OIDC callback view.

These are explicit T3 candidates if real-user reports surface
any of them.
