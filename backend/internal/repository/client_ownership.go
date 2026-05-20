package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cern/3xui-dashboard/internal/model"
)

// ClientOwnershipRepo is the persistence side of the user↔panel-
// client bridge. Lookups by (node_id, inbound_tag, client_email)
// drive the central subscription, ownership listings, and the
// provisioning service.
type ClientOwnershipRepo struct{ db *gorm.DB }

// NewClientOwnershipRepo returns a repository bound to db.
func NewClientOwnershipRepo(db *gorm.DB) *ClientOwnershipRepo {
	return &ClientOwnershipRepo{db: db}
}

// GetByTriple returns the ownership row for one panel-side client,
// or (nil, nil) on miss.
func (r *ClientOwnershipRepo) GetByTriple(ctx context.Context, nodeID int64, inboundTag, email string) (*model.ClientOwnership, error) {
	var row model.ClientOwnership
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND inbound_tag = ? AND client_email = ?", nodeID, inboundTag, email).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("ClientOwnership.GetByTriple: %w", err)
	}
	return &row, nil
}

// Upsert inserts a new row or updates the matching triple's fields
// in place. Returns the persisted row.
func (r *ClientOwnershipRepo) Upsert(ctx context.Context, row *model.ClientOwnership) (*model.ClientOwnership, error) {
	if row.NodeID == 0 || row.InboundTag == "" || row.ClientEmail == "" {
		return nil, errors.New("ClientOwnership.Upsert: node_id + inbound_tag + client_email are required")
	}
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "node_id"}, {Name: "inbound_tag"}, {Name: "client_email"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "plan_id", "protocol", "expires_at", "traffic_limit_bytes",
				"enabled", "updated_at",
			}),
		}).
		Create(row)
	if res.Error != nil {
		return nil, fmt.Errorf("ClientOwnership.Upsert: %w", res.Error)
	}
	return row, nil
}

// ClearForClient removes the ownership row for the (node, inbound,
// email) triple. Returns nil if the row was already gone.
func (r *ClientOwnershipRepo) ClearForClient(ctx context.Context, nodeID int64, inboundTag, email string) error {
	if err := r.db.WithContext(ctx).
		Where("node_id = ? AND inbound_tag = ? AND client_email = ?", nodeID, inboundTag, email).
		Delete(&model.ClientOwnership{}).Error; err != nil {
		return fmt.Errorf("ClientOwnership.ClearForClient: %w", err)
	}
	return nil
}

// ListByUser returns every ownership row owned by userID, used by
// the central subscription assembler and the user-portal traffic
// view.
func (r *ClientOwnershipRepo) ListByUser(ctx context.Context, userID int64) ([]model.ClientOwnership, error) {
	var rows []model.ClientOwnership
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ClientOwnership.ListByUser: %w", err)
	}
	return rows, nil
}

// SetEnabled flips the enabled bit for one ownership row.
func (r *ClientOwnershipRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	res := r.db.WithContext(ctx).
		Model(&model.ClientOwnership{}).
		Where("id = ?", id).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return fmt.Errorf("ClientOwnership.SetEnabled: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FindByEmail searches every ownership row whose client_email matches
// (case-insensitively). Used by the admin "find client by email"
// flow.
func (r *ClientOwnershipRepo) FindByEmail(ctx context.Context, email string) ([]model.ClientOwnership, error) {
	var rows []model.ClientOwnership
	if err := r.db.WithContext(ctx).
		Where("LOWER(client_email) = LOWER(?)", email).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ClientOwnership.FindByEmail: %w", err)
	}
	return rows, nil
}

// ListExpired returns ownership rows where `expires_at <= now()` AND
// `enabled = true`. Used by the expiry-processing cron to find rows
// that need to be disabled. A nil expires_at is treated as "no expiry"
// and excluded.
func (r *ClientOwnershipRepo) ListExpired(ctx context.Context, now time.Time) ([]model.ClientOwnership, error) {
	var rows []model.ClientOwnership
	if err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ? AND enabled = TRUE", now).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ClientOwnership.ListExpired: %w", err)
	}
	return rows, nil
}

// ListExpiringWithin returns ownership rows where
// `now < expires_at <= now + window` AND `enabled = true`. Used by
// the expiry-reminder cron to fire warning events for clients near
// expiry.
func (r *ClientOwnershipRepo) ListExpiringWithin(ctx context.Context, now time.Time, window time.Duration) ([]model.ClientOwnership, error) {
	var rows []model.ClientOwnership
	if err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ? AND enabled = TRUE",
			now, now.Add(window)).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ClientOwnership.ListExpiringWithin: %w", err)
	}
	return rows, nil
}
