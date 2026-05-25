// Package user is the service layer for portal accounts: register,
// log in, change password, bind email, admin moderation, and OIDC
// identity binding.
package user

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// Errors callers branch on.
var (
	ErrEmailTaken           = errors.New("user: email already taken")
	ErrInvalidCredentials   = errors.New("user: invalid credentials")
	ErrUserSuspended        = errors.New("user: account suspended")
	ErrUserNotFound         = errors.New("user: not found")
	ErrInvalidEmail         = errors.New("user: invalid email")
	ErrPasswordTooShort     = errors.New("user: password too short (min 8)")
	ErrRegistrationOff      = errors.New("user: public registration disabled")
	ErrDomainNotAllowed     = errors.New("user: email domain not allowed")
	ErrNotImplemented       = errors.New("user: not implemented in this build")
	ErrOIDCEmailRequired    = errors.New("oidc: email claim is required")
	ErrOIDCEmailMismatch    = errors.New("oidc: email does not match current account")
	ErrOIDCEmailUnverified  = errors.New("oidc: verified email claim is required")
	ErrOIDCPendingInvalid   = errors.New("oidc: pending decision invalid or expired")
	ErrOIDCActionInvalid    = errors.New("oidc: invalid account decision")
	ErrOIDCProviderRequired = errors.New("oidc: provider key is required")
	ErrOIDCProviderNotFound = errors.New("oidc: provider not found")
)

// Service owns registration + auth.
type Service struct {
	users    *repository.UserRepo
	settings *repository.SettingRepo
	bus      *event.Bus
	cfg      *config.Config
	log      *slog.Logger

	// OIDC state — only used when cfg.OIDC.Enabled().
	oidcSessions     *oidcSessions
	oidcPending      *oidcPendingSessions
	oidcHTTP         *http.Client
	oidcDiscoMu      sync.Mutex
	oidcDiscoCache   *oidcDiscovery
	oidcDiscoFetched time.Time
	oidcJWKSMu       sync.Mutex
	oidcJWKSCache    map[string]*rsa.PublicKey
	oidcJWKSURL      string
	oidcJWKSFetched  time.Time
}

// New constructs the service.
func New(users *repository.UserRepo, settings *repository.SettingRepo, bus *event.Bus, cfg *config.Config, lg *slog.Logger) *Service {
	return &Service{
		users:        users,
		settings:     settings,
		bus:          bus,
		cfg:          cfg,
		log:          lg.With(slog.String("component", "service.user")),
		oidcSessions: newOIDCSessions(),
		oidcPending:  newOIDCPendingSessions(),
		oidcHTTP:     &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterInput is what the user portal POSTs.
//
// Code is the 6-digit email verification code received via SendCode. It is
// required when SMTP is configured (production); ignored in dev where the
// operator typically runs without SMTP and codes show up in stderr.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
	Verified bool   `json:"verified"`
}

// Register creates a new account. Gated by:
//   - public registration setting (settings.public_registration_enabled,
//     defaulting to cfg.PublicRegistration when no row exists)
//   - email domain allowlist (same fallback chain)
//
// Emits user.registered on success.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, ErrInvalidEmail
	}
	if len(in.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	allowed, err := s.publicRegistrationEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrRegistrationOff
	}
	if !s.emailDomainAllowed(ctx, in.Email) {
		return nil, ErrDomainNotAllowed
	}

	existing, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)
	subID, err := generateSubID()
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Email:         &in.Email,
		PasswordHash:  hashStr,
		EmailVerified: in.Verified,
		Status:        model.UserStatusActive,
		SubID:         subID,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	if err := s.applyNewUserInitialBalance(ctx, u, "new user initial balance"); err != nil {
		return nil, err
	}
	s.bus.PublishType(event.UserRegistered, RegisteredPayload{UserID: u.ID, Email: in.Email})
	return u, nil
}

