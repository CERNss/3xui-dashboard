// Integration tests for OIDC user resolution. The intended identity
// model is email-first: OIDC subject is a login credential on the
// canonical email account, and existing emails require an explicit
// user decision instead of silent auto-linking.
package user

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

func setupOIDCDB(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping oidc upsert DB tests")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := repository.Open(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := repository.MigrateUp(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close(db) })

	userRepo := repository.NewUserRepo(db)
	settingRepo := repository.NewSettingRepo(db)
	bus := event.New()
	svc := New(userRepo, settingRepo, bus, &config.Config{}, logger)
	return db, svc
}

func seedEmailUser(t *testing.T, db *gorm.DB, email string, withOIDCSub string) *model.User {
	t.Helper()
	u := &model.User{
		Status: model.UserStatusActive,
		SubID:  "seed-" + email,
		Email:  &email,
	}
	if withOIDCSub != "" {
		u.OIDCSubject = &withOIDCSub
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestEffectiveOIDC_UsesRuntimeSettings(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.OIDC = config.OIDC{
		Issuer:       "https://env.example.com",
		ClientID:     "env-client",
		ClientSecret: "env-secret",
		RedirectURL:  "https://env.example.com/callback",
		Scopes:       []string{"openid", "email"},
		DisplayName:  "Env SSO",
	}
	ctx := context.Background()
	if err := svc.settings.Set(ctx, model.SettingOIDCIssuer, "https://runtime.example.com"); err != nil {
		t.Fatalf("set issuer: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCClientID, "runtime-client"); err != nil {
		t.Fatalf("set client id: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCScopes, "openid,profile,email"); err != nil {
		t.Fatalf("set scopes: %v", err)
	}
	if err := svc.settings.Set(ctx, model.SettingOIDCDisplayName, ""); err != nil {
		t.Fatalf("set blank display name: %v", err)
	}

	got, err := svc.effectiveOIDC(ctx)
	if err != nil {
		t.Fatalf("effectiveOIDC: %v", err)
	}
	if got.Issuer != "https://runtime.example.com" {
		t.Fatalf("issuer = %q", got.Issuer)
	}
	if got.ClientID != "runtime-client" {
		t.Fatalf("client id = %q", got.ClientID)
	}
	if got.ClientSecret != "env-secret" {
		t.Fatalf("client secret fallback = %q", got.ClientSecret)
	}
	if got.DisplayName != "Env SSO" {
		t.Fatalf("display name fallback = %q", got.DisplayName)
	}
	if len(got.Scopes) != 3 || got.Scopes[1] != "profile" {
		t.Fatalf("scopes = %#v", got.Scopes)
	}
}

// Fresh sub + no existing user → create new row.
func TestUpsertOIDC_BrandNew(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	res, err := svc.upsertOIDCUser(ctx, "sub-new-1", "alice@example.com", true, "/portal")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got := res.User
	if got == nil {
		t.Fatalf("expected resolved user, got pending=%+v", res.Pending)
	}
	if got.OIDCSubject == nil || *got.OIDCSubject != "sub-new-1" {
		t.Errorf("want oidc_subject=sub-new-1, got %v", got.OIDCSubject)
	}
	if got.Email == nil || *got.Email != "alice@example.com" {
		t.Errorf("want email=alice@example.com, got %v", got.Email)
	}
	if !got.EmailVerified {
		t.Errorf("email_verified should be true (IDP asserted)")
	}
}

// Same sub, second login, email rotated → existing row updated in place.
func TestUpsertOIDC_ExistingSubRefreshesEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	firstRes, err := svc.upsertOIDCUser(ctx, "sub-rot", "old@example.com", true, "")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	first := firstRes.User
	secondRes, err := svc.upsertOIDCUser(ctx, "sub-rot", "new@example.com", true, "")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	second := secondRes.User
	if second == nil {
		t.Fatalf("expected resolved user, got pending=%+v", secondRes.Pending)
	}
	if first.ID != second.ID {
		t.Errorf("expected same row id, got %d vs %d", first.ID, second.ID)
	}
	if second.Email == nil || *second.Email != "new@example.com" {
		t.Errorf("email not refreshed: %v", second.Email)
	}
}

func TestUpsertOIDC_ExistingSubRejectsDisallowedRefreshEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.EmailDomainAllowlist = []string{"example.com"}
	ctx := context.Background()
	firstRes, err := svc.upsertOIDCUser(ctx, "sub-rot-deny", "old@example.com", true, "")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	first := firstRes.User

	_, err = svc.upsertOIDCUser(ctx, "sub-rot-deny", "new@blocked.test", true, "")
	if !errors.Is(err, ErrDomainNotAllowed) {
		t.Fatalf("want ErrDomainNotAllowed, got %v", err)
	}

	after, err := svc.users.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("get after denied refresh: %v", err)
	}
	if after.Email == nil || *after.Email != "old@example.com" {
		t.Errorf("disallowed refresh changed email: %v", after.Email)
	}
}

// Email/password user already exists → callback must not auto-link.
// It returns a pending decision and only links after the user chooses bind.
func TestUpsertOIDC_ExistingEmailRequiresDecisionThenBind(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	seeded := seedEmailUser(t, db, "alice@example.com", "")

	res, err := svc.upsertOIDCUser(ctx, "sub-link", "alice@example.com", true, "/portal/subscription")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if res.User != nil {
		t.Fatalf("expected pending decision, got user id=%d", res.User.ID)
	}
	if res.Pending == nil {
		t.Fatal("expected pending decision, got nil")
	}
	if res.Pending.Email != "alice@example.com" || res.Pending.Token == "" {
		t.Fatalf("bad pending response: %+v", res.Pending)
	}

	got, redirectAfter, err := svc.resolveOIDCPending(ctx, res.Pending.Token, "bind")
	if err != nil {
		t.Fatalf("resolve bind: %v", err)
	}
	if redirectAfter != "/portal/subscription" {
		t.Errorf("redirect_after = %q", redirectAfter)
	}
	if got.ID != seeded.ID {
		t.Errorf("expected linked to seeded user (id=%d), got %d", seeded.ID, got.ID)
	}
	if got.OIDCSubject == nil || *got.OIDCSubject != "sub-link" {
		t.Errorf("oidc_subject not linked, got %v", got.OIDCSubject)
	}
}

func TestUpsertOIDC_AutoLinkRejectsDisallowedEmail(t *testing.T) {
	db, svc := setupOIDCDB(t)
	svc.cfg.EmailDomainAllowlist = []string{"example.com"}
	ctx := context.Background()
	_ = seedEmailUser(t, db, "alice@blocked.test", "")

	_, err := svc.upsertOIDCUser(ctx, "sub-link-deny", "alice@blocked.test", true, "")
	if !errors.Is(err, ErrDomainNotAllowed) {
		t.Fatalf("want ErrDomainNotAllowed, got %v", err)
	}
}

// Email matches an existing row but IDP did NOT assert
// email_verified → still require explicit decision; no silent link.
func TestUpsertOIDC_UnverifiedExistingEmailRequiresDecision(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	_ = seedEmailUser(t, db, "alice@example.com", "")

	res, err := svc.upsertOIDCUser(ctx, "sub-evil", "alice@example.com", false, "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if res.Pending == nil {
		t.Fatalf("expected pending decision, got %+v", res)
	}
	if res.Pending.EmailVerified {
		t.Error("pending should record email_verified=false")
	}
}

// Email matches an existing row that already has a DIFFERENT
// oidc_subject linked → bind is refused, recreate can replace the
// OIDC credential on the same canonical email row.
func TestUpsertOIDC_ExistingDifferentSubRequiresRecreate(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	seeded := seedEmailUser(t, db, "alice@example.com", "sub-original")

	res, err := svc.upsertOIDCUser(ctx, "sub-other", "alice@example.com", true, "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if res.Pending == nil || !res.Pending.ExistingHasOIDC {
		t.Fatalf("expected pending existing_has_oidc, got %+v", res.Pending)
	}
	_, _, err = svc.resolveOIDCPending(ctx, res.Pending.Token, "bind")
	if !errors.Is(err, ErrOIDCEmailConflict) {
		t.Fatalf("bind should conflict when another OIDC subject is linked, got %v", err)
	}

	res, err = svc.upsertOIDCUser(ctx, "sub-other", "alice@example.com", true, "")
	if err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	got, _, err := svc.resolveOIDCPending(ctx, res.Pending.Token, "recreate")
	if err != nil {
		t.Fatalf("resolve recreate: %v", err)
	}
	if got.ID != seeded.ID {
		t.Errorf("expected same canonical email row id=%d, got %d", seeded.ID, got.ID)
	}
	if got.OIDCSubject == nil || *got.OIDCSubject != "sub-other" {
		t.Errorf("oidc_subject not replaced, got %v", got.OIDCSubject)
	}
	if got.PasswordHash != nil {
		t.Error("recreate should remove password credential")
	}
}

func TestUpsertOIDC_BrandNewRejectsDisallowedEmail(t *testing.T) {
	_, svc := setupOIDCDB(t)
	svc.cfg.EmailDomainAllowlist = []string{"example.com"}
	ctx := context.Background()

	_, err := svc.upsertOIDCUser(ctx, "sub-new-deny", "alice@blocked.test", true, "")
	if !errors.Is(err, ErrDomainNotAllowed) {
		t.Fatalf("want ErrDomainNotAllowed, got %v", err)
	}
}

// No email in claims → reject. Email is the canonical user identity.
func TestUpsertOIDC_NoEmailRejected(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	_, err := svc.upsertOIDCUser(ctx, "sub-noemail", "", false, "")
	if !errors.Is(err, ErrOIDCEmailRequired) {
		t.Fatalf("want ErrOIDCEmailRequired, got %v", err)
	}
}
