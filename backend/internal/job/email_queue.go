package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
)

// EmailQueueJob drains the email_outbox table. Runs every 30s by
// default (caller decides the cron expression at scheduler.Add).
// Each tick claims a batch of pending rows, attempts SMTP delivery,
// and either marks them sent or schedules a backed-off retry.
//
// Backoff schedule (exponential with a soft cap):
//
//	attempt 1 → next try in  1m
//	attempt 2 → next try in  2m
//	attempt 3 → next try in  5m
//	attempt 4 → next try in 15m
//	attempt 5 → next try in  1h
//	attempt 6 → terminal failure
//
// This gives ~1h25m of buffer for a flaky SMTP provider before
// the dashboard gives up. Verification-code emails expire in 10
// minutes, so an SMTP outage longer than that just produces
// undeliverable codes — the user requests another one.
type EmailQueueJob struct {
	outbox    *repository.EmailOutboxRepo
	mailer    *mailer.Mailer
	log       *slog.Logger
	batchSize int
}

// NewEmailQueueJob constructs the worker. batchSize <= 0 picks the
// default of 20 — enough to amortize the SELECT FOR UPDATE round-
// trip without holding row locks for too long.
func NewEmailQueueJob(outbox *repository.EmailOutboxRepo, m *mailer.Mailer, lg *slog.Logger, batchSize int) *EmailQueueJob {
	if batchSize <= 0 {
		batchSize = 20
	}
	return &EmailQueueJob{
		outbox:    outbox,
		mailer:    m,
		log:       lg.With(slog.String("component", "job.email_queue")),
		batchSize: batchSize,
	}
}

// RunOnce processes one batch. Safe to call concurrently from
// multiple workers — the underlying ClaimBatch uses SKIP LOCKED.
func (j *EmailQueueJob) RunOnce(ctx context.Context) {
	rows, err := j.outbox.ClaimBatch(ctx, j.batchSize)
	if err != nil {
		j.log.Error("claim batch failed", slog.String("err", err.Error()))
		return
	}
	for _, row := range rows {
		j.process(ctx, &row)
	}
}

func (j *EmailQueueJob) process(ctx context.Context, row *model.EmailOutbox) {
	attempts := row.Attempts + 1
	err := j.mailer.Send(row.ToAddr, row.Subject, row.Body)
	if err == nil {
		if mErr := j.outbox.MarkSent(ctx, row.ID); mErr != nil {
			j.log.Error("mark sent failed",
				slog.Int64("id", row.ID), slog.String("err", mErr.Error()))
		}
		j.log.Info("email sent",
			slog.Int64("id", row.ID),
			slog.String("to", row.ToAddr),
			slog.Int("attempts", attempts),
		)
		return
	}

	if attempts >= maxEmailAttempts {
		if mErr := j.outbox.MarkFailed(ctx, row.ID, err.Error()); mErr != nil {
			j.log.Error("mark failed errored",
				slog.Int64("id", row.ID), slog.String("err", mErr.Error()))
		}
		j.log.Error("email permanently failed",
			slog.Int64("id", row.ID),
			slog.String("to", row.ToAddr),
			slog.Int("attempts", attempts),
			slog.String("err", err.Error()),
		)
		return
	}

	next := time.Now().UTC().Add(backoffFor(attempts))
	if mErr := j.outbox.MarkRetry(ctx, row.ID, attempts, err.Error(), next); mErr != nil {
		j.log.Error("mark retry failed",
			slog.Int64("id", row.ID), slog.String("err", mErr.Error()))
	}
	j.log.Warn("email send transient failure, will retry",
		slog.Int64("id", row.ID),
		slog.String("to", row.ToAddr),
		slog.Int("attempts", attempts),
		slog.Duration("next_in", backoffFor(attempts)),
		slog.String("err", err.Error()),
	)
}

const maxEmailAttempts = 6

// backoffFor returns how long to wait after the N-th failed
// attempt before retrying. Constants kept inline so the policy
// is grep-able from job logs.
func backoffFor(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Minute
	case 2:
		return 2 * time.Minute
	case 3:
		return 5 * time.Minute
	case 4:
		return 15 * time.Minute
	case 5:
		return 1 * time.Hour
	default:
		return 1 * time.Hour
	}
}
