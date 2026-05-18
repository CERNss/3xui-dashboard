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
