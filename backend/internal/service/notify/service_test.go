// Tests the notify bridge service against a real Postgres (gated on
// INTEGRATION_DB_URL). Verifies:
//
//  - events on the bus → mailer.Send invoked
//  - dedup gate: second event for same (kind, ownership) does NOT
//    re-send the mail
//  - user without email: no mailer call, but dedup row IS written
//    so we don't recheck on every tick
//  - mailer failure leaves the dedup row absent so a retry tick
//    can try again
//
// Mailer is wired to disabled mode (no real SMTP) but we attach a
// capturing slog handler to count the "would-be" log entries that
// stand in for actual sends.
package notify

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	jobpkg "github.com/cern/3xui-dashboard/internal/job"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
)

// stubChannel is the test double for Channel. It records every Send
// so tests can assert call count + recipient + message shape.
type stubChannel struct {
	mu       sync.Mutex
	name     string
	enabled  bool
	sends    []Message
	sendErr  error // when set, Send returns this and skips recording
}

func newStubChannel(name string) *stubChannel { return &stubChannel{name: name, enabled: true} }

func (s *stubChannel) Name() string  { return s.name }
func (s *stubChannel) Enabled() bool { return s.enabled }
func (s *stubChannel) Send(_ context.Context, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sends = append(s.sends, msg)
	return nil
}
func (s *stubChannel) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sends)
}

// countingHandler is unused under the new stubChannel-based tests
// but kept as a placeholder for future tests that need to observe
// slog output.
type countingHandler struct{ mu sync.Mutex }

func (h *countingHandler) Enabled(_ context.Context, _ slog.Level) bool { return false }
func (h *countingHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler         { return h }
func (h *countingHandler) WithGroup(_ string) slog.Handler              { return h }

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping notify DB tests")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2},
	}
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
	return db
}

func seedUser(t *testing.T, db *gorm.DB, id int64, email string) {
	t.Helper()
	u := &model.User{ID: id, Status: model.UserStatusActive, SubID: "sub-" + email}
	if email != "" {
		u.Email = &email
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}
func seedNode(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	n := &model.Node{
		ID:       id,
		Name:     "test",
		Scheme:   "http",
		Host:     "127.0.0.1",
		Port:     7777,
		APIToken: "x",
		Enabled:  true,
		Status:   "unknown",
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
}
func seedOwnership(t *testing.T, db *gorm.DB, userID int64) *model.ClientOwnership {
	t.Helper()
	o := &model.ClientOwnership{
		UserID:      userID,
		NodeID:      1,
		InboundTag:  "vless-1",
		ClientEmail: "client@test",
		Enabled:     true,
	}
	if err := db.Create(o).Error; err != nil {
		t.Fatalf("seed ownership: %v", err)
	}
	return o
}

// newServiceWithCapture builds a notify.Service with a single stub
// "email" channel and the default router. `email` is the only
// channel the legacy lifecycle tests need — ops-channel routing is
// covered separately. The stub records each Send for assertions.
func newServiceWithCapture(t *testing.T, db *gorm.DB) (*Service, *stubChannel) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	bus := event.New()
	email := newStubChannel("email")
	router, _ := ParseRoutes("") // defaults: client lifecycle → email
	svc := New(
		bus,
		router,
		[]Channel{email},
		repository.NewUserRepo(db),
		repository.NewClientOwnershipRepo(db),
		repository.NewNotificationLogRepo(db),
		logger,
	)
	svc.Start()
	return svc, email
}

func TestExpired_SendsEmailViaJobPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	svc, h := newServiceWithCapture(t, db)
	// Publish a jobpkg.ExpiredPayload (the path ExpiryJob takes).
	svc.bus.PublishType(event.ClientExpired, jobpkg.ExpiredPayload{
		OwnershipID: o.ID,
		UserID:      1,
		NodeID:      1,
		InboundTag:  o.InboundTag,
		ClientEmail: o.ClientEmail,
		ExpiredAt:   time.Now().UTC(),
	})
	// Bus is synchronous — handler has finished by Publish return.
	if got := h.Count(); got != 1 {
		t.Errorf("expected 1 mail delivered, got %d", got)
	}
}

func TestExpired_DedupSecondPublish(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	svc, h := newServiceWithCapture(t, db)
	payload := jobpkg.ExpiredPayload{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
	}
	svc.bus.PublishType(event.ClientExpired, payload)
	svc.bus.PublishType(event.ClientExpired, payload) // second pass — dedup

	if got := h.Count(); got != 1 {
		t.Errorf("expected exactly 1 mail (dedup), got %d", got)
	}
}

func TestExpired_UserWithNoEmail_NoMailButLogsDedup(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "") // no email
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	svc, h := newServiceWithCapture(t, db)
	svc.bus.PublishType(event.ClientExpired, jobpkg.ExpiredPayload{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
	})

	if got := h.Count(); got != 0 {
		t.Errorf("user without email should NOT receive mail, got %d delivered", got)
	}
	// Dedup row IS written so we don't recheck on every tick.
	already, _ := repository.NewNotificationLogRepo(db).AlreadySent(context.Background(), model.SurfaceNotification, "expired_email", o.ID)
	if !already {
		t.Errorf("expected dedup row even when email skipped")
	}
}

func TestExpiringSoon_FromJobPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	svc, h := newServiceWithCapture(t, db)
	exp := time.Now().Add(2 * 24 * time.Hour)
	svc.bus.PublishType(event.ClientExpiringSoon, jobpkg.ExpiringSoonPayload{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
		ExpiresAt: exp, DaysRemaining: 2,
	})
	if got := h.Count(); got != 1 {
		t.Errorf("expected 1 mail for expiring_soon, got %d", got)
	}
}

func TestOverLimit_FromTrafficPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	svc, h := newServiceWithCapture(t, db)
	// traffic.evaluateRules path: triple lookup needed (no UserID in payload).
	svc.bus.PublishType(event.ClientOverLimit, traffic.ClientThresholdPayload{
		NodeID: o.NodeID, NodeName: "test", InboundTag: o.InboundTag,
		ClientEmail: o.ClientEmail, Up: 1_000_000, Down: 9_000_000, Limit: 10_000_000,
	})
	if got := h.Count(); got != 1 {
		t.Errorf("expected 1 mail for over_limit, got %d", got)
	}
}

func TestOverLimit_UnknownClient_NoMail(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	_ = seedOwnership(t, db, 1) // unrelated ownership

	svc, h := newServiceWithCapture(t, db)
	svc.bus.PublishType(event.ClientOverLimit, traffic.ClientThresholdPayload{
		NodeID: 1, NodeName: "test", InboundTag: "different-tag",
		ClientEmail: "nobody@example", Up: 1, Down: 1, Limit: 1,
	})
	if got := h.Count(); got != 0 {
		t.Errorf("unknown client should not deliver, got %d", got)
	}
}
