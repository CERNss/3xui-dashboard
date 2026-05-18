package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/traffic"
)

// TrafficJob runs traffic.Service.CollectAll on a cron schedule.
// Default cadence is 60 s; the scheduler wiring in cmd/dashboard
// picks the spec.
type TrafficJob struct {
	svc *traffic.Service
	log *slog.Logger
}

// NewTrafficJob builds a job.
func NewTrafficJob(svc *traffic.Service, lg *slog.Logger) *TrafficJob {
	return &TrafficJob{
		svc: svc,
		log: lg.With(slog.String("component", "job.traffic")),
	}
}

// RunOnce kicks off one collection pass.
func (j *TrafficJob) RunOnce(ctx context.Context) {
	now := time.Now().UTC()
	errs, err := j.svc.CollectAll(ctx, now)
	if err != nil {
		j.log.Error("CollectAll", slog.String("error", err.Error()))
		return
	}
	if len(errs) > 0 {
		j.log.Warn("traffic collection had per-node failures",
			slog.Int("failed_nodes", len(errs)),
		)
	}
}
