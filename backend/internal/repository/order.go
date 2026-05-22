package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// OrderRepo persists Order rows. Purchase idempotency uses
// orders.idempotency_key — the migration enforces the unique
// constraint so concurrent dupes fail at the DB layer.
type OrderRepo struct{ db *gorm.DB }

// NewOrderRepo returns a repository bound to db.
func NewOrderRepo(db *gorm.DB) *OrderRepo { return &OrderRepo{db: db} }

// DB exposes the underlying handle for service-level transactions
// that need to compose order writes with another repository.
func (r *OrderRepo) DB() *gorm.DB { return r.db }

// GetByIdempotencyKey returns the order matching key, or (nil, nil)
// on miss. Used by Purchase to short-circuit dupe attempts.
func (r *OrderRepo) GetByIdempotencyKey(ctx context.Context, key string) (*model.Order, error) {
	if key == "" {
		return nil, nil
	}
	var o model.Order
	if err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("OrderRepo.GetByIdempotencyKey: %w", err)
	}
	return &o, nil
}

// GetByUserAndIdempotencyKey returns the order matching user+key, or
// (nil, nil) on miss. Callers that expose idempotency replays to end
// users should prefer this scoped lookup before falling back to the
// global key for conflict detection.
func (r *OrderRepo) GetByUserAndIdempotencyKey(ctx context.Context, userID int64, key string) (*model.Order, error) {
	if key == "" {
		return nil, nil
	}
	var o model.Order
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND idempotency_key = ?", userID, key).
		First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("OrderRepo.GetByUserAndIdempotencyKey: %w", err)
	}
	return &o, nil
}

// Create persists a new order row.
func (r *OrderRepo) Create(ctx context.Context, o *model.Order) error {
	if err := r.db.WithContext(ctx).Create(o).Error; err != nil {
		return fmt.Errorf("OrderRepo.Create: %w", err)
	}
	return nil
}

// MarkCompleted stamps the order completed + writes the ownership id.
func (r *OrderRepo) MarkCompleted(ctx context.Context, id, ownershipID int64) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":              model.OrderStatusCompleted,
			"client_ownership_id": ownershipID,
			"completed_at":        now,
		})
	if res.Error != nil {
		return fmt.Errorf("OrderRepo.MarkCompleted: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkFailed records the failure reason. Idempotent.
func (r *OrderRepo) MarkFailed(ctx context.Context, id int64, msg string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        model.OrderStatusFailed,
			"error_message": msg,
		})
	if res.Error != nil {
		return fmt.Errorf("OrderRepo.MarkFailed: %w", res.Error)
	}
	return nil
}

// MarkRefunded stamps the order refunded (used by Purchase when a
// provisioning failure rolls back the balance charge).
func (r *OrderRepo) MarkRefunded(ctx context.Context, id int64, msg string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        model.OrderStatusRefunded,
			"error_message": msg,
		})
	if res.Error != nil {
		return fmt.Errorf("OrderRepo.MarkRefunded: %w", res.Error)
	}
	return nil
}

// GetByProviderOrderID returns the order whose payment_provider_order_id
// matches `pid`. Used by the notify endpoint + payment-poll job to
// look up an order from the gateway's identifier. Empty pid SHALL
// return (nil, nil) — never match the default-empty column.
func (r *OrderRepo) GetByProviderOrderID(ctx context.Context, pid string) (*model.Order, error) {
	if pid == "" {
		return nil, nil
	}
	var o model.Order
	if err := r.db.WithContext(ctx).
		Where("payment_provider_order_id = ?", pid).
		First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("OrderRepo.GetByProviderOrderID: %w", err)
	}
	return &o, nil
}

// Get returns the order by id. Returns (nil, nil) on miss.
func (r *OrderRepo) Get(ctx context.Context, id int64) (*model.Order, error) {
	var o model.Order
	if err := r.db.WithContext(ctx).First(&o, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("OrderRepo.Get: %w", err)
	}
	return &o, nil
}

