// OIDC login flow — authorization code with PKCE, ID token
// verification via the IDP's JWKS, and user resolution keyed by
// email. `sub` is stored as a login credential on the canonical
// email account; it is not the primary identity. Stdlib +
// golang-jwt/jwt/v5 only; no oidc SDK to keep the dependency
// surface small and auditable.
//
// Wiring: handlers/user/auth.go calls Service.OIDCStart to get the
// authorize URL, then Service.OIDCCallback once the IDP comes back
// with code + state. The Service holds a 10-minute in-memory state
// store keyed on the random `state` parameter — this is fine for
// the single-instance dashboard topology; if we ever scale out,
// move the store into Postgres with an expires_at column.
package user

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// ErrOIDCStateInvalid fires when callback's state doesn't match any
// stored start. Most common cause: user took >10min to log in (the
// store TTL); occasionally a CSRF attempt or two parallel logins
// stepping on each other. Surface verbatim to the caller — the
// handler maps to 400.
var ErrOIDCStateInvalid = errors.New("oidc: state mismatch or expired")

// ErrOIDCBadIDToken means the IDP returned a token the dashboard
// couldn't verify (signature / iss / aud / exp). Treat as auth
// failure → 401 at the handler.
var ErrOIDCBadIDToken = errors.New("oidc: id_token verification failed")

// ErrOIDCEmailConflict fires when an OIDC login arrives with an
// email that's already bound to a different OIDC subject. Because
// email is the canonical user identity, the dashboard requires an
// explicit user decision instead of silently creating a duplicate.
// Handler maps it to 409.
var ErrOIDCEmailConflict = errors.New("oidc: email already linked to a different account")

// oidcState is one in-flight login: state parameter + PKCE verifier
// + post-login redirect target.
type oidcState struct {
	verifier      string
	redirectAfter string
	linkUserID    int64
	expiresAt     time.Time
}

// OIDCLoginResult is returned by OIDCCallback. Exactly one of User or
// Pending is set. Pending means the IDP identity is valid, but the
// email already exists and the frontend must ask the user whether to
// bind to that existing account or reset/recreate that email identity.
type OIDCLoginResult struct {
	User          *model.User
	Pending       *OIDCPendingDecision
	RedirectAfter string
}

// OIDCPendingDecision is safe to serialize to the browser. It does
// not expose the OIDC subject; the short-lived token points at the
// server-side pending record.
type OIDCPendingDecision struct {
	Token           string
	Email           string
	EmailVerified   bool
	ExistingUserID  int64
	ExistingHasOIDC bool
	ExpiresAt       time.Time
}

type oidcPending struct {
	sub            string
	email          string
	emailVerified  bool
	redirectAfter  string
	existingUserID int64
	expiresAt      time.Time
}

type oidcPendingSessions struct {
	mu sync.Mutex
	m  map[string]*oidcPending
}

func newOIDCPendingSessions() *oidcPendingSessions {
	return &oidcPendingSessions{m: map[string]*oidcPending{}}
}

func (s *oidcPendingSessions) put(token string, p *oidcPending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.m[token] = p
}

func (s *oidcPendingSessions) take(token string) *oidcPending {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	p, ok := s.m[token]
	if !ok {
		return nil
	}
	delete(s.m, token)
	if p.expiresAt.Before(time.Now()) {
		return nil
	}
	return p
}

func (s *oidcPendingSessions) sweepLocked() {
	now := time.Now()
	for k, v := range s.m {
		if v.expiresAt.Before(now) {
			delete(s.m, k)
		}
	}
}

// oidcSessions holds the short-lived state map. Construct via
// newOIDCSessions; it self-sweeps stale entries on every access.
type oidcSessions struct {
	mu sync.Mutex
	m  map[string]*oidcState
}

func newOIDCSessions() *oidcSessions {
	return &oidcSessions{m: map[string]*oidcState{}}
}

func (s *oidcSessions) put(state string, st *oidcState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.m[state] = st
}

// take removes-and-returns the entry, so each state can only be
// consumed once (replay defense).
func (s *oidcSessions) take(state string) *oidcState {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	st, ok := s.m[state]
	if !ok {
		return nil
	}
	delete(s.m, state)
	if st.expiresAt.Before(time.Now()) {
		return nil
	}
	return st
}

func (s *oidcSessions) sweepLocked() {
	now := time.Now()
	for k, v := range s.m {
		if v.expiresAt.Before(now) {
			delete(s.m, k)
		}
	}
}

// oidcDiscovery is the relevant subset of an IDP's
// /.well-known/openid-configuration document.
type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

