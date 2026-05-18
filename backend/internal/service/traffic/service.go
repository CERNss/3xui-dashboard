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
	"github.com/cern/3xui-dashboard/internal/service/event"
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
	nodes     NodeListSource
	bus       *event.Bus
	log       *slog.Logger

	dedup sync.Map // key string → last emit time.Time

	// Concurrency cap for fleet walks. Zero = default (8).
	FleetConcurrency int
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

// CollectAll fans out across every enabled node, fetches a traffic
// snapshot per node, and persists rows in one batch insert per node.
// Per-node failures never abort the pass; they're logged and the
// per-node error is collected into the returned map.
func (s *Service) CollectAll(ctx context.Context, now time.Time) (map[int64]string, error) {
	nodes, err := s.nodes.ListEnabledNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("traffic.CollectAll: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	conc := s.FleetConcurrency
	if conc <= 0 {
		conc = 8
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(conc)

	var (
		mu     sync.Mutex
		errMap = map[int64]string{}
	)

	for i := range nodes {
		n := nodes[i]
		g.Go(func() error {
			if err := s.collectOne(gctx, n, now); err != nil {
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

func (s *Service) collectOne(ctx context.Context, n NodeRef, now time.Time) error {
	r, err := s.rt.Get(ctx, n.ID)
	if err != nil {
		return err
	}
	snap, err := r.FetchTrafficSnapshot(ctx)
	if err != nil {
		return err
	}
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
	NodeID       int64     `json:"node_id"`
	InboundTag   string    `json:"inbound_tag"`
	ClientEmail  string    `json:"client_email"`
	Up           int64     `json:"up"`
	Down         int64     `json:"down"`
	TotalBytes   int64     `json:"total"`
	LimitBytes   *int64    `json:"limit,omitempty"`
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

// HistoryForInbound returns time-bucketed usage points for an
// inbound (sum over all clients).
func (s *Service) HistoryForInbound(ctx context.Context, nodeID int64, inboundTag string, from, to time.Time, bucket time.Duration) ([]BucketPoint, error) {
	rows, err := s.samples.ChronologicalForInbound(ctx, nodeID, inboundTag, from, to)
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

// ---- helpers --------------------------------------------------------------

func strPtr(s string) *string { return &s }

// PublishClientPayloads — kept exported so subscribers can typecast.
type ClientThresholdPayload struct {
	NodeID      int64  `json:"node_id"`
	NodeName    string `json:"node_name"`
	InboundTag  string `json:"inbound_tag"`
	ClientEmail string `json:"client_email"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
	Limit       int64  `json:"limit"`
}

type ClientExpiredPayload struct {
	NodeID      int64     `json:"node_id"`
	NodeName    string    `json:"node_name"`
	InboundTag  string    `json:"inbound_tag"`
	ClientEmail string    `json:"client_email"`
	ExpiredAt   time.Time `json:"expired_at"`
}

// Helpful sentinel re-exports so handlers don't import runtime
// directly for error checks.
var (
	ErrNodeNotFound = runtime.ErrNodeNotFound
	ErrTagNotFound  = runtime.ErrTagNotFound
)
