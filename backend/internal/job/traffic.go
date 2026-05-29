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
	running bool
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
		Enabled:       true,
		Interval:      datacollection.DefaultTrafficInterval,
		Retention:     datacollection.DefaultTrafficRetention,
		Concurrency:   datacollection.DefaultConcurrency,
		Timeout:       datacollection.DefaultTrafficTimeout,
		RetryAttempts: datacollection.DefaultRetryAttempts,
	}
	if j.config != nil {
		cfg = j.config.Traffic(ctx)
	}
	if !cfg.Enabled {
		return
	}
	if !j.startRun(now, cfg.Interval) {
		return
	}
	defer j.finishRun()

	errs, err := j.svc.CollectAllWithOptions(ctx, now, traffic.CollectOptions{
		Concurrency:    cfg.Concurrency,
		PerNodeTimeout: cfg.Timeout,
		RetryAttempts:  cfg.RetryAttempts,
	})
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

	// Shared-quota enforcement: after the snapshot pass updated each
	// client's cumulative counters, check every (user, plan) group
	// against plan.traffic_limit_bytes and either disable over-limit
	// groups or restore previously over-limit groups whose counters
	// have dropped (panel-side reset, renewal, admin reset).
	stats, err := j.svc.EnforceSharedQuotas(ctx, now)
	if err != nil {
		j.log.Warn("EnforceSharedQuotas", slog.String("error", err.Error()))
	} else if stats.OwnersDisabled > 0 || stats.OwnersRestored > 0 || len(stats.Errors) > 0 {
		j.log.Info("shared quota enforcement",
			slog.Int("groups_examined", stats.GroupsExamined),
			slog.Int("groups_over", stats.GroupsOver),
			slog.Int("owners_disabled", stats.OwnersDisabled),
			slog.Int("owners_restored", stats.OwnersRestored),
			slog.Int("errors", len(stats.Errors)),
		)
	}
}

func (j *TrafficJob) startRun(now time.Time, interval time.Duration) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if interval <= 0 {
		interval = datacollection.DefaultTrafficInterval
	}
	if j.running {
		j.log.Warn("traffic collection skipped; previous run still active")
		return false
	}
	if !j.lastRun.IsZero() && now.Sub(j.lastRun) < interval {
		return false
	}
	j.lastRun = now
	j.running = true
	return true
}

func (j *TrafficJob) finishRun() {
	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
}
