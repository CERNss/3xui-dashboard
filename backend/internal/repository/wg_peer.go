package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// WGPeerRepo persists the WireGuard peer mirror rows used to
// re-render subscriptions without round-tripping to the node.
//
// The companion advisory lock helper lives here too: the WG
// peer mutation path SHALL `pg_advisory_xact_lock(inbound_id)` to
// serialize dashboard-side RMW cycles. See proposal §4.1.
type WGPeerRepo struct{ db *gorm.DB }

// NewWGPeerRepo returns a repo bound to db.
func NewWGPeerRepo(db *gorm.DB) *WGPeerRepo { return &WGPeerRepo{db: db} }

// DB exposes the underlying gorm handle so callers can wrap an
// inbound mutation in a tx that also acquires the advisory lock.
// Kept tightly scoped — no service code should be doing raw SQL
// elsewhere through this.
func (r *WGPeerRepo) DB() *gorm.DB { return r.db }

// AdvisoryLock acquires pg_advisory_xact_lock(inboundID) on the
// given transaction. The lock is auto-released at commit/rollback.
// Returns nil on success; any DB error is wrapped.
func AdvisoryLock(ctx context.Context, tx *gorm.DB, inboundID int64) error {
	if err := tx.WithContext(ctx).
		Exec("SELECT pg_advisory_xact_lock(?)", inboundID).Error; err != nil {
		return fmt.Errorf("wg: pg_advisory_xact_lock(%d): %w", inboundID, err)
	}
	return nil
}

// Create inserts a new peer row. Caller MUST be inside a tx that
// already holds the advisory lock — concurrent inserts without it
// can produce IP collisions even with the unique index.
func (r *WGPeerRepo) Create(ctx context.Context, tx *gorm.DB, p *model.WGPeer) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("WGPeer.Create: %w", err)
	}
	return nil
}

// GetByOwnership returns the peer row for the named ownership, or
// (nil, nil) on miss.
func (r *WGPeerRepo) GetByOwnership(ctx context.Context, ownershipID int64) (*model.WGPeer, error) {
	var p model.WGPeer
	err := r.db.WithContext(ctx).
		Where("client_ownership_id = ?", ownershipID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("WGPeer.GetByOwnership: %w", err)
	}
	return &p, nil
}

// ListByInbound returns every peer attached to the inbound. Used
// for re-rendering when there is no email/ownership in hand.
func (r *WGPeerRepo) ListByInbound(ctx context.Context, inboundID int64) ([]model.WGPeer, error) {
	var rows []model.WGPeer
	if err := r.db.WithContext(ctx).
		Where("inbound_id = ?", inboundID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("WGPeer.ListByInbound: %w", err)
	}
	return rows, nil
}

// AllocatedIPs returns the set of taken IPs on this inbound. The
// allocator picks the next free one inside the inbound's subnet.
// Caller MUST hold the advisory lock — otherwise the returned set
// can race with a concurrent insert.
func (r *WGPeerRepo) AllocatedIPs(ctx context.Context, tx *gorm.DB, inboundID int64) (map[string]struct{}, error) {
	if tx == nil {
		tx = r.db
	}
	var ips []string
	if err := tx.WithContext(ctx).
		Model(&model.WGPeer{}).
		Where("inbound_id = ?", inboundID).
		Pluck("allocated_ip::text", &ips).Error; err != nil {
		return nil, fmt.Errorf("WGPeer.AllocatedIPs: %w", err)
	}
	out := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		out[ip] = struct{}{}
	}
	return out, nil
}

// DeleteByOwnership removes the peer mirror row. Idempotent.
func (r *WGPeerRepo) DeleteByOwnership(ctx context.Context, tx *gorm.DB, ownershipID int64) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.WithContext(ctx).
		Where("client_ownership_id = ?", ownershipID).
		Delete(&model.WGPeer{}).Error; err != nil {
		return fmt.Errorf("WGPeer.DeleteByOwnership: %w", err)
	}
	return nil
}
