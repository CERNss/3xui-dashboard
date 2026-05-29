package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/datacollection"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
)

// NodeListSource enumerates enabled nodes for the collection job.
type NodeListSource interface {
	ListEnabledNodes(ctx context.Context) ([]NodeRef, error)
}

// NodeRef is the minimum node identification this package needs.
type NodeRef struct {
	ID   int64
	Name string
}

// Service composes the runtime manager, repos, event bus, and
// dedup state for the threshold/expiry rules.
type Service struct {
	rt        *runtime.Manager
	samples   *repository.TrafficSampleRepo
	ownership *repository.ClientOwnershipRepo
	plans     PlanLookup
	clients   ClientEnableSetter
	nodes     NodeListSource
	bus       *event.Bus
	log       *slog.Logger

	dedup sync.Map // key string → last emit time.Time

	// Concurrency cap for fleet walks. Zero = default (8).
	FleetConcurrency int
}

// PlanLookup is the slice of plan repo behaviour the shared-quota
// enforcement needs. Decoupled so tests can stub.
type PlanLookup interface {
	Get(ctx context.Context, id int64) (*model.Plan, error)
}

// ClientEnableSetter abstracts the panel-side "flip enable bit"
// operation needed by shared-quota enforcement. Satisfied by
// service/client.Service.SetClientEnabled.
type ClientEnableSetter interface {
	SetClientEnabled(ctx context.Context, nodeID int64, inboundTag, email string, enabled bool) error
}

type CollectOptions struct {
	Concurrency    int
	PerNodeTimeout time.Duration
	RetryAttempts  int
}

// New constructs the service.
func New(rt *runtime.Manager, samples *repository.TrafficSampleRepo, ownership *repository.ClientOwnershipRepo, nodes NodeListSource, bus *event.Bus, lg *slog.Logger) *Service {
	return &Service{
		rt:        rt,
		samples:   samples,
		ownership: ownership,
		nodes:     nodes,
		bus:       bus,
		log:       lg.With(slog.String("component", "service.traffic")),
	}
}

// SetSharedQuotaDeps wires the plans + client.SetEnabled deps the
// shared-quota enforcement path needs. Called once from app wiring
// after both repos exist. If left unwired EnforceSharedQuotas is a
// no-op (used by single-target deployments / tests).
func (s *Service) SetSharedQuotaDeps(plans PlanLookup, clients ClientEnableSetter) {
	s.plans = plans
	s.clients = clients
}

// CollectAll fans out across every enabled node, fetches a traffic
// snapshot per node, and persists rows in one batch insert per node.
// Per-node failures never abort the pass; they're logged and the
// per-node error is collected into the returned map.
func (s *Service) CollectAll(ctx context.Context, now time.Time) (map[int64]string, error) {
	return s.CollectAllWithOptions(ctx, now, CollectOptions{})
}

func (s *Service) CollectAllWithOptions(ctx context.Context, now time.Time, opts CollectOptions) (map[int64]string, error) {
	nodes, err := s.nodes.ListEnabledNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("traffic.CollectAll: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	opts = s.normalizeCollectOptions(opts)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	var (
		mu     sync.Mutex
		errMap = map[int64]string{}
	)

	for i := range nodes {
		n := nodes[i]
		g.Go(func() error {
			if err := s.collectOneWithPolicy(gctx, n, now, opts); err != nil {
				mu.Lock()
				errMap[n.ID] = err.Error()
				mu.Unlock()
				s.log.Warn("traffic collect failed",
					slog.Int64("node_id", n.ID),
					slog.String("node", n.Name),
					slog.String("error", err.Error()),
				)
			}
			return nil
		})
	}
	_ = g.Wait()
	if len(errMap) == 0 {
		return nil, nil
	}
	return errMap, nil
}

func (s *Service) normalizeCollectOptions(opts CollectOptions) CollectOptions {
	if opts.Concurrency <= 0 {
		opts.Concurrency = s.FleetConcurrency
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = datacollection.DefaultConcurrency
	}
	if opts.Concurrency > datacollection.MaxConcurrency {
		opts.Concurrency = datacollection.MaxConcurrency
	}
	if opts.PerNodeTimeout <= 0 {
		opts.PerNodeTimeout = datacollection.DefaultTrafficTimeout
	}
	if opts.PerNodeTimeout < datacollection.MinTimeout {
		opts.PerNodeTimeout = datacollection.MinTimeout
	}
	if opts.PerNodeTimeout > datacollection.MaxTimeout {
		opts.PerNodeTimeout = datacollection.MaxTimeout
	}
	if opts.RetryAttempts < 0 {
		opts.RetryAttempts = datacollection.DefaultRetryAttempts
	}
	if opts.RetryAttempts > datacollection.MaxRetryAttempts {
		opts.RetryAttempts = datacollection.MaxRetryAttempts
	}
	return opts
}

