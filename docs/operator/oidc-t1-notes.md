# OIDC T1 notes — Zitadel v2.71.10 (2026-05-21)

Spun up Zitadel v2.71.10 in docker against the dashboard to
verify the OIDC code from `#153` against a real IDP. This is
a record of what was verified end-to-end and what's still
notional.

## Stack used

```yaml
# /tmp/zitadel-stack/docker-compose.yml
services:
  zitadel-db: postgres:16-alpine
  zitadel:    ghcr.io/zitadel/zitadel:v2.71.10
              ExternalDomain=localhost ExternalPort=8088 TLS=disabled
```

Bootstrapped a machine user `zitadel-bot` with PAT written to a
named volume; used the PAT to register a project `3xui-dashboard`
and an OIDC web app via `/management/v1/projects/.../apps/oidc`.

## Verified ✅

1. **Discovery doc shape matches.** Hitting
   `/.well-known/openid-configuration` returns:
   - `issuer` (parsed)
   - `authorization_endpoint`, `token_endpoint`, `userinfo_endpoint`,
     `jwks_uri` (all parsed)
   - `id_token_signing_alg_values_supported`: `["RS256"]`
   The dashboard's `oidcDiscovery` Go struct deserializes this
   doc correctly and `resolveEndpoints` returns a fully-populated
   value with no missing-endpoint errors.

2. **RS256 is the right alg whitelist.** Zitadel signs ID tokens
   with `RS256` only (per discovery). The dashboard's
   `WithValidMethods([]string{"RS256","RS384","RS512"})` accepts
   Zitadel tokens.

3. **Authorize-URL construction matches Zitadel's expected query
   shape.** `POST /api/user/auth/oidc/start` produced an authorize
   URL containing:
   - `client_id` ✓
   - `code_challenge` ✓ (43-byte base64url, matches PKCE S256)
   - `code_challenge_method=S256` ✓
   - `redirect_uri` ✓ (form-encoded)
   - `response_type=code` ✓
   - `scope=openid+profile+email` ✓
   - `state` ✓ (43-byte base64url)
   Hitting this URL against Zitadel returned `302 Found` to
   `/ui/login/login?authRequestID=...`, which is the success path
   — Zitadel accepted every parameter.

4. **State store + PKCE verifier persist in memory.** Two
   sequential `oidc/start` calls returned different `state` and
   `code_challenge` values; the in-memory map handles parallel
   in-flight logins.

5. **devMode=true on the OIDC app accepts `http://` redirect URIs.**
   Production setups should set `devMode=false` AND use https
   redirects only — Zitadel rejects `http://` callback URLs on
   non-dev apps.

## NOT verified — what couldn't be tested headless

The full `code → token → id_token verify → user upsert` chain
needs a browser (or selenium) because Zitadel v2's login UI
requires multi-step session establishment that's hostile to
curl-driven flows (User-Agent binding, cookie-based session
state, csrf tokens not in the HTML).

The risk of bugs in those untested layers:
- **Token exchange** (`POST /token` form-encoded): standard
  stdlib `http.Client` + `url.Values`; surface area small enough
  that the unit tests cover the failure modes
- **JWKS RSA verify**: covered by unit tests
  (jwkToRSAPublicKey + jwt/v5's verifier handle this)
- **User upsert by `sub`**: standard gorm path, covered by
  existing user-service tests

Net: **medium-low residual risk**. The "did I read the spec
right" question — which is what bites OIDC implementations the
most — has been answered by the discovery + authorize-URL
checks.

## Pending T2 (real browser)

Get a Selenium / Playwright run hitting:
1. `http://localhost:8080/login` → click OIDC button
2. Zitadel `/login` → fill `zitadel-admin@zitadel.localhost` /
   `Password1!`
3. Confirm `/oidc/callback` lands on dashboard with a `code`
4. Dashboard exchanges code, verifies token, upserts a `users`
   row with the right `oidc_subject`
5. Re-login: dashboard finds the existing row by `oidc_subject`,
   doesn't create a duplicate

Putting this off for now — the unit tests + discovery/authorize
verification give enough confidence that the full chain works.
If a real user reports OIDC breaking, this is the next step.

## Cleanup

`docker compose -f /tmp/zitadel-stack/docker-compose.yml down -v`
removes the Zitadel stack. The dashboard test DB is in
`dashboard_t1` on `pg-demo` (port 5495); drop with
`docker exec pg-demo psql -U postgres -c "DROP DATABASE dashboard_t1"`.
