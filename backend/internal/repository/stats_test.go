package repository

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
)

// setupStatsDB mirrors the pattern from job/expiry_test: integration
// DB required, schema reset per test invocation, migrations run.
// Skips cleanly when the env var is unset so unit suites stay green
// without a Postgres dependency.
func setupStatsDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping stats DB tests")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := Open(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := MigrateUp(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })
	return db
}

func TestStatsRepo_Users_CountsByStatus(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	emailA, emailB, emailC := "a@x", "b@x", "c@x"
	users := []model.User{
		{Email: &emailA, SubID: "sa", Status: "active", BalanceCents: 100},
		{Email: &emailB, SubID: "sb", Status: "active", BalanceCents: 200},
		{Email: &emailC, SubID: "sc", Status: "suspended", BalanceCents: 0},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	monthStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := monthStart.AddDate(0, -1, 0)
	got, err := NewStatsRepo(db).Users(ctx, monthStart, prevMonthStart)
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	if got.Total != 3 {
		t.Errorf("Total = %d, want 3", got.Total)
	}
	if got.Active != 2 {
		t.Errorf("Active = %d, want 2", got.Active)
	}
	if got.Suspended != 1 {
		t.Errorf("Suspended = %d, want 1", got.Suspended)
	}
	if got.TotalBalance != 300 {
		t.Errorf("TotalBalance = %d, want 300", got.TotalBalance)
	}
	if got.AvgBalance != 100 {
		// (100 + 200 + 0) / 3 = 100
		t.Errorf("AvgBalance = %d, want 100", got.AvgBalance)
	}
}

func TestStatsRepo_Users_EmptyDB(t *testing.T) {
	db := setupStatsDB(t)
	monthStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	got, err := NewStatsRepo(db).Users(context.Background(), monthStart, monthStart.AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	if got.Total != 0 || got.AvgBalance != 0 || got.TotalBalance != 0 {
		t.Errorf("empty DB returned non-zero stats: %+v", got)
	}
}

func TestStatsRepo_Plans_CountsByEnabled(t *testing.T) {
	db := setupStatsDB(t)
	plans := []model.Plan{
		{Name: "A", DurationDays: 30, TrafficLimitBytes: 1 << 30, PriceCents: 500, Enabled: true},
		{Name: "B", DurationDays: 30, TrafficLimitBytes: 1 << 30, PriceCents: 1000, Enabled: true},
		{Name: "C", DurationDays: 30, TrafficLimitBytes: 1 << 30, PriceCents: 1500, Enabled: false},
	}
	for i := range plans {
		if err := db.Create(&plans[i]).Error; err != nil {
			t.Fatalf("create plan: %v", err)
		}
	}
	got, err := NewStatsRepo(db).Plans(context.Background())
	if err != nil {
		t.Fatalf("Plans: %v", err)
	}
	if got.Total != 3 || got.Enabled != 2 || got.Disabled != 1 {
		t.Errorf("Plans stats = %+v, want total=3 enabled=2 disabled=1", got)
	}
}

func TestStatsRepo_Orders_RevenueAndMonth(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	// Need a user + plan for FK
	uEmail := "u@x"
	u := model.User{Email: &uEmail, SubID: "u1", Status: "active"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := model.Plan{Name: "P", DurationDays: 30, TrafficLimitBytes: 1 << 30, PriceCents: 500, Enabled: true}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastMonth := monthStart.AddDate(0, 0, -5)

	orders := []model.Order{
		// 2 completed this month — month_count=2, month_revenue=300
		{UserID: u.ID, PlanID: p.ID, IdempotencyKey: "k1", PriceCents: 100, Status: "completed", CreatedAt: now},
		{UserID: u.ID, PlanID: p.ID, IdempotencyKey: "k2", PriceCents: 200, Status: "completed", CreatedAt: now},
		// 1 completed last month — counted in total revenue but not month
		{UserID: u.ID, PlanID: p.ID, IdempotencyKey: "k3", PriceCents: 500, Status: "completed", CreatedAt: lastMonth},
		// 1 failed, 1 refunded — counted in their buckets
		{UserID: u.ID, PlanID: p.ID, IdempotencyKey: "k4", PriceCents: 100, Status: "failed", CreatedAt: now},
		{UserID: u.ID, PlanID: p.ID, IdempotencyKey: "k5", PriceCents: 100, Status: "refunded", CreatedAt: now},
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("create order: %v", err)
		}
	}

	got, err := NewStatsRepo(db).Orders(ctx, monthStart)
	if err != nil {
		t.Fatalf("Orders: %v", err)
	}
	if got.Total != 5 {
		t.Errorf("Total = %d, want 5", got.Total)
	}
	if got.Completed != 3 {
		t.Errorf("Completed = %d, want 3", got.Completed)
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
	if got.Refunded != 1 {
		t.Errorf("Refunded = %d, want 1", got.Refunded)
	}
	if got.Revenue != 800 {
		t.Errorf("Revenue = %d, want 800", got.Revenue)
	}
	if got.MonthCount != 2 {
		t.Errorf("MonthCount = %d, want 2", got.MonthCount)
	}
	if got.MonthRevenue != 300 {
		t.Errorf("MonthRevenue = %d, want 300", got.MonthRevenue)
	}
}

func TestStatsRepo_RecentOrders_JoinsAndLimits(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	userEmail := "user@x"
	u := model.User{Email: &userEmail, SubID: "s1", Status: "active"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := model.Plan{Name: "Pro", DurationDays: 30, TrafficLimitBytes: 1 << 30, PriceCents: 999, Enabled: true}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	base := time.Now().UTC()
	for i := 0; i < 7; i++ {
		o := model.Order{
			UserID: u.ID, PlanID: p.ID,
			IdempotencyKey: time.Now().Format("k150405") + string(rune('a'+i)),
			PriceCents:     100, Status: "completed",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := db.Create(&o).Error; err != nil {
			t.Fatalf("create order %d: %v", i, err)
		}
	}

	got, err := NewStatsRepo(db).RecentOrders(ctx, 5)
	if err != nil {
		t.Fatalf("RecentOrders: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d rows, want 5", len(got))
	}
	// Newest first
	for i := 1; i < len(got); i++ {
		if !got[i-1].CreatedAt.After(got[i].CreatedAt) {
			t.Errorf("not sorted desc at %d: %v -> %v", i, got[i-1].CreatedAt, got[i].CreatedAt)
		}
	}
	if got[0].UserEmail != "user@x" {
		t.Errorf("UserEmail = %q, want user@x", got[0].UserEmail)
	}
	if got[0].PlanName != "Pro" {
		t.Errorf("PlanName = %q, want Pro", got[0].PlanName)
	}
}

// seedNode is a minimal helper for the traffic-rank tests: just
// enough columns to satisfy NOT NULL constraints.
func seedNode(t *testing.T, db *gorm.DB, name string) int64 {
	t.Helper()
	n := model.Node{Name: name, Scheme: "https", Host: name + ".test", Port: 443, APIToken: "t-" + name, Enabled: true}
	if err := db.Create(&n).Error; err != nil {
		t.Fatalf("seed node %s: %v", name, err)
	}
	return n.ID
}

// seedSample inserts one cum-counter reading. Pass nil for inbound
// or email when seeding the per-node rollup row.
func seedSample(t *testing.T, db *gorm.DB, nodeID int64, inbound, email *string, up, down int64, at time.Time) {
	t.Helper()
	s := model.TrafficSample{
		NodeID:       nodeID,
		InboundTag:   inbound,
		ClientEmail:  email,
		UpCumBytes:   up,
		DownCumBytes: down,
		TakenAt:      at,
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("seed sample: %v", err)
	}
}

func TestStatsRepo_Traffic_AggregatesDeltasOverGroups(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID := seedNode(t, db, "edge-jp")
	inbound := "vmess-in"
	email := "alice@example.com"

	dayStart := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	now := dayStart.Add(2 * time.Hour)

	// Two samples for one client: 100→500 up, 1000→3000 down → +400 / +2000
	seedSample(t, db, nodeID, &inbound, &email, 100, 1000, dayStart.Add(10*time.Minute))
	seedSample(t, db, nodeID, &inbound, &email, 500, 3000, dayStart.Add(70*time.Minute))
	// One earlier sample inside month (before today) that's a different group.
	otherEmail := "bob@example.com"
	seedSample(t, db, nodeID, &inbound, &otherEmail, 0, 0, monthStart.Add(time.Hour))
	seedSample(t, db, nodeID, &inbound, &otherEmail, 100, 200, monthStart.Add(2*time.Hour))

	got, err := NewStatsRepo(db).Traffic(ctx, monthStart, dayStart, now)
	if err != nil {
		t.Fatalf("Traffic: %v", err)
	}
	// today window includes only alice → 400 / 2000
	if got.TodayUp != 400 || got.TodayDown != 2000 {
		t.Errorf("today = %d/%d, want 400/2000", got.TodayUp, got.TodayDown)
	}
	// month window includes both → alice 400/2000 + bob 100/200 = 500/2200
	if got.MonthUp != 500 || got.MonthDown != 2200 {
		t.Errorf("month = %d/%d, want 500/2200", got.MonthUp, got.MonthDown)
	}
}

func TestStatsRepo_Traffic_CounterResetCountsFullSample(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID := seedNode(t, db, "edge-uk")
	inbound := "vless-in"
	email := "carol@example.com"
	dayStart := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	monthStart := dayStart.AddDate(0, 0, -23)
	now := dayStart.Add(time.Hour)

	// 1000 → reset to 50 → 250: delta is 50 (reset) + 200 (normal) = 250.
	seedSample(t, db, nodeID, &inbound, &email, 1000, 1000, dayStart.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &email, 50, 50, dayStart.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &email, 250, 250, dayStart.Add(3*time.Minute))

	got, err := NewStatsRepo(db).Traffic(ctx, monthStart, dayStart, now)
	if err != nil {
		t.Fatalf("Traffic: %v", err)
	}
	if got.TodayUp != 250 || got.TodayDown != 250 {
		t.Errorf("today = %d/%d, want 250/250 (50 reset + 200 normal)", got.TodayUp, got.TodayDown)
	}
}

func TestStatsRepo_Traffic_PrefersInboundRollupOverClientSamples(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID := seedNode(t, db, "edge-hk")
	inbound := "reality-in"
	alice := "alice@example.com"
	bob := "bob@example.com"

	dayStart := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	now := dayStart.Add(time.Hour)

	// The collector stores both an inbound rollup and per-client rows for
	// the same counters. Fleet totals must use the rollup once, not rollup
	// plus every client row.
	seedSample(t, db, nodeID, &inbound, nil, 0, 0, dayStart.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, nil, 1000, 2000, dayStart.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &alice, 0, 0, dayStart.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &alice, 400, 900, dayStart.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 0, 0, dayStart.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 600, 1100, dayStart.Add(2*time.Minute))

	got, err := NewStatsRepo(db).Traffic(ctx, monthStart, dayStart, now)
	if err != nil {
		t.Fatalf("Traffic: %v", err)
	}
	if got.TodayUp != 1000 || got.TodayDown != 2000 {
		t.Errorf("today = %d/%d, want inbound rollup 1000/2000 without double counting clients", got.TodayUp, got.TodayDown)
	}
	if got.MonthUp != 1000 || got.MonthDown != 2000 {
		t.Errorf("month = %d/%d, want inbound rollup 1000/2000 without double counting clients", got.MonthUp, got.MonthDown)
	}
}

func TestStatsRepo_TopNodes_RanksByTotalBytes(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	n1 := seedNode(t, db, "edge-jp")
	n2 := seedNode(t, db, "edge-uk")
	n3 := seedNode(t, db, "edge-sg")
	inbound := "vmess-in"
	email := "alice@example.com"

	since := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	now := since.Add(time.Hour)

	// Bytes per node: n1=5000, n2=100, n3=500 → expect n1, n3, n2
	seedSample(t, db, n1, &inbound, &email, 0, 0, since.Add(time.Minute))
	seedSample(t, db, n1, &inbound, &email, 1000, 4000, since.Add(2*time.Minute))
	seedSample(t, db, n2, &inbound, &email, 0, 0, since.Add(time.Minute))
	seedSample(t, db, n2, &inbound, &email, 40, 60, since.Add(2*time.Minute))
	seedSample(t, db, n3, &inbound, &email, 0, 0, since.Add(time.Minute))
	seedSample(t, db, n3, &inbound, &email, 200, 300, since.Add(2*time.Minute))

	got, err := NewStatsRepo(db).TopNodes(ctx, since, now, 6)
	if err != nil {
		t.Fatalf("TopNodes: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(got), got)
	}
	if got[0].Key != "edge-jp" || got[0].Bytes != 5000 {
		t.Errorf("rank0 = %+v, want edge-jp/5000", got[0])
	}
	if got[1].Key != "edge-sg" || got[1].Bytes != 500 {
		t.Errorf("rank1 = %+v, want edge-sg/500", got[1])
	}
	if got[2].Key != "edge-uk" || got[2].Bytes != 100 {
		t.Errorf("rank2 = %+v, want edge-uk/100", got[2])
	}
}

func TestStatsRepo_TopNodes_PrefersInboundRollupOverClientSamples(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID := seedNode(t, db, "edge-hk")
	inbound := "reality-in"
	alice := "alice@example.com"
	bob := "bob@example.com"
	since := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	now := since.Add(time.Hour)

	seedSample(t, db, nodeID, &inbound, nil, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, nil, 1000, 2000, since.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &alice, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &alice, 400, 900, since.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 600, 1100, since.Add(2*time.Minute))

	got, err := NewStatsRepo(db).TopNodes(ctx, since, now, 6)
	if err != nil {
		t.Fatalf("TopNodes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(got), got)
	}
	if got[0].Key != "edge-hk" || got[0].Up != 1000 || got[0].Down != 2000 || got[0].Bytes != 3000 {
		t.Errorf("rank0 = %+v, want edge-hk up=1000 down=2000 bytes=3000 without double counting clients", got[0])
	}
}

func TestStatsRepo_TopUsers_SkipsNullEmail(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID := seedNode(t, db, "edge-jp")
	inbound := "vmess-in"
	alice := "alice@example.com"
	bob := "bob@example.com"
	since := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	now := since.Add(time.Hour)

	// alice: 600 bytes, bob: 100 bytes, inbound-level rollup (no email): 9999 — must be skipped.
	seedSample(t, db, nodeID, &inbound, &alice, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &alice, 200, 400, since.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, &bob, 50, 50, since.Add(2*time.Minute))
	seedSample(t, db, nodeID, &inbound, nil, 0, 0, since.Add(time.Minute))
	seedSample(t, db, nodeID, &inbound, nil, 5000, 4999, since.Add(2*time.Minute))

	got, err := NewStatsRepo(db).TopUsers(ctx, since, now, 6)
	if err != nil {
		t.Fatalf("TopUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (inbound rollup must be skipped): %+v", len(got), got)
	}
	if got[0].Key != alice || got[0].Bytes != 600 {
		t.Errorf("rank0 = %+v, want alice/600", got[0])
	}
	if got[1].Key != bob || got[1].Bytes != 100 {
		t.Errorf("rank1 = %+v, want bob/100", got[1])
	}
}

func TestStatsRepo_Audit_BucketsBySeverity(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mk := func(status int, errMsg string, at time.Time) {
		a := model.AdminAction{
			AdminUsername: "admin", Method: "POST", Path: "/x",
			TargetResource: "user", TargetID: "1",
			IP: "127.0.0.1", StatusCode: status, ErrorMsg: errMsg, CreatedAt: at,
		}
		if err := db.Create(&a).Error; err != nil {
			t.Fatalf("seed action: %v", err)
		}
	}
	// 2 info (2xx, no error_msg), 2 warn (1 × 4xx + 1 × 2xx with error_msg),
	// 1 err (5xx). One row before `since` should be filtered out.
	mk(200, "", since.Add(time.Hour))
	mk(204, "", since.Add(2*time.Hour))
	mk(404, "not found", since.Add(3*time.Hour))
	mk(200, "soft error rendered to caller", since.Add(4*time.Hour))
	mk(503, "downstream timeout", since.Add(5*time.Hour))
	mk(500, "ignored", since.Add(-time.Hour)) // before since — excluded

	got, err := NewStatsRepo(db).Audit(ctx, since)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if got.Info != 2 || got.Warn != 2 || got.Err != 1 {
		t.Errorf("severity = %+v, want info=2 warn=2 err=1", got)
	}
}

func TestStatsRepo_Users_MonthNewBreakdown(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	monthStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := monthStart.AddDate(0, -1, 0)

	mk := func(email string, createdAt time.Time) {
		u := model.User{Email: &email, SubID: "sid-" + email, Status: "active", CreatedAt: createdAt}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	mk("old@x.com", prevMonthStart.AddDate(0, -1, 0)) // before prev month
	mk("prev1@x.com", prevMonthStart.Add(24*time.Hour))
	mk("prev2@x.com", monthStart.Add(-time.Hour))
	mk("new1@x.com", monthStart.Add(time.Hour))
	mk("new2@x.com", monthStart.Add(2*time.Hour))
	mk("new3@x.com", monthStart.Add(3*time.Hour))

	got, err := NewStatsRepo(db).Users(ctx, monthStart, prevMonthStart)
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	if got.MonthNew != 3 {
		t.Errorf("MonthNew = %d, want 3", got.MonthNew)
	}
	if got.PrevMonthNew != 2 {
		t.Errorf("PrevMonthNew = %d, want 2", got.PrevMonthNew)
	}
}
