// Integration tests for OIDC user upsert — exercises the
// auto-link / refuse / re-find paths added on top of the original
// "create new row by oidc_subject" behavior. Gated on
// INTEGRATION_DB_URL (same pattern as messages/notify integration
// suites).
package user

import (
	"context"
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

// Fresh sub + no existing user → create new row.
func TestUpsertOIDC_BrandNew(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	got, err := svc.upsertOIDCUser(ctx, "sub-new-1", "alice@example.com", true)
	if err != nil {
		t.Fatalf("upsert: %v", err)
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
	first, err := svc.upsertOIDCUser(ctx, "sub-rot", "old@example.com", true)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.upsertOIDCUser(ctx, "sub-rot", "new@example.com", true)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("expected same row id, got %d vs %d", first.ID, second.ID)
	}
	if second.Email == nil || *second.Email != "new@example.com" {
		t.Errorf("email not refreshed: %v", second.Email)
	}
}

// Email/password user already exists, IDP asserts email_verified=true,
// no existing sub on that row → auto-link.
func TestUpsertOIDC_AutoLinkWhenVerified(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	seeded := seedEmailUser(t, db, "alice@example.com", "")

	got, err := svc.upsertOIDCUser(ctx, "sub-link", "alice@example.com", true)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.ID != seeded.ID {
		t.Errorf("expected linked to seeded user (id=%d), got %d", seeded.ID, got.ID)
	}
	if got.OIDCSubject == nil || *got.OIDCSubject != "sub-link" {
		t.Errorf("oidc_subject not linked, got %v", got.OIDCSubject)
	}
}

// Email matches an existing row but IDP did NOT assert
// email_verified → refuse with ErrOIDCEmailConflict.
func TestUpsertOIDC_RefuseWhenEmailUnverified(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	_ = seedEmailUser(t, db, "alice@example.com", "")

	_, err := svc.upsertOIDCUser(ctx, "sub-evil", "alice@example.com", false)
	if err == nil {
		t.Fatal("expected ErrOIDCEmailConflict, got nil")
	}
	if err != ErrOIDCEmailConflict {
		t.Errorf("want ErrOIDCEmailConflict, got %v", err)
	}
}

// Email matches an existing row that already has a DIFFERENT
// oidc_subject linked → refuse even when email_verified=true (two
// IDP identities claiming the same address, operator must resolve).
func TestUpsertOIDC_RefuseWhenExistingHasDifferentSub(t *testing.T) {
	db, svc := setupOIDCDB(t)
	ctx := context.Background()
	_ = seedEmailUser(t, db, "alice@example.com", "sub-original")

	_, err := svc.upsertOIDCUser(ctx, "sub-other", "alice@example.com", true)
	if err == nil {
		t.Fatal("expected ErrOIDCEmailConflict, got nil")
	}
	if err != ErrOIDCEmailConflict {
		t.Errorf("want ErrOIDCEmailConflict, got %v", err)
	}
}

// No email in claims → fall through to path 3 (brand-new account
// with email=nil). Useful for IDPs that don't return email scope.
func TestUpsertOIDC_NoEmailCreatesEmailLessUser(t *testing.T) {
	_, svc := setupOIDCDB(t)
	ctx := context.Background()
	got, err := svc.upsertOIDCUser(ctx, "sub-noemail", "", false)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.Email != nil {
		t.Errorf("expected nil email, got %v", got.Email)
	}
	if got.OIDCSubject == nil || *got.OIDCSubject != "sub-noemail" {
		t.Errorf("oidc_subject missing, got %v", got.OIDCSubject)
	}
}