func (s *Service) effectiveOIDC(ctx context.Context) (config.OIDC, error) {
	oidc := s.cfg.OIDC
	if s.settings == nil {
		return oidc, nil
	}
	var err error
	read := func(key, fallback string) string {
		if err != nil {
			return fallback
		}
		var v string
		v, err = s.settings.GetString(ctx, key, fallback)
		v = strings.TrimSpace(v)
		if v == "" {
			return fallback
		}
		return v
	}
	oidc.Issuer = read(model.SettingOIDCIssuer, oidc.Issuer)
	oidc.ClientID = read(model.SettingOIDCClientID, oidc.ClientID)
	oidc.ClientSecret = read(model.SettingOIDCClientSecret, oidc.ClientSecret)
	oidc.RedirectURL = read(model.SettingOIDCRedirectURL, oidc.RedirectURL)
	scopesRaw := read(model.SettingOIDCScopes, strings.Join(oidc.Scopes, ","))
	if scopesRaw != "" {
		oidc.Scopes = splitOIDCScopes(scopesRaw)
	}
	oidc.DisplayName = read(model.SettingOIDCDisplayName, oidc.DisplayName)
	oidc.IconURL = read(model.SettingOIDCIconURL, oidc.IconURL)
	oidc.AuthURL = read(model.SettingOIDCAuthURL, oidc.AuthURL)
	oidc.TokenURL = read(model.SettingOIDCTokenURL, oidc.TokenURL)
	oidc.JWKSURL = read(model.SettingOIDCJWKSURL, oidc.JWKSURL)
	oidc.UserURL = read(model.SettingOIDCUserInfoURL, oidc.UserURL)
	if err != nil {
		return oidc, err
	}
	return oidc, nil
}

