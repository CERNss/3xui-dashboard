// OIDC login flow — authorization code with PKCE, ID token
// verification via the IDP's JWKS, and user resolution via
// provider-scoped identities. Stdlib + golang-jwt/jwt/v5 only; no
// oidc SDK to keep the dependency surface small and auditable.
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
)

const defaultOIDCProviderKey = "default"

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
	providerKey   string
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
	User          *model.User          `json:"-"`
	Pending       *OIDCPendingDecision `json:"pending,omitempty"`
	RedirectAfter string               `json:"redirect_after,omitempty"`
}

// OIDCPendingDecision is safe to serialize to the browser. It does
// not expose the OIDC subject; the short-lived token points at the
// server-side pending record.
type OIDCPendingDecision struct {
	Token               string    `json:"token"`
	ProviderKey         string    `json:"provider_key"`
	ProviderDisplayName string    `json:"provider_display_name"`
	ProviderIconURL     string    `json:"provider_icon_url,omitempty"`
	Email               string    `json:"email"`
	EmailVerified       bool      `json:"email_verified"`
	ExistingUserID      int64     `json:"existing_user_id,omitempty"`
	ExistingUser        bool      `json:"existing_user"`
	ExistingHasOIDC     bool      `json:"existing_has_oidc"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type oidcPending struct {
	providerKey         string
	providerDisplayName string
	providerIconURL     string
	sub                 string
	email               string
	emailVerified       bool
	redirectAfter       string
	existingUserID      int64
	existingHasOIDC     bool
	expiresAt           time.Time
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

func (s *oidcPendingSessions) get(token string) *oidcPending {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	p, ok := s.m[token]
	if !ok || p.expiresAt.Before(time.Now()) {
		if ok {
			delete(s.m, token)
		}
		return nil
	}
	cp := *p
	return &cp
}

func (s *oidcPendingSessions) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, token)
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

func oidcFromProvider(p model.OIDCProvider) config.OIDC {
	return config.OIDC{
		Issuer:       p.Issuer,
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURL:  p.RedirectURL,
		Scopes:       []string(p.Scopes),
		AuthURL:      p.AuthURL,
		TokenURL:     p.TokenURL,
		JWKSURL:      p.JWKSURL,
		UserURL:      p.UserInfoURL,
		DisplayName:  p.DisplayName,
		IconURL:      p.IconURL,
	}
}

func providerFromOIDC(providerKey string, oidc config.OIDC) model.OIDCProvider {
	return model.OIDCProvider{
		ProviderKey:  providerKey,
		DisplayName:  OIDCDisplayName(oidc),
		IconURL:      strings.TrimSpace(oidc.IconURL),
		Issuer:       strings.TrimSpace(oidc.Issuer),
		ClientID:     strings.TrimSpace(oidc.ClientID),
		ClientSecret: strings.TrimSpace(oidc.ClientSecret),
		RedirectURL:  strings.TrimSpace(oidc.RedirectURL),
		Scopes:       model.StringSlice(oidc.Scopes),
		AuthURL:      strings.TrimSpace(oidc.AuthURL),
		TokenURL:     strings.TrimSpace(oidc.TokenURL),
		JWKSURL:      strings.TrimSpace(oidc.JWKSURL),
		UserInfoURL:  strings.TrimSpace(oidc.UserURL),
		Enabled:      oidc.Enabled(),
	}
}

func (s *Service) defaultOIDCProvider(ctx context.Context) (*model.OIDCProvider, error) {
	if s.users != nil {
		if p, err := s.users.GetOIDCProvider(ctx, defaultOIDCProviderKey); err != nil {
			return nil, err
		} else if p != nil {
			return p, nil
		}
	}
	oidc, err := s.effectiveOIDC(ctx)
	if err != nil {
		return nil, err
	}
	if !oidc.Enabled() {
		return nil, nil
	}
	p := providerFromOIDC(defaultOIDCProviderKey, oidc)
	return &p, nil
}

func (s *Service) getOIDCProvider(ctx context.Context, providerKey string) (*model.OIDCProvider, error) {
	providerKey = strings.TrimSpace(providerKey)
	if providerKey == "" {
		return nil, ErrOIDCProviderRequired
	}
	if s.users != nil {
		if p, err := s.users.GetOIDCProvider(ctx, providerKey); err != nil {
			return nil, err
		} else if p != nil {
			if !p.Enabled {
				return nil, ErrOIDCProviderNotFound
			}
			return p, nil
		}
	}
	if providerKey == defaultOIDCProviderKey {
		if p, err := s.defaultOIDCProvider(ctx); err != nil {
			return nil, err
		} else if p != nil && p.Enabled {
			return p, nil
		}
	}
	return nil, ErrOIDCProviderNotFound
}

func (s *Service) ensureOIDCProvider(ctx context.Context, p model.OIDCProvider) error {
	if s.users == nil {
		return nil
	}
	return s.users.UpsertOIDCProvider(ctx, &p)
}

// ListOIDCProviders returns enabled providers with optional linked
// state for a user profile. It includes the env/runtime single-provider
// fallback as "default" until admins move config into oidc_providers.
func (s *Service) ListOIDCProviders(ctx context.Context, userID int64) ([]OIDCProviderView, error) {
	seen := map[string]struct{}{}
	var providers []model.OIDCProvider
	if s.users != nil {
		rows, err := s.users.ListOIDCProviders(ctx, repository.OIDCProviderFilter{EnabledOnly: true})
		if err != nil {
			return nil, err
		}
		providers = append(providers, rows...)
		for _, p := range rows {
			seen[p.ProviderKey] = struct{}{}
		}
	}
	if _, ok := seen[defaultOIDCProviderKey]; !ok {
		p, err := s.defaultOIDCProvider(ctx)
		if err != nil {
			return nil, err
		}
		if p != nil && p.Enabled {
			providers = append(providers, *p)
		}
	}

	linkedByProvider := map[string]model.UserOIDCIdentity{}
	if userID > 0 && s.users != nil {
		linked, err := s.users.ListOIDCIdentities(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, identity := range linked {
			linkedByProvider[identity.ProviderKey] = identity
		}
	}
	out := make([]OIDCProviderView, 0, len(providers))
	for _, p := range providers {
		view := OIDCProviderView{
			ProviderKey: p.ProviderKey,
			DisplayName: p.DisplayName,
			IconURL:     strings.TrimSpace(p.IconURL),
			StartURL:    "/api/user/auth/oidc/start",
		}
		if identity, ok := linkedByProvider[p.ProviderKey]; ok {
			view.Linked = true
			view.ProviderEmail = identity.ProviderEmail
		}
		out = append(out, view)
	}
	return out, nil
}

// ListLinkedOIDCIdentities returns all provider identities linked to a
// local user account.
func (s *Service) ListLinkedOIDCIdentities(ctx context.Context, userID int64) ([]model.UserOIDCIdentity, error) {
	return s.users.ListOIDCIdentities(ctx, userID)
}

// FindOIDCIdentityByProviderSubject returns the account identity for
// an OIDC callback's provider subject pair.
func (s *Service) FindOIDCIdentityByProviderSubject(ctx context.Context, providerKey, subject string) (*model.UserOIDCIdentity, error) {
	return s.users.FindOIDCIdentity(ctx, providerKey, subject)
}

// LinkOIDCIdentityToUser links a verified provider identity to an
// existing user for callback/account-completion flows.
func (s *Service) LinkOIDCIdentityToUser(ctx context.Context, userID int64, providerKey, subject, providerEmail string, providerEmailVerified bool) (*model.UserOIDCIdentity, error) {
	providerKey = strings.TrimSpace(providerKey)
	subject = strings.TrimSpace(subject)
	providerEmail = strings.TrimSpace(strings.ToLower(providerEmail))
	if providerKey == "" {
		return nil, ErrOIDCProviderRequired
	}
	if subject == "" {
		return nil, fmt.Errorf("oidc: subject is required")
	}
	if providerEmail == "" {
		return nil, ErrOIDCEmailRequired
	}
	if !providerEmailVerified {
		return nil, ErrOIDCEmailUnverified
	}
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	if u.Status == model.UserStatusSuspended {
		return nil, ErrUserSuspended
	}
	if _, err := s.getOIDCProvider(ctx, providerKey); err != nil {
		return nil, err
	}
	identity := &model.UserOIDCIdentity{
		UserID:                userID,
		ProviderKey:           providerKey,
		Subject:               subject,
		ProviderEmail:         providerEmail,
		ProviderEmailVerified: providerEmailVerified,
	}
	if err := s.users.LinkOIDCIdentityToUser(ctx, identity); err != nil {
		if errors.Is(err, repository.ErrOIDCIdentityConflict) {
			return nil, ErrOIDCEmailConflict
		}
		return nil, err
	}
	return s.users.FindOIDCIdentity(ctx, providerKey, subject)
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
	return s.oidcStartImplForProvider(ctx, defaultOIDCProviderKey, oidc, redirectAfter, linkUserID)
}

func (s *Service) oidcStartImplForProvider(ctx context.Context, providerKey string, oidc config.OIDC, redirectAfter string, linkUserID int64) (string, error) {
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
		providerKey:   strings.TrimSpace(providerKey),
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

// OIDCStartForProvider starts a login flow for a configured provider key.
// The existing OIDCStart method remains as the default-provider wrapper
// used by current handlers.
func (s *Service) OIDCStartForProvider(ctx context.Context, providerKey, redirectAfter string) (string, error) {
	p, err := s.getOIDCProvider(ctx, providerKey)
	if err != nil {
		return "", err
	}
	if err := s.ensureOIDCProvider(ctx, *p); err != nil {
		return "", err
	}
	return s.oidcStartImplForProvider(ctx, p.ProviderKey, oidcFromProvider(*p), redirectAfter, 0)
}

// OIDCLinkStartForProvider starts an OIDC authorization flow for an
// already-signed-in user and the selected provider.
func (s *Service) OIDCLinkStartForProvider(ctx context.Context, userID int64, providerKey, redirectAfter string) (string, error) {
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", ErrUserNotFound
	}
	if u.Status == model.UserStatusSuspended {
		return "", ErrUserSuspended
	}
	p, err := s.getOIDCProvider(ctx, providerKey)
	if err != nil {
		return "", err
	}
	if err := s.ensureOIDCProvider(ctx, *p); err != nil {
		return "", err
	}
	return s.oidcStartImplForProvider(ctx, p.ProviderKey, oidcFromProvider(*p), redirectAfter, userID)
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
	providerKey := st.providerKey
	if providerKey == "" {
		providerKey = defaultOIDCProviderKey
	}
	if p, err := s.getOIDCProvider(ctx, providerKey); err == nil {
		oidc = oidcFromProvider(*p)
	} else if providerKey != defaultOIDCProviderKey {
		return nil, err
	}
	provider := providerFromOIDC(providerKey, oidc)
	if err := s.ensureOIDCProvider(ctx, provider); err != nil {
		return nil, err
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
		return s.linkOIDCToUser(ctx, st.linkUserID, providerKey, sub, email, emailVerifiedClaim, st.redirectAfter)
	}
	return s.upsertOIDCUser(ctx, providerKey, sub, email, emailVerifiedClaim, st.redirectAfter)
}

// upsertOIDCUser resolves verified IDP claims to a User row or a
// pending completion decision. Returning linked identities log in
// immediately; unlinked subjects never create passwordless users.
func (s *Service) upsertOIDCUser(ctx context.Context, args ...any) (*OIDCLoginResult, error) {
	providerKey, sub, email, emailVerified, redirectAfter, err := parseOIDCUserArgs(args...)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrOIDCEmailRequired
	}
	if !emailVerified {
		return nil, ErrOIDCEmailUnverified
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, ErrDomainNotAllowed
	}
	provider, err := s.getOIDCProvider(ctx, providerKey)
	if err != nil {
		return nil, err
	}
	if err := s.ensureOIDCProvider(ctx, *provider); err != nil {
		return nil, err
	}

	identity, err := s.users.FindOIDCIdentity(ctx, providerKey, sub)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		u, err := s.users.Get(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, ErrUserNotFound
		}
		if u.Status == model.UserStatusSuspended {
			return nil, ErrUserSuspended
		}
		if err := s.users.LinkOIDCIdentityToUser(ctx, &model.UserOIDCIdentity{
			UserID:                identity.UserID,
			ProviderKey:           providerKey,
			Subject:               sub,
			ProviderEmail:         email,
			ProviderEmailVerified: emailVerified,
		}); err != nil {
			return nil, err
		}
		u, err = s.users.Get(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		return &OIDCLoginResult{User: u, RedirectAfter: redirectAfter}, nil
	}

	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return s.makeOIDCPendingForUser(ctx, provider, existing, sub, email, emailVerified, redirectAfter)
}

func parseOIDCUserArgs(args ...any) (providerKey, sub, email string, emailVerified bool, redirectAfter string, err error) {
	providerKey = defaultOIDCProviderKey
	switch len(args) {
	case 4:
		var ok bool
		sub, ok = args[0].(string)
		if !ok {
			err = fmt.Errorf("oidc: sub must be string")
			return
		}
		email, ok = args[1].(string)
		if !ok {
			err = fmt.Errorf("oidc: email must be string")
			return
		}
		emailVerified, ok = args[2].(bool)
		if !ok {
			err = fmt.Errorf("oidc: email_verified must be bool")
			return
		}
		redirectAfter, ok = args[3].(string)
		if !ok {
			err = fmt.Errorf("oidc: redirect_after must be string")
			return
		}
	case 5:
		var ok bool
		providerKey, ok = args[0].(string)
		if !ok {
			err = fmt.Errorf("oidc: provider_key must be string")
			return
		}
		sub, ok = args[1].(string)
		if !ok {
			err = fmt.Errorf("oidc: sub must be string")
			return
		}
		email, ok = args[2].(string)
		if !ok {
			err = fmt.Errorf("oidc: email must be string")
			return
		}
		emailVerified, ok = args[3].(bool)
		if !ok {
			err = fmt.Errorf("oidc: email_verified must be bool")
			return
		}
		redirectAfter, ok = args[4].(string)
		if !ok {
			err = fmt.Errorf("oidc: redirect_after must be string")
			return
		}
	default:
		err = fmt.Errorf("oidc: expected 4 or 5 args, got %d", len(args))
	}
	providerKey = strings.TrimSpace(providerKey)
	if providerKey == "" {
		providerKey = defaultOIDCProviderKey
	}
	return
}

func (s *Service) makeOIDCPendingForUser(ctx context.Context, provider *model.OIDCProvider, existing *model.User, sub, email string, emailVerified bool, redirectAfter string) (*OIDCLoginResult, error) {
	existingUserID := int64(0)
	existingHasOIDC := false
	if existing != nil {
		existingUserID = existing.ID
		linked, err := s.users.ListOIDCIdentities(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		existingHasOIDC = len(linked) > 0
	}
	token, err := randomURLString(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	s.oidcPending.put(token, &oidcPending{
		providerKey:         provider.ProviderKey,
		providerDisplayName: provider.DisplayName,
		providerIconURL:     provider.IconURL,
		sub:                 sub,
		email:               email,
		emailVerified:       emailVerified,
		redirectAfter:       redirectAfter,
		existingUserID:      existingUserID,
		existingHasOIDC:     existingHasOIDC,
		expiresAt:           expiresAt,
	})
	return &OIDCLoginResult{
		Pending: &OIDCPendingDecision{
			Token:               token,
			ProviderKey:         provider.ProviderKey,
			ProviderDisplayName: provider.DisplayName,
			ProviderIconURL:     provider.IconURL,
			Email:               email,
			EmailVerified:       emailVerified,
			ExistingUserID:      existingUserID,
			ExistingUser:        existingUserID > 0,
			ExistingHasOIDC:     existingHasOIDC,
			ExpiresAt:           expiresAt,
		},
		RedirectAfter: redirectAfter,
	}, nil
}

func (s *Service) linkOIDCToUser(ctx context.Context, userID int64, args ...any) (*OIDCLoginResult, error) {
	providerKey, sub, email, emailVerified, redirectAfter, err := parseOIDCUserArgs(args...)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrOIDCEmailRequired
	}
	if !emailVerified {
		return nil, ErrOIDCEmailUnverified
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, ErrDomainNotAllowed
	}

	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	if u.Status == model.UserStatusSuspended {
		return nil, ErrUserSuspended
	}
	if u.Email == nil || *u.Email == "" {
		existing, err := s.users.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != userID {
			return nil, ErrEmailTaken
		}
		if err := s.users.Update(ctx, userID, map[string]any{
			"email":          email,
			"email_verified": emailVerified,
		}); err != nil {
			return nil, err
		}
	}
	if _, err := s.LinkOIDCIdentityToUser(ctx, userID, providerKey, sub, email, emailVerified); err != nil {
		return nil, err
	}
	got, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &OIDCLoginResult{User: got, RedirectAfter: redirectAfter}, nil
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