// SetPaymentMetadata persists the redirect/QR target URL + provider
// order id + expires_at on a freshly-created payment_pending order.
// Runs once right after the gateway's CreatePayment succeeds.
func (r *OrderRepo) SetPaymentMetadata(ctx context.Context, id int64, providerOrderID, targetURL string, expiresAt time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"payment_provider_order_id": providerOrderID,
			"payment_target_url":        targetURL,
			"payment_expires_at":        expiresAt,
		})
	if res.Error != nil {
		return fmt.Errorf("OrderRepo.SetPaymentMetadata: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AdvanceStatusGuarded transitions an order from `from` to `to`,
// returning gorm.ErrRecordNotFound (treated as "already advanced
// by someone else") if the status isn't currently `from`. This is
// the lock we use to make notify + poll-job idempotent: two
// confirmations race, only one wins the transition.
func (r *OrderRepo) AdvanceStatusGuarded(ctx context.Context, id int64, from, to string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ? AND status = ?", id, from).
		Update("status", to)
	if res.Error != nil {
		return fmt.Errorf("OrderRepo.AdvanceStatusGuarded: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListPaymentPending returns orders in payment_pending status,
// optionally filtered by max age. Used by the payment-poll job.
// maxAge <= 0 returns all pending orders.
func (r *OrderRepo) ListPaymentPending(ctx context.Context, maxAge time.Duration) ([]model.Order, error) {
	var rows []model.Order
	q := r.db.WithContext(ctx).Where("status = ?", model.OrderStatusPaymentPending)
	if maxAge > 0 {
		now := time.Now().UTC()
		cutoff := now.Add(-maxAge)
		q = q.Where("(payment_expires_at IS NOT NULL AND payment_expires_at > ?) OR (payment_expires_at IS NULL AND created_at > ?)", now, cutoff)
	}
	if err := q.Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("OrderRepo.ListPaymentPending: %w", err)
	}
	return rows, nil
}

// ListExpiredPending returns payment_pending orders whose gateway
// expiry timestamp is past now. Rows without payment_expires_at
// fall back to created_at so older data still gets reaped.
func (r *OrderRepo) ListExpiredPending(ctx context.Context, now, fallbackCutoff time.Time) ([]model.Order, error) {
	var rows []model.Order
	err := r.db.WithContext(ctx).
		Where("status = ? AND ((payment_expires_at IS NOT NULL AND payment_expires_at <= ?) OR (payment_expires_at IS NULL AND created_at <= ?))",
			model.OrderStatusPaymentPending, now, fallbackCutoff).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("OrderRepo.ListExpiredPending: %w", err)
	}
	return rows, nil
}

// ListByUser returns the user's order history (newest first).
func (r *OrderRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Order, error) {
	var rows []model.Order
	q := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("OrderRepo.ListByUser: %w", err)
	}
	return rows, nil
}

// UserHasAccessHistory reports whether the user has any order that
// represents paid/usable access. It deliberately ignores failed,
// refunded, and merely-pending balance orders so starter-plan rules
// continue to apply until the user actually enters a paid flow.
func (r *OrderRepo) UserHasAccessHistory(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("user_id = ?", userID).
		Where("status IN ?", []string{
			model.OrderStatusCompleted,
			model.OrderStatusPaid,
			model.OrderStatusPaymentPending,
		}).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("OrderRepo.UserHasAccessHistory: %w", err)
	}
	return count > 0, nil
}

// ListAdmin returns paged orders with optional filters.
type OrderFilter struct {
	UserID *int64
	Status *string
}

func (r *OrderRepo) ListAdmin(ctx context.Context, f OrderFilter, limit, offset int) ([]model.Order, error) {
	var rows []model.Order
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Status != nil && *f.Status != "" {
		q = q.Where("status = ?", *f.Status)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("OrderRepo.ListAdmin: %w", err)
	}
	return rows, nil
}
