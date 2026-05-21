// Integration tests for messages.Service exercising the bus-driven
// client-lifecycle path against a real Postgres (gated on
// INTEGRATION_DB_URL). The unit-level tests in service_test.go
// cover dedup mechanics in isolation; these tests verify the
// full event → mail flow.
//
// Run with:
//
//	INTEGRATION_DB_URL='postgres://postgres:demo@127.0.0.1:5495/dashboard_demo?sslmode=disable' \
//	  go test ./internal/service/messages/...
package messages

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
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
)

// countingMailer records every Send call so tests can assert
// "would-be sent" without needing a real SMTP server.
type countingMailer struct {
	mu    sync.Mutex
	sent  []sentMail
	err   error
}

type sentMail struct {
	to, subject, body string
}

func (m *countingMailer) Enabled() bool { return true }
func (m *countingMailer) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, sentMail{to, subject, body})
	return nil
}
func (m *countingMailer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping messages DB tests")
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

// newServiceWithCapture builds a messages.Service wired against the
// real DB + a counting mailer, with bus subscribers active. Use
// this in tests that publish lifecycle events.
func newServiceWithCapture(t *testing.T, db *gorm.DB) (*Service, *countingMailer, *event.Bus) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	bus := event.New()
	m := &countingMailer{}
	svc := New(
		m,
		repository.NewNotificationLogRepo(db),
		bus,
		repository.NewUserRepo(db),
		repository.NewClientOwnershipRepo(db),
		logger,
	)
	svc.Start()
	return svc, m, bus
}

func TestExpired_SendsEmailViaJobPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	_, m, bus := newServiceWithCapture(t, db)
	bus.PublishType(event.ClientExpired, payload.ClientExpired{
		OwnershipID: o.ID,
		UserID:      1,
		NodeID:      1,
		InboundTag:  o.InboundTag,
		ClientEmail: o.ClientEmail,
		ExpiredAt:   time.Now().UTC(),
	})
	// Bus is synchronous — handler has finished by Publish return.
	if got := m.Count(); got != 1 {
		t.Errorf("expected 1 mail delivered, got %d", got)
	}
	// Mail goes to the user's bound email — not the ops mailbox.
	if got := m.sent[0].to; got != "alice@example.com" {
		t.Errorf("expected to=alice@example.com, got %q", got)
	}
}

func TestExpired_DedupSecondPublish(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	_, m, bus := newServiceWithCapture(t, db)
	payload := payload.ClientExpired{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
	}
	bus.PublishType(event.ClientExpired, payload)
	bus.PublishType(event.ClientExpired, payload) // second pass — dedup
	if got := m.Count(); got != 1 {
		t.Errorf("expected exactly 1 mail (dedup), got %d", got)
	}
}

func TestExpired_UserWithNoEmail_NoMail(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "") // no email
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	_, m, bus := newServiceWithCapture(t, db)
	bus.PublishType(event.ClientExpired, payload.ClientExpired{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
	})
	if got := m.Count(); got != 0 {
		t.Errorf("user without email should NOT receive mail, got %d delivered", got)
	}
	// No dedup row either — messages.Service doesn't book a row when
	// it short-circuits on missing recipient (unlike the old notify
	// path which booked an "I skipped this" sentinel). Re-publishing
	// after the user binds an email later WILL send the mail.
	already, _ := repository.NewNotificationLogRepo(db).AlreadySent(
		context.Background(), model.SurfaceMessage, "client_expired", o.ID,
	)
	if already {
		t.Errorf("expected NO dedup row when send was skipped for missing recipient")
	}
}

func TestExpiringSoon_FromJobPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	_, m, bus := newServiceWithCapture(t, db)
	exp := time.Now().Add(2 * 24 * time.Hour)
	bus.PublishType(event.ClientExpiringSoon, payload.ClientExpiringSoon{
		OwnershipID: o.ID, UserID: 1, NodeID: 1,
		InboundTag: o.InboundTag, ClientEmail: o.ClientEmail,
		ExpiresAt: exp, DaysRemaining: 2,
	})
	if got := m.Count(); got != 1 {
		t.Errorf("expected 1 mail for expiring_soon, got %d", got)
	}
}

func TestOverLimit_FromTrafficPayload(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	o := seedOwnership(t, db, 1)

	_, m, bus := newServiceWithCapture(t, db)
	bus.PublishType(event.ClientOverLimit, payload.ClientThreshold{
		NodeID: o.NodeID, NodeName: "test", InboundTag: o.InboundTag,
		ClientEmail: o.ClientEmail, Up: 1_000_000, Down: 9_000_000, Limit: 10_000_000,
	})
	if got := m.Count(); got != 1 {
		t.Errorf("expected 1 mail for over_limit, got %d", got)
	}
}

func TestOverLimit_UnknownClient_NoMail(t *testing.T) {
	db := setupDB(t)
	seedUser(t, db, 1, "alice@example.com")
	seedNode(t, db, 1)
	_ = seedOwnership(t, db, 1) // unrelated ownership

	_, m, bus := newServiceWithCapture(t, db)
	bus.PublishType(event.ClientOverLimit, payload.ClientThreshold{
		NodeID: 1, NodeName: "test", InboundTag: "different-tag",
		ClientEmail: "nobody@example", Up: 1, Down: 1, Limit: 1,
	})
	if got := m.Count(); got != 0 {
		t.Errorf("unknown client should not deliver, got %d", got)
	}
}