// LoginInput is what the user portal POSTs.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginMethods describes the sign-in credentials visible on the
// user's profile page.
type LoginMethods struct {
	Email LoginMethodEmail   `json:"email"`
	OIDC  LoginMethodOIDC    `json:"oidc"`
	OIDCs []OIDCProviderView `json:"oidc_providers,omitempty"`
}

type LoginMethodEmail struct {
	Bound    bool   `json:"bound"`
	Email    string `json:"email,omitempty"`
	Verified bool   `json:"verified"`
}

type LoginMethodOIDC struct {
	Enabled bool   `json:"enabled"`
	Bound   bool   `json:"bound"`
	Name    string `json:"name,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

type OIDCProviderView struct {
	ProviderKey   string `json:"provider_key"`
	DisplayName   string `json:"display_name"`
	IconURL       string `json:"icon_url,omitempty"`
	StartURL      string `json:"start_url,omitempty"`
	Linked        bool   `json:"linked,omitempty"`
	ProviderEmail string `json:"provider_email,omitempty"`
}

type UpdateProfileInput struct {
	DisplayName *string `json:"display_name,omitempty"`
}

type OIDCBindExistingInput struct {
	PendingToken string `json:"pending_token"`
	Password     string `json:"password"`
}

type OIDCCreateAccountInput struct {
	PendingToken string `json:"pending_token"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

// UpdateProfile updates profile metadata that does not participate in
// login identity or uniqueness checks.
func (s *Service) UpdateProfile(ctx context.Context, userID int64, in UpdateProfileInput) (*model.User, error) {
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	updates := map[string]any{}
	if in.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*in.DisplayName)
	}
	if len(updates) > 0 {
		if err := s.users.Update(ctx, userID, updates); err != nil {
			return nil, err
		}
	}
	return s.users.Get(ctx, userID)
}

// Login verifies the credentials and returns the user row. Caller
// (handler) issues the JWT.
func (s *Service) Login(ctx context.Context, in LoginInput) (*model.User, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	u, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if u == nil || u.PasswordHash == "" || u.PasswordHash == model.DisabledPasswordHash {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if u.Status == model.UserStatusSuspended {
		return nil, ErrUserSuspended
	}
	return u, nil
}

// ChangePassword updates a user's password. Every P5 account has a
// local credential, so oldPassword must verify.
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrUserNotFound
	}
	if u.PasswordHash == "" || u.PasswordHash == model.DisabledPasswordHash {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.users.Update(ctx, userID, map[string]any{"password_hash": string(hash)})
}

// BindEmail attaches an email to an existing (OIDC-only) account.
// Domain allowlist is checked. SMTP delivery / verification is not
// implemented in v1 — the email is stored with email_verified=false
// and a follow-up will add the verification flow.
func (s *Service) BindEmail(ctx context.Context, userID int64, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	if !s.emailDomainAllowed(ctx, email) {
		return ErrDomainNotAllowed
	}
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != userID {
		return ErrEmailTaken
	}
	return s.users.Update(ctx, userID, map[string]any{
		"email":          email,
		"email_verified": false,
	})
}

// ChangeEmailVerified updates the local login email after the caller
// has already verified ownership via the email-verification service.
func (s *Service) ChangeEmailVerified(ctx context.Context, userID int64, email string) (*model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, ErrDomainNotAllowed
	}
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != userID {
		return nil, ErrEmailTaken
	}
	if err := s.users.Update(ctx, userID, map[string]any{
		"email":          email,
		"email_verified": true,
	}); err != nil {
		return nil, err
	}
	return s.users.Get(ctx, userID)
}

