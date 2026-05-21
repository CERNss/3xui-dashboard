// Tests for the email-verification service. Gated by INTEGRATION_DB_URL
// like the e2e suite — the service depends on a real Postgres for
// FOR UPDATE / partial-index behavior. Mailer is set to disabled mode
// so tests don't try to dial SMTP.
//
// Run with:
//
//	INTEGRATION_DB_URL='postgres://postgres:test@127.0.0.1:5499/dashboard_e2e?sslmode=disable' \
//	  go test ./internal/service/verification/...
package verification

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/messages"
)

const (
	testEmail = "tester@example.com"
)

// setupDB connects to INTEGRATION_DB_URL, nukes + recreates the schema,
// runs migrations. Returns the open db + a fresh Service. Test skips
// (not fails) when the env var is unset.
func setupDB(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping verification DB tests")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.Config{
		DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2, MigrateOnBoot: false},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := repository.Open(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Reset schema for a clean per-test slate.
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

	// Mailer in disabled mode — messages.Service then short-circuits
	// Send to a no-op so tests don't try to dial SMTP.
	m := mailer.New(config.SMTP{}, logger)
	msgs := messages.New(m, repository.NewNotificationLogRepo(db), nil, nil, nil, logger)
	return db, New(db, msgs, logger)
}

// firstActiveRow returns the most recent unconsumed row for an email so
// tests can inspect attempts / consumed_at without re-implementing the
// service's query.
func firstActiveRow(t *testing.T, db *gorm.DB, email string) record {
	t.Helper()
	var r record
	err := db.Where("email = ? AND consumed_at IS NULL", email).
		Order("sent_at DESC").Limit(1).First(&r).Error
	if err != nil {
		t.Fatalf("query active row: %v", err)
	}
	return r
}

// readCode fishes the plaintext code out of a row by brute force —
// only practical because the space is 10^6 and we control which code
// was generated (we don't capture it cleanly through SendCode's
// public surface). Used only by Consume happy-path tests.
//
// Returns "" if no match is found in the iteration budget.
func readCode(codeHash string) string {
	for i := 0; i < 1_000_000; i++ {
		c := fmt.Sprintf("%06d", i)
		if hashCode(c) == codeHash {
			return c
		}
	}
	return ""
}

// ---- SendCode --------------------------------------------------------------

