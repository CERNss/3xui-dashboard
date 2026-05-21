package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// AdminActionRepo persists the admin-audit trail. Writes are
// fire-and-forget from the middleware path; reads serve the
// admin audit-log UI.
type AdminActionRepo struct{ db *gorm.DB }

func NewAdminActionRepo(db *gorm.DB) *AdminActionRepo {
	return &AdminActionRepo{db: db}
}

// Insert persists one audit row. Errors are logged but never
// propagated to the request path — audit failure must not break
// admin operations.
func (r *AdminActionRepo) Insert(ctx context.Context, a *model.AdminAction) error {
	if err := r.db.WithContext(ctx).
		Select("AdminUsername", "Method", "Path", "TargetResource", "TargetID", "QueryString", "IP", "UserAgent", "StatusCode", "ErrorMsg").
		Create(a).Error; err != nil {
		return fmt.Errorf("AdminActionRepo.Insert: %w", err)
	}
	return nil
}

// AdminActionFilter narrows the audit-log listing.
type AdminActionFilter struct {
	AdminUsername  *string
	TargetResource *string
	TargetID       *string
	Method         *string
}

// List returns rows matching filter, newest first, paginated.
func (r *AdminActionRepo) List(ctx context.Context, filter AdminActionFilter, limit, offset int) ([]model.AdminAction, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Model(&model.AdminAction{})
	if filter.AdminUsername != nil {
		q = q.Where("admin_username = ?", *filter.AdminUsername)
	}
	if filter.TargetResource != nil {
		q = q.Where("target_resource = ?", *filter.TargetResource)
	}
	if filter.TargetID != nil {
		q = q.Where("target_id = ?", *filter.TargetID)
	}
	if filter.Method != nil {
		q = q.Where("method = ?", *filter.Method)
	}
	var rows []model.AdminAction
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("AdminActionRepo.List: %w", err)
	}
	return rows, nil
}