func splitOIDCScopes(raw string) []string {
	raw = strings.ReplaceAll(raw, " ", ",")
	parts := strings.Split(raw, ",")
	out := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// resolveEndpoints returns the URLs the dashboard needs, hitting
// discovery only when an explicit override is missing. Caches the
// discovery result in-process so we don't refetch on every login.
func (s *Service) resolveEndpoints(ctx context.Context, oidc config.OIDC) (oidcDiscovery, error) {
	s.oidcDiscoMu.Lock()
	defer s.oidcDiscoMu.Unlock()

	// Start from explicit overrides; only fetch discovery for the
	// remaining gaps. This lets operators run against IDPs whose
	// discovery doc is broken / behind auth.
	d := oidcDiscovery{
		AuthorizationEndpoint: oidc.AuthURL,
		TokenEndpoint:         oidc.TokenURL,
		UserInfoEndpoint:      oidc.UserURL,
		JWKSURI:               oidc.JWKSURL,
		Issuer:                oidc.Issuer,
	}
	if d.AuthorizationEndpoint != "" && d.TokenEndpoint != "" && d.JWKSURI != "" {
		s.oidcDiscoCache = &d
		s.oidcDiscoFetched = time.Now()
		return d, nil
	}
	// Cached + still fresh? (24h TTL — endpoints rotate rarely.)
	if s.oidcDiscoCache != nil && s.oidcDiscoCache.Issuer == oidc.Issuer && time.Since(s.oidcDiscoFetched) < 24*time.Hour {
		return *s.oidcDiscoCache, nil
	}
	if oidc.Issuer == "" {
		return d, errors.New("oidc: issuer not configured")
	}
	discoURL := strings.TrimRight(oidc.Issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoURL, nil)
	if err != nil {
		return d, fmt.Errorf("oidc disco: %w", err)
	}
	resp, err := s.oidcHTTP.Do(req)
	if err != nil {
		return d, fmt.Errorf("oidc disco fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return d, fmt.Errorf("oidc disco HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var fetched oidcDiscovery
	if err := json.Unmarshal(body, &fetched); err != nil {
		return d, fmt.Errorf("oidc disco decode: %w", err)
	}
	// Overlay: explicit cfg overrides fetched values.
	if d.AuthorizationEndpoint == "" {
		d.AuthorizationEndpoint = fetched.AuthorizationEndpoint
	}
	if d.TokenEndpoint == "" {
		d.TokenEndpoint = fetched.TokenEndpoint
	}
	if d.UserInfoEndpoint == "" {
		d.UserInfoEndpoint = fetched.UserInfoEndpoint
	}
	if d.JWKSURI == "" {
		d.JWKSURI = fetched.JWKSURI
	}
	if d.Issuer == "" {
		d.Issuer = fetched.Issuer
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return d, fmt.Errorf("oidc disco: missing required endpoints (auth=%q token=%q jwks=%q)",
			d.AuthorizationEndpoint, d.TokenEndpoint, d.JWKSURI)
	}
	s.oidcDiscoCache = &d
	s.oidcDiscoFetched = time.Now()
	return d, nil
}

// oidcStartImpl is the real implementation. Returns the IDP's
// authorize URL the frontend should navigate the user to.
func (s *Service) oidcStartImpl(ctx context.Context, oidc config.OIDC, redirectAfter string, linkUserID int64) (string, error) {
	disco, err := s.resolveEndpoints(ctx, oidc)
	if err != nil {
		return "", err
	}
	state, err := randomURLString(32)
	if err != nil {
		return "", err
	}
	// PKCE per RFC 7636 — 43-char verifier, S256 challenge.
	verifier, err := randomURLString(43)
	if err != nil {
		return "", err
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	s.oidcSessions.put(state, &oidcState{
		verifier:      verifier,
		redirectAfter: redirectAfter,
		linkUserID:    linkUserID,
		expiresAt:     time.Now().Add(10 * time.Minute),
	})

	scopes := strings.Join(oidc.Scopes, " ")
	if scopes == "" {
		scopes = "openid profile email"
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", oidc.ClientID)
	q.Set("redirect_uri", oidc.RedirectURL)
	q.Set("scope", scopes)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	sep := "?"
	if strings.Contains(disco.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	return disco.AuthorizationEndpoint + sep + q.Encode(), nil
}

// tokenResponse is the relevant subset of /token's response.
type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// oidcCallbackImpl exchanges code → tokens, verifies the id_token,
// resolves the user row by canonical email, and returns either a
// logged-in user or a pending decision for the frontend.
func (s *Service) oidcCallbackImpl(ctx context.Context, oidc config.OIDC, code, state string) (*OIDCLoginResult, error) {
	st := s.oidcSessions.take(state)
	if st == nil {
		return nil, ErrOIDCStateInvalid
	}
	disco, err := s.resolveEndpoints(ctx, oidc)
	if err != nil {
		return nil, err
	}

	// Token exchange — form-encoded POST per OIDC spec.
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", oidc.RedirectURL)
	form.Set("client_id", oidc.ClientID)
	form.Set("code_verifier", st.verifier)
	if oidc.ClientSecret != "" {
		form.Set("client_secret", oidc.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disco.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oidc token req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.oidcHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	var tok oidcTokenResponse
	if jerr := json.Unmarshal(body, &tok); jerr != nil {
		return nil, fmt.Errorf("oidc token decode: %w (body=%s)", jerr, snippet(body))
	}
	if tok.Error != "" {
		return nil, fmt.Errorf("oidc token error %s: %s", tok.Error, tok.ErrorDesc)
	}
	if tok.IDToken == "" {
		return nil, fmt.Errorf("oidc: id_token missing in token response")
	}

	// Verify id_token signature against the IDP's JWKS + standard
	// claim checks (iss / aud / exp).
	claims, err := s.verifyIDToken(ctx, tok.IDToken, oidc, disco)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOIDCBadIDToken, err)
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("oidc: id_token missing sub claim")
	}
	email, _ := claims["email"].(string)
	emailVerifiedClaim, _ := claims["email_verified"].(bool)

	if st.linkUserID > 0 {
		return s.linkOIDCToUser(ctx, st.linkUserID, sub, email, emailVerifiedClaim, st.redirectAfter)
	}
	return s.upsertOIDCUser(ctx, sub, email, emailVerifiedClaim, st.redirectAfter)
}

// upsertOIDCUser resolves verified IDP claims to a User row.
// Identity is email-first: returning OIDC users are accepted only if
// their subject still points at the same email row; new OIDC subjects
// with an existing email become pending user decisions.
func (s *Service) upsertOIDCUser(ctx context.Context, sub, email string, emailVerified bool, redirectAfter string) (*OIDCLoginResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrOIDCEmailRequired
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, ErrDomainNotAllowed
	}

	// Existing subject is allowed only while it still belongs to the
	// same canonical email. If the IDP changed the email claim, ask the
	// user to decide rather than silently moving identity.
	user, err := s.users.GetByOIDCSubject(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user != nil {
		if user.Email != nil && strings.EqualFold(*user.Email, email) {
			updates := map[string]any{}
			if emailVerified && !user.EmailVerified {
				updates["email_verified"] = true
			}
			if err := s.users.Update(ctx, user.ID, updates); err != nil {
				return nil, err
			}
			u, err := s.users.Get(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			return &OIDCLoginResult{User: u, RedirectAfter: redirectAfter}, nil
		}
		if user.Email == nil {
			if err := s.users.Update(ctx, user.ID, map[string]any{
				"email":          email,
				"email_verified": emailVerified,
			}); err != nil {
				return nil, err
			}
			u, err := s.users.Get(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			return &OIDCLoginResult{User: u, RedirectAfter: redirectAfter}, nil
		}
		existing, err := s.users.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return s.makeOIDCPendingForUser(ctx, existing, sub, email, emailVerified, redirectAfter)
		}
		updates := map[string]any{"email": email, "email_verified": emailVerified}
		if err := s.users.Update(ctx, user.ID, updates); err != nil {
			return nil, err
		}
		u, err := s.users.Get(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		return &OIDCLoginResult{User: u, RedirectAfter: redirectAfter}, nil
	}

	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.makeOIDCPendingForUser(ctx, existing, sub, email, emailVerified, redirectAfter)
	}

	subID, err := generateSubID()
	if err != nil {
		return nil, err
	}
	created := &model.User{
		Email:         &email,
		EmailVerified: emailVerified,
		OIDCSubject:   &sub,
		SubID:         subID,
		Status:        model.UserStatusActive,
	}
	if err := s.users.Create(ctx, created); err != nil {
		return nil, err
	}
	if err := s.applyNewUserInitialBalance(ctx, created, "new OIDC user initial balance"); err != nil {
		return nil, err
	}
	s.bus.PublishType(event.UserRegistered, RegisteredPayload{
		UserID: created.ID,
		Email:  email,
	})
	return &OIDCLoginResult{User: created, RedirectAfter: redirectAfter}, nil
}

func (s *Service) makeOIDCPendingForUser(_ context.Context, existing *model.User, sub, email string, emailVerified bool, redirectAfter string) (*OIDCLoginResult, error) {
	if existing.OIDCSubject != nil && *existing.OIDCSubject == sub {
		return &OIDCLoginResult{User: existing, RedirectAfter: redirectAfter}, nil
	}
	token, err := randomURLString(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	s.oidcPending.put(token, &oidcPending{
		sub:            sub,
		email:          email,
		emailVerified:  emailVerified,
		redirectAfter:  redirectAfter,
		existingUserID: existing.ID,
		expiresAt:      expiresAt,
	})
	hasOIDC := existing.OIDCSubject != nil && *existing.OIDCSubject != ""
	return &OIDCLoginResult{
		Pending: &OIDCPendingDecision{
			Token:           token,
			Email:           email,
			EmailVerified:   emailVerified,
			ExistingUserID:  existing.ID,
			ExistingHasOIDC: hasOIDC,
			ExpiresAt:       expiresAt,
		},
		RedirectAfter: redirectAfter,
	}, nil
}

func (s *Service) linkOIDCToUser(ctx context.Context, userID int64, sub, email string, emailVerified bool, redirectAfter string) (*OIDCLoginResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrOIDCEmailRequired
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, ErrDomainNotAllowed
	}

	var got *model.User
	err := s.users.InTx(ctx, func(tx *repository.UserRepo) error {
		u, err := tx.GetForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if u == nil {
			return ErrUserNotFound
		}
		if u.Status == model.UserStatusSuspended {
			return ErrUserSuspended
		}
		if u.OIDCSubject != nil && *u.OIDCSubject != "" && *u.OIDCSubject != sub {
			return ErrOIDCEmailConflict
		}

		if linked, err := tx.GetByOIDCSubject(ctx, sub); err != nil {
			return err
		} else if linked != nil && linked.ID != userID {
			return ErrOIDCEmailConflict
		}

		updates := map[string]any{"oidc_subject": sub}
		if u.Email != nil && *u.Email != "" {
			if !strings.EqualFold(*u.Email, email) {
				return ErrOIDCEmailMismatch
			}
		} else {
			existing, err := tx.GetByEmail(ctx, email)
			if err != nil {
				return err
			}
			if existing != nil && existing.ID != userID {
				return ErrEmailTaken
			}
			updates["email"] = email
			updates["email_verified"] = emailVerified
		}
		if emailVerified && !u.EmailVerified {
			updates["email_verified"] = true
		}
		if err := tx.Update(ctx, userID, updates); err != nil {
			return err
		}
		got, err = tx.Get(ctx, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &OIDCLoginResult{User: got, RedirectAfter: redirectAfter}, nil
}

func (s *Service) resolveOIDCPending(ctx context.Context, pendingToken, action string) (*model.User, string, error) {
	p := s.oidcPending.take(strings.TrimSpace(pendingToken))
	if p == nil {
		return nil, "", ErrOIDCPendingInvalid
	}
	existing, err := s.users.Get(ctx, p.existingUserID)
	if err != nil {
		return nil, "", err
	}
	if existing == nil || existing.Email == nil || !strings.EqualFold(*existing.Email, p.email) {
		return nil, "", ErrOIDCPendingInvalid
	}
	switch action {
	case "bind":
		if existing.OIDCSubject != nil && *existing.OIDCSubject != "" && *existing.OIDCSubject != p.sub {
			return nil, "", ErrOIDCEmailConflict
		}
		updates := map[string]any{"oidc_subject": p.sub}
		if p.emailVerified && !existing.EmailVerified {
			updates["email_verified"] = true
		}
		if err := s.users.Update(ctx, existing.ID, updates); err != nil {
			return nil, "", err
		}
	case "recreate":
		newSubID, err := generateSubID()
		if err != nil {
			return nil, "", err
		}
		updates := map[string]any{
			"oidc_subject":  p.sub,
			"password_hash": nil,
			"sub_id":        newSubID,
			"status":        model.UserStatusActive,
		}
		if p.emailVerified {
			updates["email_verified"] = true
		}
		if err := s.users.Update(ctx, existing.ID, updates); err != nil {
			return nil, "", err
		}
	default:
		return nil, "", ErrOIDCActionInvalid
	}
	u, err := s.users.Get(ctx, existing.ID)
	if err != nil {
		return nil, "", err
	}
	return u, p.redirectAfter, nil
}

// verifyIDToken parses, fetches JWKS, verifies signature, and
// validates iss/aud/exp. Returns the token claims as a map.
func (s *Service) verifyIDToken(ctx context.Context, raw string, oidc config.OIDC, disco oidcDiscovery) (jwt.MapClaims, error) {
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return s.lookupJWK(ctx, disco.JWKSURI, kid)
	},
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithAudience(oidc.ClientID),
		jwt.WithIssuer(disco.Issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// jwksDoc is the relevant subset of /.well-known/jwks.json.
type jwksDoc struct {
	Keys []jwksKey `json:"keys"`
}
type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg,omitempty"`
	Use string `json:"use,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// lookupJWK returns the RSA public key with the given kid. The
// JWKS doc is cached for 1 hour to limit per-login JWKS GETs;
// on kid miss we force a refetch (handles key rotation).
func (s *Service) lookupJWK(ctx context.Context, jwksURL, kid string) (*rsa.PublicKey, error) {
	s.oidcJWKSMu.Lock()
	defer s.oidcJWKSMu.Unlock()

	if s.oidcJWKSCache == nil || s.oidcJWKSURL != jwksURL || time.Since(s.oidcJWKSFetched) > time.Hour {
		if err := s.refreshJWKSLocked(ctx, jwksURL); err != nil {
			return nil, err
		}
	}
	pub := s.oidcJWKSCache[kid]
	if pub != nil {
		return pub, nil
	}
	// Cache miss on a kid — IDP may have rotated. Refetch once.
	if err := s.refreshJWKSLocked(ctx, jwksURL); err != nil {
		return nil, err
	}
	pub = s.oidcJWKSCache[kid]
	if pub == nil {
		return nil, fmt.Errorf("jwks: no key for kid=%q", kid)
	}
	return pub, nil
}

func (s *Service) refreshJWKSLocked(ctx context.Context, jwksURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.oidcHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var doc jwksDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("jwks decode: %w", err)
	}
	out := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k)
		if err != nil {
			continue
		}
		out[k.Kid] = pub
	}
	s.oidcJWKSCache = out
	s.oidcJWKSURL = jwksURL
	s.oidcJWKSFetched = time.Now()
	return nil
}

// jwkToRSAPublicKey decodes the base64url n + e fields into an
// *rsa.PublicKey. EC keys (kty="EC") are not supported in v1 —
// most IDPs default to RS256 and the dashboard's WithValidMethods
// list above already rejects ES tokens.
func jwkToRSAPublicKey(k jwksKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("jwk n decode: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("jwk e decode: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, errors.New("jwk e is zero")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// randomURLString returns n bytes of crypto/rand, base64url-
// encoded WITHOUT padding. The output length grows ~4/3 of n.
func randomURLString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// snippet trims body for inclusion in error messages.
func snippet(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}

func OIDCDisplayName(oidc config.OIDC) string {
	if strings.TrimSpace(oidc.DisplayName) != "" {
		return strings.TrimSpace(oidc.DisplayName)
	}
	if u, err := url.Parse(oidc.Issuer); err == nil && u.Host != "" {
		return u.Host
	}
	return "OIDC"
}