func TestSendCode_FirstCallSucceeds(t *testing.T) {
	db, svc := setupDB(t)
	if err := svc.SendCode(context.Background(), testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	r := firstActiveRow(t, db, testEmail)
	if r.Email != testEmail {
		t.Errorf("email = %q, want %q", r.Email, testEmail)
	}
	if r.Purpose != string(PurposeRegister) {
		t.Errorf("purpose = %q, want %q", r.Purpose, PurposeRegister)
	}
	if r.CodeHash == "" {
		t.Errorf("code_hash should be populated")
	}
	if r.ExpiresAt.Before(time.Now()) {
		t.Errorf("expires_at should be in the future, got %v", r.ExpiresAt)
	}
	if r.ConsumedAt != nil {
		t.Errorf("fresh row should have nil consumed_at")
	}
	if r.Attempts != 0 {
		t.Errorf("fresh row should have attempts=0, got %d", r.Attempts)
	}
}

func TestSendCode_HashedAtRest(t *testing.T) {
	db, svc := setupDB(t)
	if err := svc.SendCode(context.Background(), testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	r := firstActiveRow(t, db, testEmail)

	// Brute the hash → plaintext to confirm the stored value matches the
	// sha256 of a 6-digit code (not the plaintext itself).
	plaintext := readCode(r.CodeHash)
	if plaintext == "" {
		t.Fatalf("could not invert hash — code_hash is not sha256 of a 6-digit code")
	}
	if len(plaintext) != 6 {
		t.Errorf("recovered plaintext should be 6 digits, got %q", plaintext)
	}
	if hashCode(plaintext) != r.CodeHash {
		t.Errorf("hashCode round-trip mismatch")
	}
}

func TestSendCode_RateLimitedWithin60s(t *testing.T) {
	_, svc := setupDB(t)
	ctx := context.Background()
	if err := svc.SendCode(ctx, testEmail, PurposeRegister); err != nil {
		t.Fatalf("first SendCode: %v", err)
	}
	// Second send within cooldown should be rejected.
	err := svc.SendCode(ctx, testEmail, PurposeRegister)
	if err != ErrRateLimited {
		t.Fatalf("second SendCode within 60s: want ErrRateLimited, got %v", err)
	}
}

// ---- Consume ---------------------------------------------------------------

func TestConsume_HappyPath(t *testing.T) {
	db, svc := setupDB(t)
	ctx := context.Background()
	if err := svc.SendCode(ctx, testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	r := firstActiveRow(t, db, testEmail)
	code := readCode(r.CodeHash)
	if code == "" {
		t.Fatalf("could not recover plaintext code")
	}

	if err := svc.Consume(ctx, testEmail, code, PurposeRegister); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	// Row should now be consumed.
	var after record
	if err := db.First(&after, r.ID).Error; err != nil {
		t.Fatalf("reload row: %v", err)
	}
	if after.ConsumedAt == nil {
		t.Errorf("consumed_at should be set after successful Consume")
	}

	// Replay → ErrCodeNotFound (no unconsumed row remains).
	if err := svc.Consume(ctx, testEmail, code, PurposeRegister); err != ErrCodeNotFound {
		t.Errorf("replay consume: want ErrCodeNotFound, got %v", err)
	}
}

func TestConsume_NoActiveCode(t *testing.T) {
	_, svc := setupDB(t)
	err := svc.Consume(context.Background(), testEmail, "000000", PurposeRegister)
	if err != ErrCodeNotFound {
		t.Errorf("want ErrCodeNotFound, got %v", err)
	}
}

func TestConsume_MismatchIncrementsAttempts(t *testing.T) {
	db, svc := setupDB(t)
	ctx := context.Background()
	if err := svc.SendCode(ctx, testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}

	r := firstActiveRow(t, db, testEmail)
	correct := readCode(r.CodeHash)
	if correct == "" {
		t.Fatalf("could not recover plaintext code")
	}
	// Try 3 wrong codes (deliberately not the correct one).
	wrong := "000000"
	if wrong == correct {
		wrong = "111111"
	}
	for i := 0; i < 3; i++ {
		if err := svc.Consume(ctx, testEmail, wrong, PurposeRegister); err != ErrCodeMismatch {
			t.Fatalf("attempt %d: want ErrCodeMismatch, got %v", i, err)
		}
	}
	after := firstActiveRow(t, db, testEmail)
	if after.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", after.Attempts)
	}
	if after.ConsumedAt != nil {
		t.Errorf("row should not be consumed after mismatches")
	}
}

func TestConsume_BurntRowReturnsTooManyAttempts(t *testing.T) {
	db, svc := setupDB(t)
	ctx := context.Background()
	if err := svc.SendCode(ctx, testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	r := firstActiveRow(t, db, testEmail)
	correct := readCode(r.CodeHash)
	if correct == "" {
		t.Fatalf("could not recover plaintext code")
	}

	// Bump attempts to the max directly (faster than 5 round-trips).
	if err := db.Model(&record{}).Where("id = ?", r.ID).Update("attempts", maxAttempts).Error; err != nil {
		t.Fatalf("bump attempts: %v", err)
	}

	// Even the correct code should fail once the row is burnt.
	if err := svc.Consume(ctx, testEmail, correct, PurposeRegister); err != ErrTooManyAttempts {
		t.Errorf("burnt row consume: want ErrTooManyAttempts, got %v", err)
	}
}

func TestConsume_ExpiredRowReturnsExpired(t *testing.T) {
	db, svc := setupDB(t)
	ctx := context.Background()
	if err := svc.SendCode(ctx, testEmail, PurposeRegister); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	r := firstActiveRow(t, db, testEmail)
	correct := readCode(r.CodeHash)
	if correct == "" {
		t.Fatalf("could not recover plaintext code")
	}

	// Force-expire the row.
	past := time.Now().Add(-time.Hour)
	if err := db.Model(&record{}).Where("id = ?", r.ID).Update("expires_at", past).Error; err != nil {
		t.Fatalf("force-expire: %v", err)
	}

	if err := svc.Consume(ctx, testEmail, correct, PurposeRegister); err != ErrCodeExpired {
		t.Errorf("expired row consume: want ErrCodeExpired, got %v", err)
	}
}

// ---- helpers -------------------------------------------------------------

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"Alice@Example.com":    "alice@example.com",
		"  alice@example.com ": "alice@example.com",
		"Alice@EXAMPLE.COM\n":  "alice@example.com",
	}
	for in, want := range cases {
		if got := normalizeEmail(in); got != want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateCode_SixDigits(t *testing.T) {
	for i := 0; i < 100; i++ {
		c, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode: %v", err)
		}
		if len(c) != 6 {
			t.Errorf("code len = %d, want 6 (%q)", len(c), c)
		}
		for _, r := range c {
			if r < '0' || r > '9' {
				t.Errorf("code contains non-digit %q", r)
			}
		}
	}
}
