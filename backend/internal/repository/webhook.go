package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// WebhookRepo persists webhook configurations.
type WebhookRepo struct{ db *gorm.DB }

func NewWebhookRepo(db *gorm.DB) *WebhookRepo { return &WebhookRepo{db: db} }

func (r *WebhookRepo) List(ctx context.Context) ([]model.Webhook, error) {
	var rows []model.Webhook
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("WebhookRepo.List: %w", err)
	}
	return rows, nil
}

func (r *WebhookRepo) ListEnabled(ctx context.Context) ([]model.Webhook, error) {
	var rows []model.Webhook
	if err := r.db.WithContext(ctx).Where("enabled = TRUE").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("WebhookRepo.ListEnabled: %w", err)
	}
	return rows, nil
}

func (r *WebhookRepo) Get(ctx context.Context, id int64) (*model.Webhook, error) {
	var w model.Webhook
	if err := r.db.WithContext(ctx).First(&w, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("WebhookRepo.Get: %w", err)
	}
	return &w, nil
}

func (r *WebhookRepo) Create(ctx context.Context, w *model.Webhook) error {
	// Select("*") so zero-value bool Enabled lands as false (gorm
	// otherwise lets the column default override it). Same fix as
	// PlanRepo.Create.
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(w).Error; err != nil {
		return fmt.Errorf("WebhookRepo.Create: %w", err)
	}
	return nil
}

func (r *WebhookRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&model.Webhook{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("WebhookRepo.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *WebhookRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.Webhook{}, id)
	if res.Error != nil {
		return fmt.Errorf("WebhookRepo.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---- Deliveries -----------------------------------------------------------

type WebhookDeliveryRepo struct{ db *gorm.DB }

func NewWebhookDeliveryRepo(db *gorm.DB) *WebhookDeliveryRepo { return &WebhookDeliveryRepo{db: db} }

func (r *WebhookDeliveryRepo) Create(ctx context.Context, d *model.WebhookDelivery) error {
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return fmt.Errorf("WebhookDeliveryRepo.Create: %w", err)
	}
	return nil
}

func (r *WebhookDeliveryRepo) MarkSuccess(ctx context.Context, id int64, attempt, httpStatus int, body string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&model.WebhookDelivery{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":          model.WebhookDeliveryStatusSuccess,
			"attempt":         attempt,
			"http_status":     httpStatus,
			"response_body":   truncate(body, 4096),
			"delivered_at":    now,
			"next_attempt_at": now,
			"error":           "",
		}).Error
}

// ScheduleRetry keeps the row in "pending" and pushes next_attempt_at
// out to t. The cron retry job picks it up when due.
func (r *WebhookDeliveryRepo) ScheduleRetry(ctx context.Context, id int64, attempt, httpStatus int, errMsg, body string, next time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.WebhookDelivery{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":          model.WebhookDeliveryStatusPending,
			"attempt":         attempt,
			"http_status":     httpStatus,
			"response_body":   truncate(body, 4096),
			"error":           truncate(errMsg, 512),
			"next_attempt_at": next,
		}).Error
}

// MarkTerminallyFailed is called when attempt count reaches the
// configured max — no more retries will be scheduled.
func (r *WebhookDeliveryRepo) MarkTerminallyFailed(ctx context.Context, id int64, attempt, httpStatus int, errMsg, body string) error {
	return r.db.WithContext(ctx).
		Model(&model.WebhookDelivery{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":        model.WebhookDeliveryStatusFailed,
			"attempt":       attempt,
			"http_status":   httpStatus,
			"response_body": truncate(body, 4096),
			"error":         truncate(errMsg, 512),
		}).Error
}

// ClaimDue returns up to limit pending+due delivery rows for the
// retry job. The query uses SELECT ... FOR UPDATE SKIP LOCKED so
// multiple workers (or restarted instances) can fan out across rows
// without stepping on each other.
//
// Caller MUST wrap this in a transaction and process the returned
// rows before the transaction closes — the lock is released on
// commit/rollback. We keep the transaction inline here by returning
// the rows + a finalizer; for v1 we accept that the caller commits
// quickly (dispatcher fires off goroutines and returns).
func (r *WebhookDeliveryRepo) ClaimDue(ctx context.Context, limit int) ([]model.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 32
	}
	var rows []model.WebhookDelivery
	err := r.db.WithContext(ctx).Raw(`
        SELECT *
          FROM webhook_deliveries
         WHERE status = 'pending'
           AND next_attempt_at <= now()
         ORDER BY next_attempt_at
         LIMIT ?
        FOR UPDATE SKIP LOCKED
    `, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("WebhookDeliveryRepo.ClaimDue: %w", err)
	}
	return rows, nil
}

func (r *WebhookDeliveryRepo) ListByWebhook(ctx context.Context, webhookID int64, limit, offset int) ([]model.WebhookDelivery, error) {
	var rows []model.WebhookDelivery
	q := r.db.WithContext(ctx).
		Where("webhook_id = ?", webhookID).
		Order("scheduled_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("WebhookDeliveryRepo.ListByWebhook: %w", err)
	}
	return rows, nil
}

func (r *WebhookDeliveryRepo) Get(ctx context.Context, id int64) (*model.WebhookDelivery, error) {
	var d model.WebhookDelivery
	if err := r.db.WithContext(ctx).First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("WebhookDeliveryRepo.Get: %w", err)
	}
	return &d, nil
}

// MarshalDeliveryPayload is exposed so service code doesn't have to
// double-import encoding/json + know about JSON envelope details.
func MarshalDeliveryPayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
