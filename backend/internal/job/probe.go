// Package job hosts the periodic background workers (probe, traffic
// collection, webhook dispatch). All jobs are scheduled via a single
// robfig/cron instance and shut down with the rest of the server.
package job

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/node"
)

// ProbeJob walks every enabled node every 30 seconds and updates the
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
	bus         *event.Bus
	log         *slog.Logger
	concurrency int
	perCall     time.Duration
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

// RunOnce executes a single pass. Suitable both for cron-scheduled
// invocation and for tests.
func (j *ProbeJob) RunOnce(ctx context.Context) {
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

func (j *ProbeJob) probeOne(ctx context.Context, n model.Node) {
	res, err := j.nodes.Probe(ctx, n.ID)
	if err != nil {
		j.log.Warn("probe failed",
			slog.Int64("node_id", n.ID),
			slog.String("node", n.Name),
			slog.String("error", err.Error()),
		)
		j.bus.PublishType(event.NodeProbeFailed, NodeProbeFailedPayload{
			NodeID: n.ID, Name: n.Name, Error: err.Error(),
		})
		if n.Status == model.NodeStatusOnline {
			j.bus.PublishType(event.NodeOffline, NodeStatusChangedPayload{
				NodeID: n.ID, Name: n.Name, Prior: n.Status, Now: model.NodeStatusOffline,
			})
		}
		return
	}
	// Probe succeeded.
	if n.Status != model.NodeStatusOnline {
		j.bus.PublishType(event.NodeOnline, NodeStatusChangedPayload{
			NodeID: n.ID, Name: n.Name, Prior: n.Status, Now: model.NodeStatusOnline,
		})
	}
	_ = res
}

// Payload shapes published on the bus. Subscribers decode by event
// type → struct.
type NodeStatusChangedPayload struct {
	NodeID int64  `json:"node_id"`
	Name   string `json:"name"`
	Prior  string `json:"prior_status"`
	Now    string `json:"new_status"`
}

type NodeProbeFailedPayload struct {
	NodeID int64  `json:"node_id"`
	Name   string `json:"name"`
	Error  string `json:"error"`
}
