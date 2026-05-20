package job

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
)

// setupDB mirrors the pattern used by service/verification tests:
// open against INTEGRATION_DB_URL, nuke + recreate the schema, run
// migrations. Tests skip cleanly when the env var is unset.
func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping expiry DB tests")
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

// recordingBus collects every event Publish call for later inspection.
// Replaces a real event.Bus in tests so we don't need to wire webhook
// subscribers to observe what the job published.
type recordingBus struct {
	mu     sync.Mutex
	events []event.Event
}

func (b *recordingBus) publishType(typ string, payload any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event.Event{
		Type: typ, Time: time.Now(), Data: payload,
	})
}

// newJobWithRealBus is the standard fixture — the actual ExpiryJob
// uses event.Bus which is concrete (not an interface), so we wire a
// real bus and subscribe to capture events.
func newJobWithRealBus(t *testing.T, db *gorm.DB) (*ExpiryJob, *recordingBus) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	_ = logger
	bus := event.New()
	rec := &recordingBus{}
	// Subscribe a recorder on every relevant type.
	for _, typ := range []string{event.ClientExpired, event.ClientExpiringSoon} {
		typ := typ
		bus.Subscribe(typ, func(e event.Event) { rec.publishType(typ, e.Data) })
	}
	j := NewExpiryJob(
		repository.NewClientOwnershipRepo(db),
		repository.NewSettingRepo(db),
		repository.NewUserRepo(db),
		bus,
		logger,
	)
	return j, rec
}

// ensureUser inserts a user row with the given id (idempotent — used so
// the FK on client_ownerships is satisfied).
func ensureUser(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	var existing model.User
	if err := db.First(&existing, id).Error; err == nil {
		return
	}
	subID := "sub-" + time.Now().UTC().Format("150405.000000000")
	u := &model.User{ID: id, SubID: subID, Status: model.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("ensureUser %d: %v", id, err)
	}
}

// ensureNode inserts a node row (idempotent) so the FK on
// client_ownerships.node_id is satisfied.
func ensureNode(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	var existing model.Node
	if err := db.First(&existing, id).Error; err == nil {
		return
	}
	n := &model.Node{
		ID:       id,
		Name:     "test-node",
		Scheme:   "http",
		Host:     "127.0.0.1",
		Port:     7777,
		APIToken: "fake",
		Enabled:  true,
		Status:   "unknown",
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("ensureNode %d: %v", id, err)
	}
}

func insertOwnership(t *testing.T, db *gorm.DB, userID int64, expiresAt *time.Time, enabled bool) *model.ClientOwnership {
	t.Helper()
	ensureUser(t, db, userID)
	ensureNode(t, db, 1)
	o := &model.ClientOwnership{
		UserID:      userID,
		NodeID:      1,
		InboundTag:  "vless-1",
		ClientEmail: "test-" + time.Now().UTC().Format("150405.000000000"),
		ExpiresAt:   expiresAt,
		Enabled:     enabled,
	}
	if err := db.Create(o).Error; err != nil {
		t.Fatalf("insert ownership: %v", err)
	}
	return o
}

func TestRunOnce_FlipsExpiredRowAndEmitsEvent(t *testing.T) {
	db := setupDB(t)
	past := time.Now().UTC().Add(-1 * time.Hour)
	o := insertOwnership(t, db, 1, &past, true)

	j, rec := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())

	// Reload — should be disabled now.
	var reloaded model.ClientOwnership
	if err := db.First(&reloaded, o.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Enabled {
		t.Errorf("expired ownership should be disabled, still enabled=true")
	}

	// One client.expired event was published.
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var expiredCount int
	for _, e := range rec.events {
		if e.Type == event.ClientExpired {
			expiredCount++
		}
	}
	if expiredCount != 1 {
		t.Errorf("expected 1 client.expired event, got %d", expiredCount)
	}
}