// DeleteSamplesOlderThan trims persisted traffic samples before cutoff.
func (s *Service) DeleteSamplesOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.samples.DeleteOlderThan(ctx, cutoff)
}

func (s *Service) collectOneWithPolicy(ctx context.Context, n NodeRef, now time.Time, opts CollectOptions) error {
	var snap *runtime.TrafficSnapshot
	var err error
	for attempt := 0; attempt <= opts.RetryAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, opts.PerNodeTimeout)
		snap, err = s.fetchTrafficSnapshot(callCtx, n)
		cancel()
		if err == nil || ctx.Err() != nil {
			break
		}
		if attempt < opts.RetryAttempts {
			s.log.Warn("traffic collect failed; retrying",
				slog.Int64("node_id", n.ID),
				slog.String("node", n.Name),
				slog.Int("attempt", attempt+1),
				slog.Int("max_attempts", opts.RetryAttempts+1),
				slog.String("error", err.Error()),
			)
			if !datacollection.SleepWithContext(ctx, datacollection.RetryBackoff(attempt)) {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	return s.persistTrafficSnapshot(ctx, n, snap, now)
}

func (s *Service) fetchTrafficSnapshot(ctx context.Context, n NodeRef) (*runtime.TrafficSnapshot, error) {
	r, err := s.rt.Get(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	return r.FetchTrafficSnapshot(ctx)
}

func (s *Service) persistTrafficSnapshot(ctx context.Context, n NodeRef, snap *runtime.TrafficSnapshot, now time.Time) error {
	if snap == nil {
		return nil
	}

	rows := make([]model.TrafficSample, 0, 4*len(snap.Inbounds))
	for _, in := range snap.Inbounds {
		tag := in.Tag
		// inbound-level row
		rows = append(rows, model.TrafficSample{
			NodeID:       n.ID,
			InboundTag:   strPtr(tag),
			UpCumBytes:   in.Up,
			DownCumBytes: in.Down,
			TakenAt:      now,
		})
		for _, c := range in.ClientStats {
			email := c.Email
			rows = append(rows, model.TrafficSample{
				NodeID:       n.ID,
				InboundTag:   strPtr(tag),
				ClientEmail:  strPtr(email),
				UpCumBytes:   c.Up,
				DownCumBytes: c.Down,
				TakenAt:      now,
			})
		}
	}
	if err := s.samples.InsertBatch(ctx, rows); err != nil {
		return err
	}
	s.evaluateRules(ctx, n, snap, now)
	return nil
}

// evaluateRules walks the snapshot for over-limit / expired clients
// and publishes deduped events. Dedup window is hardcoded at 6h —
// re-emitting more often than that just adds noise.
func (s *Service) evaluateRules(ctx context.Context, n NodeRef, snap *runtime.TrafficSnapshot, now time.Time) {
	const dedupWindow = 6 * time.Hour
	for _, in := range snap.Inbounds {
		for _, c := range in.ClientStats {
			if c.Total > 0 && (c.Up+c.Down) >= c.Total {
				key := fmt.Sprintf("over_limit|%d|%s|%s", n.ID, in.Tag, c.Email)
				if s.shouldEmit(key, now, dedupWindow) {
					s.bus.PublishType(event.ClientOverLimit, ClientThresholdPayload{
						NodeID: n.ID, NodeName: n.Name, InboundTag: in.Tag,
						ClientEmail: c.Email, Up: c.Up, Down: c.Down, Limit: c.Total,
					})
				}
			}
			if c.ExpiryTime > 0 && time.UnixMilli(c.ExpiryTime).Before(now) {
				key := fmt.Sprintf("expired|%d|%s|%s", n.ID, in.Tag, c.Email)
				if s.shouldEmit(key, now, dedupWindow) {
					s.bus.PublishType(event.ClientExpired, ClientExpiredPayload{
						NodeID: n.ID, NodeName: n.Name, InboundTag: in.Tag,
						ClientEmail: c.Email,
						ExpiredAt:   time.UnixMilli(c.ExpiryTime).UTC(),
					})
				}
			}
		}
	}
}

func (s *Service) shouldEmit(key string, now time.Time, window time.Duration) bool {
	if prev, ok := s.dedup.Load(key); ok {
		if t, ok := prev.(time.Time); ok && now.Sub(t) < window {
			return false
		}
	}
	s.dedup.Store(key, now)
	return true
}

// ---- Usage queries --------------------------------------------------------

// ClientUsage is what GET admin / user traffic endpoints return for
// one ownership row.
type ClientUsage struct {
	NodeID       int64      `json:"node_id"`
	InboundTag   string     `json:"inbound_tag"`
	ClientEmail  string     `json:"client_email"`
	Up           int64      `json:"up"`
	Down         int64      `json:"down"`
	TotalBytes   int64      `json:"total"`
	LimitBytes   *int64     `json:"limit,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	LastSampleAt *time.Time `json:"last_sample_at,omitempty"`
}

// UsageForOwnership returns the cumulative-derived usage for one
// ownership row over [from, to]. Counter-reset safe.
func (s *Service) UsageForOwnership(ctx context.Context, o *model.ClientOwnership, from, to time.Time) (*ClientUsage, error) {
	rows, err := s.samples.ChronologicalForClient(ctx, o.NodeID, o.InboundTag, o.ClientEmail, from, to)
	if err != nil {
		return nil, err
	}
	up, down := SumDeltas(rows)
	usage := &ClientUsage{
		NodeID:      o.NodeID,
		InboundTag:  o.InboundTag,
		ClientEmail: o.ClientEmail,
		Up:          up,
		Down:        down,
		TotalBytes:  up + down,
		LimitBytes:  o.TrafficLimitBytes,
		ExpiresAt:   o.ExpiresAt,
	}
	if len(rows) > 0 {
		t := rows[len(rows)-1].TakenAt
		usage.LastSampleAt = &t
	}
	return usage, nil
}

// UsageForUser returns the per-ownership usage rows for one user,
// scoped to clients the user actually owns. Used by the portal.
func (s *Service) UsageForUser(ctx context.Context, userID int64, from, to time.Time) ([]ClientUsage, error) {
	owns, err := s.ownership.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ClientUsage, 0, len(owns))
	for i := range owns {
		usage, err := s.UsageForOwnership(ctx, &owns[i], from, to)
		if err != nil {
			s.log.Warn("usage compute failed",
				slog.Int64("ownership_id", owns[i].ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		out = append(out, *usage)
	}
	return out, nil
}

// HistoryForOwnership returns time-bucketed usage points for one
// ownership.
func (s *Service) HistoryForOwnership(ctx context.Context, o *model.ClientOwnership, from, to time.Time, bucket time.Duration) ([]BucketPoint, error) {
	rows, err := s.samples.ChronologicalForClient(ctx, o.NodeID, o.InboundTag, o.ClientEmail, from, to)
	if err != nil {
		return nil, err
	}
	return BucketDeltas(rows, bucket.Nanoseconds()), nil
}

// ---- Resets ---------------------------------------------------------------

// ResetClient zeroes one client's counters on the panel.
func (s *Service) ResetClient(ctx context.Context, nodeID int64, inboundTag, email string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	return r.ResetClientTraffic(ctx, inboundTag, email)
}

// ResetInbound zeroes one inbound's counters and every client on it.
func (s *Service) ResetInbound(ctx context.Context, nodeID int64, inboundTag string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := r.ResetInboundTraffic(ctx, inboundTag); err != nil {
		return err
	}
	return r.ResetAllClientTraffics(ctx, inboundTag)
}

// ResetNode zeroes every counter on a node.
func (s *Service) ResetNode(ctx context.Context, nodeID int64) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	return r.ResetAllTraffics(ctx)
}

// ---- Shared-quota aggregation ---------------------------------------------

// SharedQuotaStats summarises what one enforcement pass did. Used by
// tests and by the job's structured log so operators can spot
// "everyone got disabled today" anomalies.
type SharedQuotaStats struct {
	GroupsExamined int
	GroupsOver     int
	OwnersDisabled int
	OwnersRestored int
	Errors         []string
}

// EnforceSharedQuotas walks every (user_id, plan_id) group whose plan
// has a provisioning_pool_id set (i.e. fan-out clients), sums each
// group's used bytes across all member ownerships, and either:
//
//   - disables every still-enabled member (and stamps disabled_by_quota
//     so the next pass knows it was a quota disable, not an admin one)
//     when the group is over plan.traffic_limit_bytes, or
//   - re-enables members previously disabled_by_quota when the group
//     has dropped back under the limit (panel-side counters were
//     reset by an inbound's own traffic_reset cycle, by a renewal,
//     or by admin reset).
//
// No-op when shared-quota deps were never wired (SetSharedQuotaDeps
// was not called). The traffic job calls this once after every
// CollectAll cycle.
func (s *Service) EnforceSharedQuotas(ctx context.Context, now time.Time) (SharedQuotaStats, error) {
	var stats SharedQuotaStats
	if s.plans == nil || s.clients == nil || s.ownership == nil {
		return stats, nil
	}
	candidates, err := s.ownership.ListSharedQuotaCandidates(ctx)
	if err != nil {
		return stats, fmt.Errorf("EnforceSharedQuotas: list candidates: %w", err)
	}
	// Group by (user_id, plan_id). plan_id is guaranteed non-nil on
	// the rows returned by ListSharedQuotaCandidates.
	type groupKey struct {
		UserID int64
		PlanID int64
	}
	groups := make(map[groupKey][]model.ClientOwnership)
	for _, o := range candidates {
		if o.PlanID == nil {
			continue
		}
		k := groupKey{UserID: o.UserID, PlanID: *o.PlanID}
		groups[k] = append(groups[k], o)
	}
	for k, owners := range groups {
		stats.GroupsExamined++
		plan, err := s.plans.Get(ctx, k.PlanID)
		if err != nil || plan == nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("plan %d lookup: %v", k.PlanID, err))
			continue
		}
		if plan.TrafficLimitBytes <= 0 {
			// Unlimited plan; nothing to enforce.
			continue
		}
		used := s.aggregateGroupUsage(ctx, owners, now)
		over := used >= plan.TrafficLimitBytes
		if over {
			stats.GroupsOver++
			for _, o := range owners {
				if !o.Enabled || o.DisabledByQuota {
					continue
				}
				if err := s.clients.SetClientEnabled(ctx, o.NodeID, o.InboundTag, o.ClientEmail, false); err != nil {
					stats.Errors = append(stats.Errors, fmt.Sprintf("panel disable owner=%d: %v", o.ID, err))
					continue
				}
				if err := s.ownership.MarkDisabledByQuota(ctx, o.ID); err != nil {
					stats.Errors = append(stats.Errors, fmt.Sprintf("db disable owner=%d: %v", o.ID, err))
					continue
				}
				stats.OwnersDisabled++
			}
			s.log.Info("shared quota disabled",
				slog.Int64("user_id", k.UserID),
				slog.Int64("plan_id", k.PlanID),
				slog.Int64("used", used),
				slog.Int64("limit", plan.TrafficLimitBytes),
			)
			continue
		}
		// Under limit: restore any that were disabled by previous
		// enforcement.
		for _, o := range owners {
			if !o.DisabledByQuota {
				continue
			}
			if err := s.clients.SetClientEnabled(ctx, o.NodeID, o.InboundTag, o.ClientEmail, true); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("panel restore owner=%d: %v", o.ID, err))
				continue
			}
			if err := s.ownership.RestoreFromQuota(ctx, o.ID); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("db restore owner=%d: %v", o.ID, err))
				continue
			}
			stats.OwnersRestored++
		}
	}
	return stats, nil
}

// aggregateGroupUsage sums up+down across every member of a shared
// quota group. Uses UsageForOwnership which is counter-reset safe.
// Window starts from the earliest ownership's UpdatedAt minus a small
// buffer (lets the first post-renewal sample land) and ends at now.
func (s *Service) aggregateGroupUsage(ctx context.Context, owners []model.ClientOwnership, now time.Time) int64 {
	var total int64
	for i := range owners {
		o := owners[i]
		from := o.UpdatedAt.Add(-1 * time.Hour)
		usage, err := s.UsageForOwnership(ctx, &o, from, now)
		if err != nil {
			s.log.Warn("usage compute failed in shared quota agg",
				slog.Int64("ownership_id", o.ID),
				slog.String("err", err.Error()),
			)
			continue
		}
		total += usage.TotalBytes
	}
	return total
}

// ---- helpers --------------------------------------------------------------

func strPtr(s string) *string { return &s }

// Type aliases for the canonical payloads in
// internal/service/event/payload. Subscribers import those directly.
type ClientThresholdPayload = payload.ClientThreshold
type ClientExpiredPayload = payload.TrafficClientExpired

// Helpful sentinel re-exports so handlers don't import runtime
// directly for error checks.
var (
	ErrNodeNotFound = runtime.ErrNodeNotFound
	ErrTagNotFound  = runtime.ErrTagNotFound
)
