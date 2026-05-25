package job

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/datacollection"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
)

// TrafficJob runs traffic.Service.CollectAll on a cron schedule.
// Default cadence is 60 s; the scheduler wiring in cmd/dashboard
// picks the spec.
type TrafficJob struct {
	svc    *traffic.Service
	config *datacollection.ConfigService
	log    *slog.Logger

	mu      sync.Mutex
	lastRun time.Time
}

// NewTrafficJob builds a job.
func NewTrafficJob(svc *traffic.Service, lg *slog.Logger) *TrafficJob {
	return &TrafficJob{
		svc: svc,
		log: lg.With(slog.String("component", "job.traffic")),
	}
}

// SetConfig wires runtime data-collection settings. Nil keeps defaults.
func (j *TrafficJob) SetConfig(config *datacollection.ConfigService) {
	j.config = config
}

// RunOnce kicks off one collection pass.
func (j *TrafficJob) RunOnce(ctx context.Context) {
	now := time.Now().UTC()
	cfg := datacollection.CollectorConfig{
		Enabled:   true,
		Interval:  datacollection.DefaultTrafficInterval,
		Retention: datacollection.DefaultTrafficRetention,
	}
	if j.config != nil {
		cfg = j.config.Traffic(ctx)
	}
	if !cfg.Enabled {
		return
	}
	if !j.shouldRun(now, cfg.Interval) {
		return
	}

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
	if cfg.Retention > 0 {
		deleted, err := j.svc.DeleteSamplesOlderThan(ctx, now.Add(-cfg.Retention))
		if err != nil {
			j.log.Warn("traffic sample cleanup failed", slog.String("error", err.Error()))
		} else if deleted > 0 {
			j.log.Info("traffic sample cleanup", slog.Int64("deleted", deleted))
		}
	}
}

func (j *TrafficJob) shouldRun(now time.Time, interval time.Duration) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if interval <= 0 {
		interval = datacollection.DefaultTrafficInterval
	}
	if !j.lastRun.IsZero() && now.Sub(j.lastRun) < interval {
		return false
	}
	j.lastRun = now
	return true
}
