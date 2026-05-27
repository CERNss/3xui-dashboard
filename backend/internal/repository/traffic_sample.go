package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// TrafficSampleRepo persists cumulative byte-counter snapshots. The
// collection job inserts; query helpers turn them into per-client /
// per-inbound / per-node deltas with counter-reset detection.
type TrafficSampleRepo struct{ db *gorm.DB }

// NewTrafficSampleRepo returns a repository bound to db.
func NewTrafficSampleRepo(db *gorm.DB) *TrafficSampleRepo {
	return &TrafficSampleRepo{db: db}
}

// InsertBatch persists rows in one statement. Empty slice is a no-op.
func (r *TrafficSampleRepo) InsertBatch(ctx context.Context, rows []model.TrafficSample) error {
	if len(rows) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return fmt.Errorf("TrafficSample.InsertBatch: %w", err)
	}
	return nil
}

// ChronologicalForClient returns samples for one client between from
// and to, oldest-first. Used by HistoryBuckets and delta walks.
func (r *TrafficSampleRepo) ChronologicalForClient(ctx context.Context, nodeID int64, inboundTag, email string, from, to time.Time) ([]model.TrafficSample, error) {
	var rows []model.TrafficSample
	q := r.db.WithContext(ctx).
		Where("node_id = ? AND inbound_tag = ? AND client_email = ?", nodeID, inboundTag, email)
	if !from.IsZero() {
		q = q.Where("taken_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("taken_at <= ?", to)
	}
	q = q.Order("taken_at ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("TrafficSample.ChronologicalForClient: %w", err)
	}
	return rows, nil
}

// DeleteOlderThan trims the table by deleting samples taken before
// cutoff. Returns the number of deleted rows.
func (r *TrafficSampleRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("taken_at < ?", cutoff).
		Delete(&model.TrafficSample{})
	if res.Error != nil {
		return 0, fmt.Errorf("TrafficSample.DeleteOlderThan: %w", res.Error)
	}
	return res.RowsAffected, nil
}
