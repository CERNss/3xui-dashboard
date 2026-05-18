package job

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler is a thin wrapper around robfig/cron that captures the
// jobs registered with it and lets the caller invoke RunOnce on a
// single-shot context (handy for tests + smoke runs).
type Scheduler struct {
	cron *cron.Cron
	log  *slog.Logger

	mu   sync.Mutex
	once map[string]func(context.Context)
}

// NewScheduler returns a scheduler with minute-resolution parsing
// (the cron/v3 default).
func NewScheduler(lg *slog.Logger) *Scheduler {
	c := cron.New(
		cron.WithLogger(cron.PrintfLogger(slogPrintf{lg: lg.With(slog.String("component", "cron"))})),
	)
	return &Scheduler{cron: c, log: lg, once: make(map[string]func(context.Context))}
}

// Add registers fn to run on the given cron spec or @every duration
// (e.g. "@every 30s"). id lets RunOnce trigger fn outside the cron
// loop.
func (s *Scheduler) Add(id, spec string, fn func(context.Context)) error {
	s.mu.Lock()
	s.once[id] = fn
	s.mu.Unlock()
	_, err := s.cron.AddFunc(spec, func() { fn(context.Background()) })
	return err
}

// Start spins up the cron loop.
func (s *Scheduler) Start() { s.cron.Start() }

// Stop returns a context that is Done when every in-flight job has
// finished. Callers should wait on it during shutdown.
func (s *Scheduler) Stop() context.Context { return s.cron.Stop() }

// RunOnce invokes the named job synchronously — useful from tests
// and from on-demand admin endpoints.
func (s *Scheduler) RunOnce(ctx context.Context, id string) {
	s.mu.Lock()
	fn, ok := s.once[id]
	s.mu.Unlock()
	if !ok {
		return
	}
	fn(ctx)
}

type slogPrintf struct{ lg *slog.Logger }

func (s slogPrintf) Printf(format string, v ...any) {
	s.lg.Info("cron", slog.String("msg", fmt.Sprintf(format, v...)))
}
