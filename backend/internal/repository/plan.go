package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// PlanRepo is the persistence side of purchasable plans.
type PlanRepo struct{ db *gorm.DB }

// NewPlanRepo returns a repository bound to db.
func NewPlanRepo(db *gorm.DB) *PlanRepo { return &PlanRepo{db: db} }

// Get returns a plan by id. Returns (nil, nil) on miss.
func (r *PlanRepo) Get(ctx context.Context, id int64) (*model.Plan, error) {
	var p model.Plan
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("PlanRepo.Get: %w", err)
	}
	return &p, nil
}

// List returns plans. Pass onlyEnabled=true to filter to plans the
// portal should show.
func (r *PlanRepo) List(ctx context.Context, onlyEnabled bool) ([]model.Plan, error) {
	var rows []model.Plan
	q := r.db.WithContext(ctx).Order("price_cents ASC, id ASC")
	if onlyEnabled {
		q = q.Where("enabled = TRUE")
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("PlanRepo.List: %w", err)
	}
	return rows, nil
}

// Create persists a new plan.
func (r *PlanRepo) Create(ctx context.Context, p *model.Plan) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("PlanRepo.Create: %w", err)
	}
	return nil
}

// Update applies a partial patch by primary key.
func (r *PlanRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.Plan{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("PlanRepo.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete removes a plan. Orders referencing the plan are blocked by
// FK RESTRICT — caller must disable, not delete, plans with order
// history.
func (r *PlanRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.Plan{}, id)
	if res.Error != nil {
		return fmt.Errorf("PlanRepo.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
