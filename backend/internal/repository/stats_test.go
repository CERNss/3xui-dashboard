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
	got, err := NewStatsRepo(db).Users(ctx)
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
	got, err := NewStatsRepo(db).Users(context.Background())
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