// LoginMethods returns the current user's visible login credentials.
func (s *Service) LoginMethods(ctx context.Context, userID int64) (*LoginMethods, error) {
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	out := &LoginMethods{
		Email: LoginMethodEmail{
			Bound:    u.Email != nil && *u.Email != "",
			Verified: u.EmailVerified,
		},
	}
	if u.Email != nil {
		out.Email.Email = *u.Email
	}
	providers, err := s.ListOIDCProviders(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.OIDCs = providers
	if len(providers) > 0 {
		out.OIDC.Enabled = true
		out.OIDC.Name = providers[0].DisplayName
		out.OIDC.Icon = providers[0].IconURL
		for _, p := range providers {
			if p.Linked {
				out.OIDC.Bound = true
				break
			}
		}
	}
	return out, nil
}

// NOTE on email_verified: with SMTP disabled (v1) we can't actually
// verify, so we leave email_verified=false rather than claiming
// verified-without-checking. When SMTP support lands, this flow will
// trigger a token email and only set verified=true after the link is
// clicked.

// RotateSubID gives the user a fresh sub_id and invalidates the old
// one. Used when a sub URL has been shared / leaked: the old
// /sub/:oldID immediately starts returning 404 because the assembler
// looks up by sub_id and no row matches.
//
// Side effects:
//   - Any cached subscription on the user's existing devices breaks
//     until they re-import the new URL. That's intentional — the
//     whole point is to revoke the old surface.
//   - Existing ClientOwnership rows are NOT touched; the panel-side
//     clients use email (subID-derived) but the email column on
//     those rows is the OLD sub_id and stays valid. So WG peers,
//     Hysteria auth, etc. on the node keep working — only the
//     /sub URL changes.
//   - Returns the new sub_id so the handler can echo it back.
func (s *Service) RotateSubID(ctx context.Context, userID int64) (string, error) {
	newID, err := generateSubID()
	if err != nil {
		return "", err
	}
	if err := s.users.Update(ctx, userID, map[string]any{
		"sub_id": newID,
	}); err != nil {
		return "", fmt.Errorf("RotateSubID: %w", err)
	}
	s.log.Info("rotated sub_id",
		slog.Int64("user_id", userID),
	)
	return newID, nil
}

// ---- OIDC stubs ----------------------------------------------------------

// OIDCStart generates a state + PKCE verifier, stashes them in the
// in-memory session store, and returns the IDP's authorize URL.
// Callers (handler) hand the URL to the frontend which navigates
// the user there.
func (s *Service) OIDCStart(ctx context.Context, redirectAfter string) (string, error) {
	oidc, err := s.effectiveOIDC(ctx)
	if err != nil {
		return "", err
	}
	if !oidc.Enabled() {
		return "", ErrNotImplemented
	}
	return s.oidcStartImpl(ctx, oidc, redirectAfter, 0)
}

// OIDCLinkStart starts an OIDC authorization flow for a user who is
// already signed in. The callback links the verified provider identity
// to that same user instead of creating or resolving another account.
func (s *Service) OIDCLinkStart(ctx context.Context, userID int64, redirectAfter string) (string, error) {
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
	oidc, err := s.effectiveOIDC(ctx)
	if err != nil {
		return "", err
	}
	if !oidc.Enabled() {
		return "", ErrNotImplemented
	}
	return s.oidcStartImpl(ctx, oidc, redirectAfter, userID)
}

// OIDCCallback exchanges the IDP-returned code for tokens, verifies
// the id_token against the JWKS, and resolves the login by email.
// If the email already belongs to an account that is not linked to
// this OIDC subject yet, the caller gets a pending decision instead
// of a silent auto-link.
func (s *Service) OIDCCallback(ctx context.Context, code, state string) (*OIDCLoginResult, error) {
	oidc, err := s.effectiveOIDC(ctx)
	if err != nil {
		return nil, err
	}
	if !oidc.Enabled() {
		return nil, ErrNotImplemented
	}
	return s.oidcCallbackImpl(ctx, oidc, code, state)
}

// OIDCResolve finalizes a pending OIDC account decision returned by
// OIDCCallback. action is one of:
//   - bind: link this OIDC subject to the existing email account.
//   - recreate: reset that email account's login identity to OIDC
//     (password removed, subscription id rotated) while keeping the
//     same canonical email row.
func (s *Service) OIDCResolve(ctx context.Context, pendingToken, action string) (*model.User, string, error) {
	oidc, err := s.effectiveOIDC(ctx)
	if err != nil {
		return nil, "", err
	}
	if !oidc.Enabled() {
		return nil, "", ErrNotImplemented
	}
	return s.resolveOIDCPending(ctx, pendingToken, action)
}

// OIDCBindExisting links a pending verified provider identity to an
// existing local account after the local password is checked.
func (s *Service) OIDCBindExisting(ctx context.Context, in OIDCBindExistingInput) (*model.User, string, error) {
	if _, err := s.effectiveOIDC(ctx); err != nil {
		return nil, "", err
	}
	pendingToken := strings.TrimSpace(in.PendingToken)
	p := s.oidcPending.get(pendingToken)
	if p == nil || p.existingUserID <= 0 {
		return nil, "", ErrOIDCPendingInvalid
	}
	existing, err := s.users.Get(ctx, p.existingUserID)
	if err != nil {
		return nil, "", err
	}
	if existing == nil || existing.Email == nil || !strings.EqualFold(*existing.Email, p.email) {
		return nil, "", ErrOIDCPendingInvalid
	}
	if existing.Status == model.UserStatusSuspended {
		return nil, "", ErrUserSuspended
	}
	if existing.PasswordHash == "" || existing.PasswordHash == model.DisabledPasswordHash {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(in.Password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	updates := map[string]any{}
	if p.emailVerified && !existing.EmailVerified {
		updates["email_verified"] = true
	}
	if len(updates) > 0 {
		if err := s.users.Update(ctx, existing.ID, updates); err != nil {
			return nil, "", err
		}
	}
	if _, err := s.LinkOIDCIdentityToUser(ctx, existing.ID, p.providerKey, p.sub, p.email, p.emailVerified); err != nil {
		return nil, "", err
	}
	s.oidcPending.delete(pendingToken)
	u, err := s.users.Get(ctx, existing.ID)
	if err != nil {
		return nil, "", err
	}
	return u, p.redirectAfter, nil
}

// OIDCCreateAccount creates a local email/password account from a
// pending OIDC callback. The local email may differ from the provider
// email; the handler verifies local email ownership before calling.
func (s *Service) OIDCCreateAccount(ctx context.Context, in OIDCCreateAccountInput) (*model.User, string, error) {
	if _, err := s.effectiveOIDC(ctx); err != nil {
		return nil, "", err
	}
	pendingToken := strings.TrimSpace(in.PendingToken)
	p := s.oidcPending.get(pendingToken)
	if p == nil {
		return nil, "", ErrOIDCPendingInvalid
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, "", ErrInvalidEmail
	}
	if !s.emailDomainAllowed(ctx, email) {
		return nil, "", ErrDomainNotAllowed
	}
	if len(in.Password) < 8 {
		return nil, "", ErrPasswordTooShort
	}
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if existing != nil {
		return nil, "", ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}
	subID, err := generateSubID()
	if err != nil {
		return nil, "", err
	}
	u := &model.User{
		Email:         &email,
		PasswordHash:  string(hash),
		DisplayName:   strings.TrimSpace(in.DisplayName),
		EmailVerified: true,
		Status:        model.UserStatusActive,
		SubID:         subID,
	}
	if err := s.users.InTx(ctx, func(tx *repository.UserRepo) error {
		if err := tx.Create(ctx, u); err != nil {
			return err
		}
		return tx.LinkOIDCIdentityToUser(ctx, &model.UserOIDCIdentity{
			UserID:                u.ID,
			ProviderKey:           p.providerKey,
			Subject:               p.sub,
			ProviderEmail:         p.email,
			ProviderEmailVerified: p.emailVerified,
		})
	}); err != nil {
		if errors.Is(err, repository.ErrOIDCIdentityConflict) {
			return nil, "", ErrOIDCEmailConflict
		}
		return nil, "", err
	}
	if err := s.applyNewUserInitialBalance(ctx, u, "oidc account completion initial balance"); err != nil {
		return nil, "", err
	}
	s.bus.PublishType(event.UserRegistered, RegisteredPayload{UserID: u.ID, Email: email})
	s.oidcPending.delete(pendingToken)
	return u, p.redirectAfter, nil
}

// OIDCConfig returns the current effective OIDC config for UI metadata.
// Runtime settings override env values; empty settings fall back to env.
func (s *Service) OIDCConfig(ctx context.Context) (config.OIDC, error) {
	return s.effectiveOIDC(ctx)
}

// ---- Admin ----------------------------------------------------------------

// AdminCreateInput is the body shape for admin-side user creation.
//
// Unlike Register, AdminCreate skips the public-registration toggle,
// the email-domain allowlist, and the email verification code: an
// admin is presumed to be vetting the account out-of-band. The new
// row is created with email_verified=true.
type AdminCreateInput struct {
	Email               string `json:"email"`
	Password            string `json:"password"`
	InitialBalanceCents int64  `json:"initial_balance_cents"`
}

// AdminCreate provisions a new portal user under admin authority.
// Emits user.registered on success so downstream listeners (webhook
// fanout, metrics) treat it the same as a self-service signup.
func (s *Service) AdminCreate(ctx context.Context, in AdminCreateInput) (*model.User, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, ErrInvalidEmail
	}
	if len(in.Password) < 8 {
		return nil, ErrPasswordTooShort
	}
	if in.InitialBalanceCents < 0 {
		return nil, fmt.Errorf("user.AdminCreate: initial_balance_cents must be >= 0")
	}

	existing, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)
	subID, err := generateSubID()
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Email:         &in.Email,
		PasswordHash:  hashStr,
		EmailVerified: true, // admin-vetted, skip email-code flow
		Status:        model.UserStatusActive,
		SubID:         subID,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	// Apply initial balance via AdjustBalance so a balance_logs row
	// is written for the credit. Non-zero only.
	if in.InitialBalanceCents > 0 {
		newBal, err := s.users.AdjustBalance(
			ctx, u.ID, in.InitialBalanceCents,
			model.BalanceReasonAdminAdjust,
			"initial balance (admin create)",
			nil,
		)
		if err != nil {
			// Best-effort rollback: the user row exists but the
			// initial credit failed. Surface the error so the
			// admin can retry; the operator can delete + recreate
			// if the half-state is unwanted.
			return nil, fmt.Errorf("user.AdminCreate: apply initial balance: %w", err)
		}
		u.BalanceCents = newBal
	}

	s.bus.PublishType(event.UserRegistered, RegisteredPayload{UserID: u.ID, Email: in.Email})
	return u, nil
}

// AdminUpdateInput is the patch shape admins can apply.
type AdminUpdateInput struct {
	Email         *string `json:"email,omitempty"`
	Password      *string `json:"password,omitempty"`
	EmailVerified *bool   `json:"email_verified,omitempty"`
	Status        *string `json:"status,omitempty"` // active | suspended
	BalanceCents  *int64  `json:"balance_cents,omitempty"`
	AutoRenew     *bool   `json:"auto_renew,omitempty"`
}

// AdminUpdate applies the patch and returns the new user row.
func (s *Service) AdminUpdate(ctx context.Context, userID int64, in AdminUpdateInput) (*model.User, error) {
	updates := map[string]any{}
	if in.Email != nil {
		e := strings.TrimSpace(strings.ToLower(*in.Email))
		if _, err := mail.ParseAddress(e); err != nil {
			return nil, ErrInvalidEmail
		}
		existing, err := s.users.GetByEmail(ctx, e)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != userID {
			return nil, ErrEmailTaken
		}
		updates["email"] = e
	}
	if in.Password != nil {
		if len(*in.Password) < 8 {
			return nil, ErrPasswordTooShort
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		updates["password_hash"] = string(hash)
	}
	if in.EmailVerified != nil {
		updates["email_verified"] = *in.EmailVerified
	}
	if in.Status != nil {
		switch *in.Status {
		case model.UserStatusActive, model.UserStatusSuspended:
			updates["status"] = *in.Status
		default:
			return nil, fmt.Errorf("user.AdminUpdate: unknown status %q", *in.Status)
		}
	}
	if in.BalanceCents != nil {
		if *in.BalanceCents < 0 {
			return nil, fmt.Errorf("user.AdminUpdate: balance_cents must be >= 0")
		}
		updates["balance_cents"] = *in.BalanceCents
	}
	if in.AutoRenew != nil {
		updates["auto_renew"] = *in.AutoRenew
	}
	if len(updates) == 0 {
		return s.users.Get(ctx, userID)
	}
	if err := s.users.Update(ctx, userID, updates); err != nil {
		return nil, err
	}
	return s.users.Get(ctx, userID)
}

// AdminList returns paged user rows for the admin console.
func (s *Service) AdminList(ctx context.Context, limit, offset int) ([]model.User, error) {
	return s.users.List(ctx, limit, offset)
}

// AdminDelete removes a user. ON DELETE CASCADE on the FK takes
// care of ownerships and balance logs.
func (s *Service) AdminDelete(ctx context.Context, userID int64) error {
	return s.users.Delete(ctx, userID)
}

// ---- Helpers --------------------------------------------------------------

// publicRegistrationEnabled reads the settings table first, falling
// back to the env-driven cfg.PublicRegistration when no row exists.
func (s *Service) publicRegistrationEnabled(ctx context.Context) (bool, error) {
	if s.settings != nil {
		v, err := s.settings.GetBool(ctx, model.SettingPublicRegistrationEnabled, s.cfg.PublicRegistration)
		if err == nil {
			return v, nil
		}
	}
	return s.cfg.PublicRegistration, nil
}

// EmailVerificationRequired reads the runtime setting first, falling
// back to the supplied default when the operator has not set an
// override. AuthHandler supplies SMTP availability as the default.
func (s *Service) EmailVerificationRequired(ctx context.Context, fallback bool) (bool, error) {
	if s.settings != nil {
		v, err := s.settings.GetBool(ctx, model.SettingEmailVerificationRequired, fallback)
		if err == nil {
			return v, nil
		}
		return fallback, err
	}
	return fallback, nil
}

// emailDomainAllowed checks the settings override first, then the
// env allowlist. Empty allowlist = no domain restriction.
func (s *Service) emailDomainAllowed(ctx context.Context, email string) bool {
	allowList := s.cfg.EmailDomainAllowlist
	if s.settings != nil {
		if csv, err := s.settings.GetString(ctx, model.SettingEmailDomainAllowlist, ""); err == nil && csv != "" {
			parts := strings.Split(csv, ",")
			allowList = nil
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					allowList = append(allowList, p)
				}
			}
		}
	}
	if len(allowList) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range allowList {
		if strings.EqualFold(domain, d) {
			return true
		}
	}
	return false
}

// generateSubID returns a 32-hex random string.
func generateSubID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand for sub_id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) applyNewUserInitialBalance(ctx context.Context, u *model.User, note string) error {
	if s.settings == nil || u == nil {
		return nil
	}
	cents, err := s.settings.GetInt(ctx, model.SettingNewUserInitialBalanceCents, 0)
	if err != nil {
		return err
	}
	if cents <= 0 {
		return nil
	}
	newBal, err := s.users.AdjustBalance(ctx, u.ID, cents, model.BalanceReasonBonus, note, nil)
	if err != nil {
		return fmt.Errorf("apply initial balance: %w", err)
	}
	u.BalanceCents = newBal
	return nil
}

// RegisteredPayload is the event.UserRegistered payload.
type RegisteredPayload struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}