func TestRunOnce_SkipsNonExpiredRow(t *testing.T) {
	db := setupDB(t)
	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	o := insertOwnership(t, db, 1, &future, true)

	j, rec := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())

	var reloaded model.ClientOwnership
	_ = db.First(&reloaded, o.ID).Error
	if !reloaded.Enabled {
		t.Errorf("future-expiry row should stay enabled")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, e := range rec.events {
		if e.Type == event.ClientExpired {
			t.Errorf("should not emit client.expired for future row")
		}
	}
}

func TestRunOnce_SkipsAlreadyDisabledRow(t *testing.T) {
	db := setupDB(t)
	past := time.Now().UTC().Add(-1 * time.Hour)
	o := insertOwnership(t, db, 1, &past, false)

	j, _ := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())

	var reloaded model.ClientOwnership
	_ = db.First(&reloaded, o.ID).Error
	// Stays disabled (the ListExpired query filters enabled=true so the
	// already-disabled row never enters the loop).
	if reloaded.Enabled {
		t.Errorf("already-disabled row should stay disabled")
	}
}

func TestRunOnce_EmitsExpiringSoon(t *testing.T) {
	db := setupDB(t)
	// 2 days from now — within default warn window of 3 days.
	soon := time.Now().UTC().Add(2 * 24 * time.Hour)
	o := insertOwnership(t, db, 1, &soon, true)

	j, rec := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())

	// Row should NOT be disabled yet (it's not expired).
	var reloaded model.ClientOwnership
	_ = db.First(&reloaded, o.ID).Error
	if !reloaded.Enabled {
		t.Errorf("expiring-soon row should not be disabled")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	var soonCount int
	for _, e := range rec.events {
		if e.Type == event.ClientExpiringSoon {
			soonCount++
		}
	}
	if soonCount != 1 {
		t.Errorf("expected 1 client.expiring_soon, got %d", soonCount)
	}
}

func TestRunOnce_DedupePreventsRepeatedSoon(t *testing.T) {
	db := setupDB(t)
	soon := time.Now().UTC().Add(2 * 24 * time.Hour)
	insertOwnership(t, db, 1, &soon, true)

	j, rec := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())
	j.RunOnce(context.Background()) // second pass — should dedup

	rec.mu.Lock()
	defer rec.mu.Unlock()
	var soonCount int
	for _, e := range rec.events {
		if e.Type == event.ClientExpiringSoon {
			soonCount++
		}
	}
	if soonCount != 1 {
		t.Errorf("expected 1 client.expiring_soon (dedup), got %d", soonCount)
	}
}

func TestRunOnce_NilExpiresAtIsSkipped(t *testing.T) {
	db := setupDB(t)
	// ownership with no expiry — should never expire or warn.
	o := insertOwnership(t, db, 1, nil, true)

	j, rec := newJobWithRealBus(t, db)
	j.RunOnce(context.Background())

	var reloaded model.ClientOwnership
	_ = db.First(&reloaded, o.ID).Error
	if !reloaded.Enabled {
		t.Errorf("nil-expires row should stay enabled")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) != 0 {
		t.Errorf("nil-expires row should emit no events, got %d", len(rec.events))
	}
}

func TestWarnDays_DefaultAndOverride(t *testing.T) {
	db := setupDB(t)
	j, _ := newJobWithRealBus(t, db)

	if d := j.warnDays(context.Background()); d != 3 {
		t.Errorf("default warnDays = %d, want 3", d)
	}

	// Operator override via setting
	settings := repository.NewSettingRepo(db)
	if err := settings.Set(context.Background(), model.SettingExpiryWarnDays, "7"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if d := j.warnDays(context.Background()); d != 7 {
		t.Errorf("override warnDays = %d, want 7", d)
	}

	// Malformed override falls back to default
	if err := settings.Set(context.Background(), model.SettingExpiryWarnDays, "not-a-number"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if d := j.warnDays(context.Background()); d != 3 {
		t.Errorf("malformed warnDays should fall back to 3, got %d", d)
	}
}
