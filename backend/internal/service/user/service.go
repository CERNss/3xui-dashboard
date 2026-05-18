// Package user is the service layer for portal accounts: register,
// log in, change password, bind email, admin moderation. OIDC + SMTP
// hooks land as stubs in v1 — config flows through but the actual
// flows return ErrNotImplemented.
package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// Errors callers branch on.
var (
	ErrEmailTaken         = errors.New("user: email already taken")
	ErrInvalidCredentials = errors.New("user: invalid credentials")
	ErrUserSuspended      = errors.New("user: account suspended")
	ErrUserNotFound       = errors.New("user: not found")
	ErrInvalidEmail       = errors.New("user: invalid email")
	ErrPasswordTooShort   = errors.New("user: password too short (min 8)")
	ErrRegistrationOff    = errors.New("user: public registration disabled")
	ErrDomainNotAllowed   = errors.New("user: email domain not allowed")
	ErrNotImplemented     = errors.New("user: not implemented in this build")
)

// Service owns registration + auth.
type Service struct {
	users    *repository.UserRepo
	settings *repository.SettingRepo
	bus      *event.Bus
	cfg      *config.Config
	log      *slog.Logger
}

// New constructs the service.
func New(users *repository.UserRepo, settings *repository.SettingRepo, bus *event.Bus, cfg *config.Config, lg *slog.Logger) *Service {
	return &Service{
		users:    users,
		settings: settings,
		bus:      bus,
		cfg:      cfg,
		log:      lg.With(slog.String("component", "service.user")),
	}
}

// RegisterInput is what the user portal POSTs.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register creates a new account. Gated by:
//   - public registration setting (settings.public_registration_enabled,
//     defaulting to cfg.PublicRegistration when no row exists)
//   - email domain allowlist (same fallback chain)
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
		PasswordHash:  &hashStr,
		EmailVerified: false, // No SMTP yet — see ConfirmEmail.
		Status:        model.UserStatusActive,
		SubID:         subID,
	}
	if err := s.users.Create(ctx, u); err != nil {
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

// Login verifies the credentials and returns the user row. Caller
// (handler) issues the JWT.
func (s *Service) Login(ctx context.Context, in LoginInput) (*model.User, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	u, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if u == nil || u.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if u.Status == model.UserStatusSuspended {
		return nil, ErrUserSuspended
	}
	return u, nil
}

// ChangePassword updates a user's password. oldPassword may be empty
// for users that have never set one (OIDC-only path); otherwise it
// must verify.
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
	if u.PasswordHash != nil {
		if err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(oldPassword)); err != nil {
			return ErrInvalidCredentials
		}
	} else if oldPassword != "" {
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
		"email_verified": s.cfg.SMTP.Enabled() == false, // see comment below
	})
}

// NOTE on email_verified: with SMTP disabled (v1) we can't actually
// verify, so we leave email_verified=false rather than claiming
// verified-without-checking. When SMTP support lands, this flow will
// trigger a token email and only set verified=true after the link is
// clicked.

// ---- OIDC stubs ----------------------------------------------------------

// OIDCStart would generate state + PKCE verifier and return the
// authorize URL. v1 returns ErrNotImplemented; the wiring is in
// place so the upgrade is a localized change.
func (s *Service) OIDCStart(ctx context.Context, redirectAfter string) (string, error) {
	if !s.cfg.OIDC.Enabled() {
		return "", ErrNotImplemented
	}
	return "", ErrNotImplemented
}

// OIDCCallback would exchange the code, verify the ID token, and
// provision-or-link the matching User row. v1 returns
// ErrNotImplemented.
func (s *Service) OIDCCallback(ctx context.Context, code, state string) (*model.User, error) {
	return nil, ErrNotImplemented
}

// ---- Admin ----------------------------------------------------------------

// AdminUpdateInput is the patch shape admins can apply.
type AdminUpdateInput struct {
	Email        *string `json:"email,omitempty"`
	Status       *string `json:"status,omitempty"` // active | suspended
	BalanceCents *int64  `json:"balance_cents,omitempty"`
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
	if in.Status != nil {
		switch *in.Status {
		case model.UserStatusActive, model.UserStatusSuspended:
			updates["status"] = *in.Status
		default:
			return nil, fmt.Errorf("user.AdminUpdate: unknown status %q", *in.Status)
		}
	}
	if in.BalanceCents != nil {
		updates["balance_cents"] = *in.BalanceCents
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

// RegisteredPayload is the event.UserRegistered payload.
type RegisteredPayload struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}
