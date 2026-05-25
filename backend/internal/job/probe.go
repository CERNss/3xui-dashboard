// Package job hosts the periodic background workers (probe, traffic
// collection, webhook dispatch). All jobs are scheduled via a single
// robfig/cron instance and shut down with the rest of the server.
package job

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/datacollection"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
	"github.com/cern/3xui-dashboard/internal/service/node"
)

// ProbeJob walks every enabled node on the configured interval and updates the
// in-memory + persisted heartbeat. Status transitions emit
// `node.online` / `node.offline`; every failure additionally emits
// `node.probe_failed` so an alerting subscriber can see the reason
// even when the prior state was already offline.
//
// Probes per node run with a hard per-call timeout so one stuck node
// doesn't stall the whole pass. The pass itself runs with bounded
// concurrency.
type ProbeJob struct {
	nodes       *node.Service
	config      *datacollection.ConfigService
	bus         *event.Bus
	log         *slog.Logger
	concurrency int
	perCall     time.Duration

	mu      sync.Mutex
	lastRun time.Time
}

// NewProbeJob builds a job. concurrency=8 + perCall=12s are the
// defaults if the caller passes 0.
func NewProbeJob(nodes *node.Service, bus *event.Bus, lg *slog.Logger, concurrency int, perCall time.Duration) *ProbeJob {
	if concurrency <= 0 {
		concurrency = 8
	}
	if perCall <= 0 {
		perCall = 12 * time.Second
	}
	return &ProbeJob{
		nodes:       nodes,
		bus:         bus,
		log:         lg.With(slog.String("component", "job.probe")),
		concurrency: concurrency,
		perCall:     perCall,
	}
}

// SetConfig wires runtime data-collection settings. Nil keeps defaults.
func (j *ProbeJob) SetConfig(config *datacollection.ConfigService) {
	j.config = config
}

// RunOnce executes a single pass. Suitable both for cron-scheduled
// invocation and for tests.
func (j *ProbeJob) RunOnce(ctx context.Context) {
	cfg := datacollection.CollectorConfig{
		Enabled:   true,
		Interval:  datacollection.DefaultHealthInterval,
		Retention: datacollection.DefaultHealthRetention,
	}
	if j.config != nil {
		cfg = j.config.Health(ctx)
	}
	if !cfg.Enabled {
		return
	}
	if !j.shouldRun(time.Now().UTC(), cfg.Interval) {
		return
	}
	j.nodes.SetMetricsRetention(cfg.Retention)

	rows, err := j.nodes.ListEnabled(ctx)
	if err != nil {
		j.log.Error("list enabled nodes", slog.String("error", err.Error()))
		return
	}
	if len(rows) == 0 {
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(j.concurrency)

	for i := range rows {
		row := rows[i]
		g.Go(func() error {
			callCtx, cancel := context.WithTimeout(gctx, j.perCall)
			defer cancel()
			j.probeOne(callCtx, row)
			return nil
		})
	}
	_ = g.Wait()
}

func (j *ProbeJob) shouldRun(now time.Time, interval time.Duration) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if interval <= 0 {
		interval = datacollection.DefaultHealthInterval
	}
	if !j.lastRun.IsZero() && now.Sub(j.lastRun) < interval {
		return false
	}
	j.lastRun = now
	return true
}

func (j *ProbeJob) probeOne(ctx context.Context, n model.Node) {
	_, err := j.nodes.Probe(ctx, n.ID)
	if err != nil {
		j.log.Warn("probe failed",
			slog.Int64("node_id", n.ID),
			slog.String("node", n.Name),
			slog.String("error", err.Error()),
		)
	}
	for _, ev := range probeTransitionEvents(n.ID, n.Name, n.Status, err) {
		j.bus.PublishType(ev.Type, ev.Payload)
	}
}

// probeTransitionEvent is one (event_type, payload) pair to publish
// after a single probe call. Returned by probeTransitionEvents so
// the side-effect (Publish) is separable from the decision logic
// (what to publish based on prior status + probe outcome).
type probeTransitionEvent struct {
	Type    string
	Payload any
}

// probeTransitionEvents is the pure decision: given (nodeID, name,
// priorStatus, probeErr), what events should fire? Pulled out of
// probeOne so we can unit-test every transition matrix cell
// without standing up a node.Service.
//
// Truth table:
//
//	prior=online,    err=nil  → []                       (steady state)
//	prior=online,    err≠nil  → [probe_failed, offline]  (started failing)
//	prior=offline,   err=nil  → [online, recovered]      (recovered)
//	prior=offline,   err≠nil  → [probe_failed]           (still failing)
//	prior=unknown,   err=nil  → [online]                 (first probe ok)
//	prior=unknown,   err≠nil  → [probe_failed]           (first probe failed)
func probeTransitionEvents(nodeID int64, name, prior string, probeErr error) []probeTransitionEvent {
	if probeErr != nil {
		out := []probeTransitionEvent{{
			Type: event.NodeProbeFailed,
			Payload: NodeProbeFailedPayload{
				NodeID: nodeID, Name: name, Error: probeErr.Error(),
			},
		}}
		if prior == model.NodeStatusOnline {
			out = append(out, probeTransitionEvent{
				Type: event.NodeOffline,
				Payload: NodeStatusChangedPayload{
					NodeID: nodeID, Name: name, Prior: prior, Now: model.NodeStatusOffline,
				},
			})
		}
		return out
	}
	if prior == model.NodeStatusOnline {
		return nil
	}
	out := []probeTransitionEvent{{
		Type: event.NodeOnline,
		Payload: NodeStatusChangedPayload{
			NodeID: nodeID, Name: name, Prior: prior, Now: model.NodeStatusOnline,
		},
	}}
	if prior == model.NodeStatusOffline {
		// Genuine recovery, not first-probe — distinct event so ops
		// channels can subscribe to recoveries without boot chatter.
		out = append(out, probeTransitionEvent{
			Type: event.NodeRecovered,
			Payload: NodeStatusChangedPayload{
				NodeID: nodeID, Name: name, Prior: prior, Now: model.NodeStatusOnline,
			},
		})
	}
	return out
}

// Type aliases for the canonical payloads in
// internal/service/event/payload. Subscribers (notify, webhook)
// import those directly.
type NodeStatusChangedPayload = payload.NodeStatusChanged
type NodeProbeFailedPayload = payload.NodeProbeFailed
