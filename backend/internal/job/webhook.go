package job

import (
	"context"
	"log/slog"

	"github.com/cern/3xui-dashboard/internal/service/webhook"
)

// WebhookRetryJob runs webhook.Service.RetryDue on a cron schedule.
// Default cadence is 15s (slightly faster than the typical first
// retry-backoff of 2s + jitter) so newly-scheduled retries don't sit
// idle for a full minute before the dispatcher notices.
type WebhookRetryJob struct {
	svc *webhook.Service
	log *slog.Logger

	batch int
}

// NewWebhookRetryJob builds a job. batch <= 0 picks the default of
// 32 deliveries per pass.
func NewWebhookRetryJob(svc *webhook.Service, batch int, lg *slog.Logger) *WebhookRetryJob {
	if batch <= 0 {
		batch = 32
	}
	return &WebhookRetryJob{
		svc:   svc,
		log:   lg.With(slog.String("component", "job.webhook-retry")),
		batch: batch,
	}
}

// RunOnce kicks off one scan of the pending+due queue.
func (j *WebhookRetryJob) RunOnce(ctx context.Context) {
	j.svc.RetryDue(ctx, j.batch)
}
