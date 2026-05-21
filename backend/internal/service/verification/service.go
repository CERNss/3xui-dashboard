// Package verification issues and validates short-lived email codes.
//
// Code shape: 6 decimal digits, 10 minute TTL, single-use, scoped by
// purpose ("register" today; password reset / email change later).
//
// Storage: email_verification_codes table (migration 0004). Code is
// hashed at rest so a DB leak doesn't immediately compromise pending
// verifications.
//
// Rate limit: 60 seconds between sends for the same (email, purpose).
package verification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/service/messages"
)

// Purpose enumerates what a code is good for. Scoped so a register code
// can't be replayed against a future password-reset endpoint.
type Purpose string

const (
	PurposeRegister Purpose = "register"
)

const (
	codeLength      = 6
	codeTTL         = 10 * time.Minute
	resendCooldown  = 60 * time.Second
	maxAttempts     = 5 // per code row
)

var (
	ErrRateLimited     = errors.New("verification: send too frequent — wait before retrying")
	ErrCodeNotFound    = errors.New("verification: no active code for this email")
	ErrCodeExpired     = errors.New("verification: code expired — request a new one")
	ErrCodeMismatch    = errors.New("verification: incorrect code")
	ErrTooManyAttempts = errors.New("verification: too many attempts — request a new code")
)

// Record mirrors a row in email_verification_codes. Lives here (not in
// /model) since it's a service-internal type — handlers never see it.
type record struct {
	ID          int64
	Email       string
	Purpose     string
	CodeHash    string `gorm:"column:code_hash"`
	ExpiresAt   time.Time
	SentAt      time.Time
	ConsumedAt  *time.Time
	Attempts    int
}

func (record) TableName() string { return "email_verification_codes" }

// Service is the verification-code engine.
type Service struct {
	db       *gorm.DB
	messages *messages.Service
	logger   *slog.Logger
}

// New constructs a Service. msgs delivers the email via the unified
// user-message surface — when SMTP is disabled the underlying send
// is a no-op and SendCode still records the row, so the operator
// can read the generated code from dashboard logs.
func New(db *gorm.DB, msgs *messages.Service, logger *slog.Logger) *Service {
	return &Service{db: db, messages: msgs, logger: logger}
}

// SendCode generates a new code, stores its hash, and dispatches the
// email. Rate-limited by resendCooldown per (email, purpose). If SMTP
// is disabled, the mailer logs the code so dev can read it from stderr.
func (s *Service) SendCode(ctx context.Context, email string, purpose Purpose) error {
	email = normalizeEmail(email)
	now := time.Now()

	// Cooldown check — has there been a send within the last `resendCooldown`?
	var recent record
	err := s.db.WithContext(ctx).
		Where("email = ? AND purpose = ? AND sent_at > ?",
			email, string(purpose), now.Add(-resendCooldown)).
		Order("sent_at DESC").
		Limit(1).
		First(&recent).Error
	if err == nil {
		return ErrRateLimited
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query recent codes: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return err
	}

	row := record{
		Email:     email,
		Purpose:   string(purpose),
		CodeHash:  hashCode(code),
		ExpiresAt: now.Add(codeTTL),
		SentAt:    now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("store code: %w", err)
	}

	subject := emailSubject(purpose)
	body := emailBody(purpose, code, codeTTL)
	// Transactional one-shot: no dedup (verification self-rate-limits
	// upstream via the resendCooldown check). Empty kind + zero
	// ownership ID tell messages.Send to skip the dedup log.
	if err := s.messages.Send(ctx, email, subject, body, "", 0); err != nil {
		// Don't roll back the row — operator can re-send after cooldown,
		// or read the code from logs (dev) and use it directly.
		s.logger.Warn("verification: mail send failed", "err", err, "email", email)
		return fmt.Errorf("send mail: %w", err)
	}

	return nil
}

// Consume validates a presented code, marks it used, and returns nil
// on success. Idempotent only inasmuch as repeated success is impossible:
// the row's consumed_at flips, so a second consume returns ErrCodeNotFound.
//
// Increments `attempts` on each mismatch; after maxAttempts the code is
// burnt and the caller must request a new one.
//
// Note: the SELECT runs inside a tx so the conditional logic stays
// consistent, but the attempts/consumed_at UPDATE runs on the parent
// connection so it commits even when the function returns
// ErrCodeMismatch. Returning a non-nil error from a gorm.Transaction
// closure rolls back the whole tx, which would silently undo the
// attempts increment and let an attacker brute-force codes
// indefinitely. This split is the simplest fix; the read+write
// race window is fine because both UPDATEs are guarded by
// `WHERE consumed_at IS NULL`.
func (s *Service) Consume(ctx context.Context, email, code string, purpose Purpose) error {
	email = normalizeEmail(email)
	now := time.Now()

	var row record
	err := s.db.WithContext(ctx).
		Where("email = ? AND purpose = ? AND consumed_at IS NULL",
			email, string(purpose)).
		Order("sent_at DESC").
		Limit(1).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCodeNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup code: %w", err)
	}

	if row.ExpiresAt.Before(now) {
		return ErrCodeExpired
	}
	if row.Attempts >= maxAttempts {
		return ErrTooManyAttempts
	}

	if row.CodeHash != hashCode(code) {
		// Bump attempts on a non-tx UPDATE so the increment commits
		// independently of the ErrCodeMismatch return. WHERE
		// consumed_at IS NULL prevents the (rare) race where a
		// parallel success consumed the row between SELECT and
		// UPDATE — we don't want to leak attempts on a successful
		// consume.
		if err := s.db.WithContext(ctx).
			Model(&record{}).
			Where("id = ? AND consumed_at IS NULL", row.ID).
			Update("attempts", row.Attempts+1).Error; err != nil {
			return fmt.Errorf("increment attempts: %w", err)
		}
		return ErrCodeMismatch
	}

	// Success — flip consumed_at. Same WHERE guard ensures two
	// concurrent successful Consume calls don't both think they
	// won; the second's RowsAffected=0 surfaces as ErrCodeNotFound
	// to the caller, matching the "first wins" semantics.
	res := s.db.WithContext(ctx).
		Model(&record{}).
		Where("id = ? AND consumed_at IS NULL", row.ID).
		Update("consumed_at", now)
	if res.Error != nil {
		return fmt.Errorf("mark consumed: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrCodeNotFound
	}
	return nil
}

// ---- helpers ---------------------------------------------------------------

func generateCode() (string, error) {
	// 6 decimal digits — collision space is fine for 10-minute TTL.
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return fmt.Sprintf("%0*d", codeLength, n.Int64()), nil
}

func hashCode(code string) string {
	// SHA-256 is fine here — codes are 6 digits with 10-minute TTL, so
	// the offline-brute-force concern that motivates bcrypt for password
	// hashes doesn't apply (code expires before any meaningful brute).
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(s string) string {
	// Match how user service normalizes — trim + lowercase. Avoids the
	// "Alice@x.com sent code, alice@x.com tries to consume" mismatch.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

func emailSubject(p Purpose) string {
	switch p {
	case PurposeRegister:
		return "【3xui Central】注册验证码"
	default:
		return "【3xui Central】验证码"
	}
}

func emailBody(p Purpose, code string, ttl time.Duration) string {
	intent := "注册账户"
	if p != PurposeRegister {
		intent = "验证邮箱"
	}
	return fmt.Sprintf(
		"您正在 3xui Central %s。\n\n"+
			"验证码：%s\n\n"+
			"有效期 %d 分钟。如果不是您本人操作，请忽略这封邮件。\n",
		intent, code, int(ttl.Minutes()),
	)
}
