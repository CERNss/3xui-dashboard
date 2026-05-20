package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// EmailOutboxRepo owns the email_outbox persistence layer. The
// worker (job.EmailQueueJob) uses ClaimBatch to atomically pick
// up pending rows; producers use Enqueue. Workers + producers
// run concurrently — SELECT FOR UPDATE SKIP LOCKED keeps them
// non-blocking.
type EmailOutboxRepo struct{ db *gorm.DB }

// NewEmailOutboxRepo returns a repo bound to db.
func NewEmailOutboxRepo(db *gorm.DB) *EmailOutboxRepo {
	return &EmailOutboxRepo{db: db}
}

// Enqueue inserts a new pending row. NextAttemptAt defaults to
// now() so the worker picks it up on the next tick.
func (r *EmailOutboxRepo) Enqueue(ctx context.Context, to, subject, body string) error {
	row := &model.EmailOutbox{
		ToAddr:        to,
		Subject:       subject,
		Body:          body,
		Status:        model.EmailOutboxStatusPending,
		NextAttemptAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).
		Select("ToAddr", "Subject", "Body", "Status", "Attempts", "LastError", "NextAttemptAt").
		Create(row).Error; err != nil {
		return fmt.Errorf("EmailOutboxRepo.Enqueue: %w", err)
	}
	return nil
}

// ClaimBatch atomically picks up to `limit` pending rows whose
// next_attempt_at has passed and flips their status to 'sending'.
// Uses SELECT FOR UPDATE SKIP LOCKED so two parallel workers don't
// claim the same row.
//
// Returns the claimed rows. The caller MUST eventually call
// MarkSent / MarkRetry / MarkFailed on each — leaving rows stuck
// in 'sending' would block the queue. The worker's retry path
// uses MarkRetry on transient errors so failure recovery is
// automatic.
func (r *EmailOutboxRepo) ClaimBatch(ctx context.Context, limit int) ([]model.EmailOutbox, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []model.EmailOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hold a SELECT FOR UPDATE SKIP LOCKED over the candidates,
		// then UPDATE their status under the same tx so other
		// workers see them as 'sending' on their next scan.
		if err := tx.
			Raw(`
				SELECT * FROM email_outbox
				WHERE status = ? AND next_attempt_at <= ?
				ORDER BY next_attempt_at
				LIMIT ?
				FOR UPDATE SKIP LOCKED`,
				model.EmailOutboxStatusPending, time.Now().UTC(), limit).
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("select pending: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		if err := tx.Model(&model.EmailOutbox{}).
			Where("id IN ?", ids).
			Update("status", model.EmailOutboxStatusSending).Error; err != nil {
			return fmt.Errorf("mark sending: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("EmailOutboxRepo.ClaimBatch: %w", err)
	}
	return rows, nil
}

// MarkSent stamps a row as terminally delivered.
func (r *EmailOutboxRepo) MarkSent(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":  model.EmailOutboxStatusSent,
			"sent_at": now,
		}).Error; err != nil {
		return fmt.Errorf("EmailOutboxRepo.MarkSent: %w", err)
	}
	return nil
}

// MarkRetry bumps attempts, records the last error, and schedules
// the next retry. The caller computes the backoff so policy lives
// in one place (the worker job).
func (r *EmailOutboxRepo) MarkRetry(ctx context.Context, id int64, attempts int, lastErr string, nextAttemptAt time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          model.EmailOutboxStatusPending,
			"attempts":        attempts,
			"last_error":      lastErr,
			"next_attempt_at": nextAttemptAt,
		}).Error; err != nil {
		return fmt.Errorf("EmailOutboxRepo.MarkRetry: %w", err)
	}
	return nil
}

// MarkFailed flips the row to terminal failure. Used by the worker
// when attempts >= max.
func (r *EmailOutboxRepo) MarkFailed(ctx context.Context, id int64, lastErr string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.EmailOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     model.EmailOutboxStatusFailed,
			"last_error": lastErr,
		}).Error; err != nil {
		return fmt.Errorf("EmailOutboxRepo.MarkFailed: %w", err)
	}
	return nil
}
