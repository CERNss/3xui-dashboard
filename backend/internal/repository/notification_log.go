package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cern/3xui-dashboard/internal/model"
)

// NotificationLogRepo is the dedup gate for outbound user-facing
// messages and ops-facing notifications. Used by service/messages
// and service/notify to skip re-sending the same
// (surface, kind, ownership) tuple across cron ticks and restarts.
type NotificationLogRepo struct{ db *gorm.DB }

// NewNotificationLogRepo binds to db.
func NewNotificationLogRepo(db *gorm.DB) *NotificationLogRepo {
	return &NotificationLogRepo{db: db}
}

// AlreadySent reports whether a row exists for (surface, kind, ownershipID).
// Surface must be model.SurfaceMessage or model.SurfaceNotification —
// the DB-level CHECK constraint rejects anything else.
func (r *NotificationLogRepo) AlreadySent(ctx context.Context, surface, kind string, ownershipID int64) (bool, error) {
	var row model.NotificationLog
	err := r.db.WithContext(ctx).
		Where("surface = ? AND kind = ? AND ownership_id = ?", surface, kind, ownershipID).
		First(&row).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("NotificationLog.AlreadySent: %w", err)
}

// MarkSent inserts a row marking (surface, kind, ownership) as delivered.
// Idempotent via the unique index — duplicate inserts are silently
// swallowed so callers don't have to coordinate the "first writer
// wins" race.
func (r *NotificationLogRepo) MarkSent(ctx context.Context, surface, kind string, ownershipID int64, userEmail string) error {
	row := model.NotificationLog{
		Surface:     surface,
		Kind:        kind,
		OwnershipID: ownershipID,
		UserEmail:   userEmail,
	}
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "surface"}, {Name: "kind"}, {Name: "ownership_id"}},
			DoNothing: true,
		}).
		Create(&row)
	if res.Error != nil {
		return fmt.Errorf("NotificationLog.MarkSent: %w", res.Error)
	}
	return nil
}
