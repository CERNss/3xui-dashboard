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

// ProvenanceRepo persists the "created by this dashboard" ledger for
// inbounds and clients (managed_inbounds / managed_clients). It is
// the authoritative signal behind the admin views' managed/external
// split. client_ownerships is NOT written here — that table remains
// the user↔client billing bridge.
type ProvenanceRepo struct{ db *gorm.DB }

// NewProvenanceRepo returns a repository bound to db.
func NewProvenanceRepo(db *gorm.DB) *ProvenanceRepo {
	return &ProvenanceRepo{db: db}
}

// RecordInbound marks (nodeID, tag) as dashboard-created. Idempotent:
// re-recording an existing pair is a no-op.
func (r *ProvenanceRepo) RecordInbound(ctx context.Context, nodeID int64, tag string) error {
	if nodeID == 0 || tag == "" {
		return errors.New("Provenance.RecordInbound: node_id + inbound_tag are required")
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.ManagedInbound{NodeID: nodeID, InboundTag: tag}).Error
	if err != nil {
		return fmt.Errorf("Provenance.RecordInbound: %w", err)
	}
	return nil
}

// RecordClient marks (nodeID, tag, email) as dashboard-created.
// Idempotent like RecordInbound.
func (r *ProvenanceRepo) RecordClient(ctx context.Context, nodeID int64, tag, email string) error {
	if nodeID == 0 || tag == "" || email == "" {
		return errors.New("Provenance.RecordClient: node_id + inbound_tag + client_email are required")
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.ManagedClient{NodeID: nodeID, InboundTag: tag, ClientEmail: email}).Error
	if err != nil {
		return fmt.Errorf("Provenance.RecordClient: %w", err)
	}
	return nil
}

// ForgetClient drops the ledger row for one client. Missing row is
// success (delete paths are idempotent end-to-end).
func (r *ProvenanceRepo) ForgetClient(ctx context.Context, nodeID int64, tag, email string) error {
	if err := r.db.WithContext(ctx).
		Where("node_id = ? AND inbound_tag = ? AND client_email = ?", nodeID, tag, email).
		Delete(&model.ManagedClient{}).Error; err != nil {
		return fmt.Errorf("Provenance.ForgetClient: %w", err)
	}
	return nil
}

// ForgetInbound clears every dashboard-side record tied to one
// inbound in a single transaction: the inbound ledger row, all client
// ledger rows on it, AND the client_ownerships rows — deleting the
// inbound on the panel destroys its clients, so ownership rows would
// be dead weight (wg_peers / notification_log rows cascade away via
// their FKs on client_ownerships).
func (r *ProvenanceRepo) ForgetInbound(ctx context.Context, nodeID int64, tag string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ? AND inbound_tag = ?", nodeID, tag).
			Delete(&model.ManagedClient{}).Error; err != nil {
			return err
		}
		if err := tx.Where("node_id = ? AND inbound_tag = ?", nodeID, tag).
			Delete(&model.ManagedInbound{}).Error; err != nil {
			return err
		}
		return tx.Where("node_id = ? AND inbound_tag = ?", nodeID, tag).
			Delete(&model.ClientOwnership{}).Error
	})
	if err != nil {
		return fmt.Errorf("Provenance.ForgetInbound: %w", err)
	}
	return nil
}

// RenameInboundTag follows a panel-side tag rename across every
// tag-keyed dashboard table in one transaction: managed_inbounds,
// managed_clients, client_ownerships, and provisioning_pool_targets.
// Without this, a rename orphans ownership rows and silently breaks
// user subscriptions. No-op when the tag didn't actually change.
func (r *ProvenanceRepo) RenameInboundTag(ctx context.Context, nodeID int64, oldTag, newTag string) error {
	if newTag == "" || oldTag == newTag {
		return nil
	}
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ManagedInbound{}).
			Where("node_id = ? AND inbound_tag = ?", nodeID, oldTag).
			Update("inbound_tag", newTag).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ManagedClient{}).
			Where("node_id = ? AND inbound_tag = ?", nodeID, oldTag).
			Update("inbound_tag", newTag).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ClientOwnership{}).
			Where("node_id = ? AND inbound_tag = ?", nodeID, oldTag).
			Updates(map[string]any{"inbound_tag": newTag, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.ProvisioningPoolTarget{}).
			Where("node_id = ? AND inbound_tag = ?", nodeID, oldTag).
			Updates(map[string]any{"inbound_tag": newTag, "updated_at": now}).Error
	})
	if err != nil {
		return fmt.Errorf("Provenance.RenameInboundTag %q -> %q: %w", oldTag, newTag, err)
	}
	return nil
}

// ListInboundRefs returns every managed-inbound ledger row. Used by
// the fleet list to stamp Managed on each inbound.
func (r *ProvenanceRepo) ListInboundRefs(ctx context.Context) ([]model.ManagedInbound, error) {
	var rows []model.ManagedInbound
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("Provenance.ListInboundRefs: %w", err)
	}
	return rows, nil
}

// ClientStateRow is the merged per-client annotation the admin fleet
// view consumes: managed (ledger row exists) + the bound portal user
// (ownership row, when present). A row appears when EITHER table
// knows the client.
type ClientStateRow struct {
	NodeID      int64  `gorm:"column:node_id"`
	InboundTag  string `gorm:"column:inbound_tag"`
	ClientEmail string `gorm:"column:client_email"`
	Managed     bool   `gorm:"column:managed"`
	UserID      *int64 `gorm:"column:user_id"`
}

// ListClientStates merges managed_clients with client_ownerships into
// one annotation set (FULL OUTER JOIN on the triple).
func (r *ProvenanceRepo) ListClientStates(ctx context.Context) ([]ClientStateRow, error) {
	var rows []ClientStateRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
		  COALESCE(mc.node_id,      co.node_id)      AS node_id,
		  COALESCE(mc.inbound_tag,  co.inbound_tag)  AS inbound_tag,
		  COALESCE(mc.client_email, co.client_email) AS client_email,
		  (mc.id IS NOT NULL)                        AS managed,
		  co.user_id                                 AS user_id
		FROM managed_clients mc
		FULL OUTER JOIN client_ownerships co
		  ON  co.node_id      = mc.node_id
		  AND co.inbound_tag  = mc.inbound_tag
		  AND co.client_email = mc.client_email`).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("Provenance.ListClientStates: %w", err)
	}
	return rows, nil
}
