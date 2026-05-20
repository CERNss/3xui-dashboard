package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// StatsRepo returns aggregates for the admin overview page. Each
// method runs as a small SELECT COUNT/SUM, so a thousand-user
// deployment doesn't ship a thousand-row JSON payload to the
// browser just to render four KPI cards.
type StatsRepo struct{ db *gorm.DB }

// NewStatsRepo binds the repository to db.
func NewStatsRepo(db *gorm.DB) *StatsRepo { return &StatsRepo{db: db} }

// UserStats is the breakdown the admin overview shows for the user
// KPI block.
type UserStats struct {
	Total        int64 `json:"total"`
	Active       int64 `json:"active"`
	Suspended    int64 `json:"suspended"`
	TotalBalance int64 `json:"total_balance_cents"`
	AvgBalance   int64 `json:"avg_balance_cents"`
}

// PlanStats counts plans by enable flag.
type PlanStats struct {
	Total    int64 `json:"total"`
	Enabled  int64 `json:"enabled"`
	Disabled int64 `json:"disabled"`
}

// OrderStats sums revenue + counts by status. MonthRevenue is the
// sum of completed orders created since the first of the current
// month in UTC — matches what the previous client-side computation
// did so the headline number doesn't change shape on cut-over.
type OrderStats struct {
	Total         int64 `json:"total"`
	Completed     int64 `json:"completed"`
	Failed        int64 `json:"failed"`
	Refunded      int64 `json:"refunded"`
	Revenue       int64 `json:"revenue_cents"`
	MonthCount    int64 `json:"month_count"`
	MonthRevenue  int64 `json:"month_revenue_cents"`
}

// RecentOrder is one row in the activity feed — already joined with
// user.email + plan.name so the frontend doesn't have to do its own
// map lookups.
type RecentOrder struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	PlanID     int64     `json:"plan_id"`
	PlanName   string    `json:"plan_name"`
	PriceCents int64     `json:"price_cents"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Users returns the user KPIs. AvgBalance is integer-divided so the
// number stays in cents — the frontend formats it.
func (r *StatsRepo) Users(ctx context.Context) (UserStats, error) {
	var s UserStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status = 'active')    AS active,
		  COUNT(*) FILTER (WHERE status = 'suspended') AS suspended,
		  COALESCE(SUM(balance_cents), 0)              AS total_balance,
		  COALESCE(AVG(balance_cents), 0)::bigint      AS avg_balance
		FROM users
	`).Scan(&s).Error
	if err != nil {
		return UserStats{}, fmt.Errorf("StatsRepo.Users: %w", err)
	}
	return s, nil
}

// Plans returns the plan KPIs.
func (r *StatsRepo) Plans(ctx context.Context) (PlanStats, error) {
	var s PlanStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE enabled = TRUE)  AS enabled,
		  COUNT(*) FILTER (WHERE enabled = FALSE) AS disabled
		FROM plans
	`).Scan(&s).Error
	if err != nil {
		return PlanStats{}, fmt.Errorf("StatsRepo.Plans: %w", err)
	}
	return s, nil
}

// Orders returns the order KPIs. monthStart bounds the "this month"
// revenue — passed as an argument so tests can pin the clock.
func (r *StatsRepo) Orders(ctx context.Context, monthStart time.Time) (OrderStats, error) {
	var s OrderStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status IN ('completed', 'paid')) AS completed,
		  COUNT(*) FILTER (WHERE status = 'failed')   AS failed,
		  COUNT(*) FILTER (WHERE status = 'refunded') AS refunded,
		  COALESCE(SUM(price_cents) FILTER (WHERE status IN ('completed', 'paid')), 0) AS revenue,
		  COUNT(*) FILTER (WHERE status IN ('completed', 'paid') AND created_at >= ?) AS month_count,
		  COALESCE(SUM(price_cents) FILTER (WHERE status IN ('completed', 'paid') AND created_at >= ?), 0) AS month_revenue
		FROM orders
	`, monthStart, monthStart).Scan(&s).Error
	if err != nil {
		return OrderStats{}, fmt.Errorf("StatsRepo.Orders: %w", err)
	}
	return s, nil
}

// RecentOrders returns the n newest orders pre-joined with the
// user's email and the plan's name. Empty user.email collapses to
// "User #ID" client-side; we leave that to the frontend rather than
// branching here.
func (r *StatsRepo) RecentOrders(ctx context.Context, n int) ([]RecentOrder, error) {
	if n <= 0 {
		n = 5
	}
	var rows []RecentOrder
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  o.id          AS id,
		  o.user_id     AS user_id,
		  COALESCE(u.email, '') AS user_email,
		  o.plan_id     AS plan_id,
		  COALESCE(p.name, '')  AS plan_name,
		  o.price_cents AS price_cents,
		  o.status      AS status,
		  o.created_at  AS created_at
		FROM orders o
		LEFT JOIN users u ON u.id = o.user_id
		LEFT JOIN plans p ON p.id = o.plan_id
		ORDER BY o.created_at DESC
		LIMIT ?
	`, n).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("StatsRepo.RecentOrders: %w", err)
	}
	return rows, nil
}
