package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
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
	MonthNew     int64 `json:"month_new"`
	PrevMonthNew int64 `json:"prev_month_new"`
	TotalBalance int64 `json:"total_balance_cents"`
	AvgBalance   int64 `json:"avg_balance_cents"`
}

// TrafficStats aggregates byte deltas across the whole fleet over
// two windows: this month (since monthStart) and today (since
// dayStart). The values are bytes; the frontend formats GB/MB.
type TrafficStats struct {
	MonthUp   int64 `json:"month_up_bytes"`
	MonthDown int64 `json:"month_down_bytes"`
	TodayUp   int64 `json:"today_up_bytes"`
	TodayDown int64 `json:"today_down_bytes"`
}

// TrafficRanking is one row in the node / user "top consumer"
// leaderboard. Bytes is up+down combined; the frontend renders the
// progress bar relative to the row with the highest Bytes value.
type TrafficRanking struct {
	Key   string `json:"key"`             // node name or user email
	Up    int64  `json:"up_bytes"`
	Down  int64  `json:"down_bytes"`
	Bytes int64  `json:"bytes"`           // up + down
}

// AuditSeverity buckets recent admin actions by derived severity:
// 5xx → err, 4xx (or non-empty error_msg with 2xx) → warn, else
// info. The frontend uses these for the system-log strip.
type AuditSeverity struct {
	Info int64 `json:"info"`
	Warn int64 `json:"warn"`
	Err  int64 `json:"err"`
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
// number stays in cents — the frontend formats it. MonthNew /
// PrevMonthNew drive the "new this month + % vs last month" chip.
// prevMonthStart bounds last month's window; it can equal monthStart
// (passes through 0) for callers that don't care about the delta.
func (r *StatsRepo) Users(ctx context.Context, monthStart, prevMonthStart time.Time) (UserStats, error) {
	var s UserStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status = 'active')    AS active,
		  COUNT(*) FILTER (WHERE status = 'suspended') AS suspended,
		  COUNT(*) FILTER (WHERE created_at >= ?)      AS month_new,
		  COUNT(*) FILTER (WHERE created_at >= ? AND created_at < ?) AS prev_month_new,
		  COALESCE(SUM(balance_cents), 0)              AS total_balance,
		  COALESCE(AVG(balance_cents), 0)::bigint      AS avg_balance
		FROM users
	`, monthStart, prevMonthStart, monthStart).Scan(&s).Error
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

// trafficGroupKey identifies one logical traffic stream:
// (node, inbound, client) tuple. Two NULL flavors share keys via
// the empty string so DISTINCT ON groups them together.
type trafficGroupKey struct {
	nodeID      int64
	inboundTag  string
	clientEmail string
}

// sumDeltasOverGroups walks per-group chronological samples and
// returns total up+down across all groups. Counter resets are
// treated the same way as service/traffic.SumDeltas: a sample with
// a lower cum value than its predecessor counts its full value as
// the delta, not a negative number.
//
// Inlined (rather than calling service/traffic.SumDeltas) because
// service/traffic already imports this package — the import would
// cycle.
func sumDeltasOverGroups(grouped map[trafficGroupKey][]model.TrafficSample) (up, down int64) {
	for _, samples := range grouped {
		if len(samples) < 2 {
			continue
		}
		prevUp := samples[0].UpCumBytes
		prevDown := samples[0].DownCumBytes
		for i := 1; i < len(samples); i++ {
			cu := samples[i].UpCumBytes
			cd := samples[i].DownCumBytes
			if cu < prevUp {
				up += cu
			} else {
				up += cu - prevUp
			}
			if cd < prevDown {
				down += cd
			} else {
				down += cd - prevDown
			}
			prevUp = cu
			prevDown = cd
		}
	}
	return up, down
}

// groupSamples sorts the rows chronologically inside each
// (node, inbound, client) group. The caller can then run delta
// math without re-sorting.
func groupSamples(rows []model.TrafficSample) map[trafficGroupKey][]model.TrafficSample {
	grouped := map[trafficGroupKey][]model.TrafficSample{}
	for _, r := range rows {
		k := trafficGroupKey{nodeID: r.NodeID}
		if r.InboundTag != nil {
			k.inboundTag = *r.InboundTag
		}
		if r.ClientEmail != nil {
			k.clientEmail = *r.ClientEmail
		}
		grouped[k] = append(grouped[k], r)
	}
	for k := range grouped {
		sort.Slice(grouped[k], func(i, j int) bool {
			return grouped[k][i].TakenAt.Before(grouped[k][j].TakenAt)
		})
	}
	return grouped
}

// Traffic returns fleet-wide up/down totals for the month and today
// windows. Both are computed by replaying SumDeltas semantics over
// every (node, inbound, client) group in the window — the same
// pipeline used by the per-client history endpoint, so numbers
// reconcile when an operator drills in.
//
// Scale note: this in-Go aggregation is fine for the current sub-100
// users / sub-million samples regime. If traffic_samples grows past
// ~1e7 rows the same math wants to move into SQL — at that point
// the call sites here switch to a windowed-aggregate query without
// touching the StatsResponse shape.
func (r *StatsRepo) Traffic(ctx context.Context, monthStart, dayStart, now time.Time) (TrafficStats, error) {
	monthly, err := r.fetchSamples(ctx, monthStart, now)
	if err != nil {
		return TrafficStats{}, err
	}
	mUp, mDown := sumDeltasOverGroups(groupSamples(monthly))

	// Today's samples are a strict subset of the month window when
	// dayStart >= monthStart (true in production). Filter in Go to
	// avoid a second query against the same primary-index range.
	var today []model.TrafficSample
	if !dayStart.Before(monthStart) {
		today = make([]model.TrafficSample, 0, len(monthly)/30)
		for _, s := range monthly {
			if !s.TakenAt.Before(dayStart) {
				today = append(today, s)
			}
		}
	} else {
		today, err = r.fetchSamples(ctx, dayStart, now)
		if err != nil {
			return TrafficStats{}, err
		}
	}
	tUp, tDown := sumDeltasOverGroups(groupSamples(today))

	return TrafficStats{
		MonthUp:   mUp,
		MonthDown: mDown,
		TodayUp:   tUp,
		TodayDown: tDown,
	}, nil
}

// TopNodes returns the n nodes with the most up+down bytes between
// since and now. Names come from the nodes table; missing rows fall
// back to "node #<id>".
func (r *StatsRepo) TopNodes(ctx context.Context, since, now time.Time, n int) ([]TrafficRanking, error) {
	if n <= 0 {
		n = 6
	}
	samples, err := r.fetchSamples(ctx, since, now)
	if err != nil {
		return nil, err
	}
	type nodeAgg struct{ up, down int64 }
	agg := map[int64]*nodeAgg{}
	for k, samples := range groupSamples(samples) {
		if len(samples) < 2 {
			continue
		}
		up, down := sumDeltasOverGroups(map[trafficGroupKey][]model.TrafficSample{k: samples})
		a, ok := agg[k.nodeID]
		if !ok {
			a = &nodeAgg{}
			agg[k.nodeID] = a
		}
		a.up += up
		a.down += down
	}
	if len(agg) == 0 {
		return []TrafficRanking{}, nil
	}
	// Resolve node names in one query.
	ids := make([]int64, 0, len(agg))
	for id := range agg {
		ids = append(ids, id)
	}
	type nameRow struct {
		ID   int64
		Name string
	}
	var nameRows []nameRow
	if err := r.db.WithContext(ctx).
		Raw(`SELECT id, name FROM nodes WHERE id IN ?`, ids).
		Scan(&nameRows).Error; err != nil {
		return nil, fmt.Errorf("StatsRepo.TopNodes: %w", err)
	}
	names := make(map[int64]string, len(nameRows))
	for _, nr := range nameRows {
		names[nr.ID] = nr.Name
	}
	rows := make([]TrafficRanking, 0, len(agg))
	for id, a := range agg {
		name := names[id]
		if name == "" {
			name = fmt.Sprintf("node #%d", id)
		}
		rows = append(rows, TrafficRanking{
			Key:   name,
			Up:    a.up,
			Down:  a.down,
			Bytes: a.up + a.down,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Bytes > rows[j].Bytes })
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}

// TopUsers returns the n client emails with the most up+down bytes
// between since and now. Samples whose client_email is NULL belong
// to inbound-level rollups; those skip the user leaderboard.
func (r *StatsRepo) TopUsers(ctx context.Context, since, now time.Time, n int) ([]TrafficRanking, error) {
	if n <= 0 {
		n = 6
	}
	samples, err := r.fetchSamples(ctx, since, now)
	if err != nil {
		return nil, err
	}
	type userAgg struct{ up, down int64 }
	agg := map[string]*userAgg{}
	for k, samples := range groupSamples(samples) {
		if k.clientEmail == "" || len(samples) < 2 {
			continue
		}
		up, down := sumDeltasOverGroups(map[trafficGroupKey][]model.TrafficSample{k: samples})
		a, ok := agg[k.clientEmail]
		if !ok {
			a = &userAgg{}
			agg[k.clientEmail] = a
		}
		a.up += up
		a.down += down
	}
	rows := make([]TrafficRanking, 0, len(agg))
	for email, a := range agg {
		rows = append(rows, TrafficRanking{
			Key:   email,
			Up:    a.up,
			Down:  a.down,
			Bytes: a.up + a.down,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Bytes > rows[j].Bytes })
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}

// fetchSamples pulls the raw samples in [since, until]. The caller
// is responsible for grouping + delta math.
func (r *StatsRepo) fetchSamples(ctx context.Context, since, until time.Time) ([]model.TrafficSample, error) {
	var rows []model.TrafficSample
	err := r.db.WithContext(ctx).
		Where("taken_at >= ? AND taken_at <= ?", since, until).
		Order("taken_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("StatsRepo.fetchSamples: %w", err)
	}
	return rows, nil
}

// Audit returns severity counts for admin_actions created since the
// given cutoff. Severity is derived: 5xx → err, 4xx OR non-empty
// error_msg → warn, else info. The query buckets in one pass to
// avoid three round trips.
func (r *StatsRepo) Audit(ctx context.Context, since time.Time) (AuditSeverity, error) {
	var s AuditSeverity
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COUNT(*) FILTER (
		    WHERE status_code BETWEEN 200 AND 399 AND COALESCE(error_msg, '') = ''
		  ) AS info,
		  COUNT(*) FILTER (
		    WHERE status_code BETWEEN 400 AND 499
		       OR (status_code BETWEEN 200 AND 399 AND COALESCE(error_msg, '') <> '')
		  ) AS warn,
		  COUNT(*) FILTER (WHERE status_code >= 500) AS err
		FROM admin_actions
		WHERE created_at >= ?
	`, since).Scan(&s).Error
	if err != nil {
		return AuditSeverity{}, fmt.Errorf("StatsRepo.Audit: %w", err)
	}
	return s, nil
}
