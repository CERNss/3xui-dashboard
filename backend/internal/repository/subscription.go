package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// SubscriptionProfileRepo persists the admin-defined routing profiles
// the /sub converter applies when rendering ruled targets.
type SubscriptionProfileRepo struct{ db *gorm.DB }

func NewSubscriptionProfileRepo(db *gorm.DB) *SubscriptionProfileRepo {
	return &SubscriptionProfileRepo{db: db}
}

func (r *SubscriptionProfileRepo) List(ctx context.Context) ([]model.SubscriptionProfile, error) {
	var rows []model.SubscriptionProfile
	if err := r.db.WithContext(ctx).Order("is_default DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("SubscriptionProfile.List: %w", err)
	}
	return rows, nil
}

func (r *SubscriptionProfileRepo) Get(ctx context.Context, id int64) (*model.SubscriptionProfile, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *SubscriptionProfileRepo) GetByKey(ctx context.Context, key string) (*model.SubscriptionProfile, error) {
	return r.first(ctx, "key = ?", key)
}

// GetDefault returns the global default profile, or (nil, nil) when none
// is configured — callers fall back to the renderer's embedded default.
func (r *SubscriptionProfileRepo) GetDefault(ctx context.Context) (*model.SubscriptionProfile, error) {
	return r.first(ctx, "is_default = ?", true)
}

func (r *SubscriptionProfileRepo) first(ctx context.Context, query string, args ...any) (*model.SubscriptionProfile, error) {
	var row model.SubscriptionProfile
	if err := r.db.WithContext(ctx).Where(query, args...).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("SubscriptionProfile.first: %w", err)
	}
	return &row, nil
}

func (r *SubscriptionProfileRepo) Create(ctx context.Context, p *model.SubscriptionProfile) error {
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(p).Error; err != nil {
		return fmt.Errorf("SubscriptionProfile.Create: %w", err)
	}
	return nil
}

func (r *SubscriptionProfileRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&model.SubscriptionProfile{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("SubscriptionProfile.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetDefault makes id the sole default in one transaction (the partial
// unique index forbids two defaults, so the old one must be cleared
// first).
func (r *SubscriptionProfileRepo) SetDefault(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SubscriptionProfile{}).
			Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		res := tx.Model(&model.SubscriptionProfile{}).Where("id = ?", id).Update("is_default", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *SubscriptionProfileRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.SubscriptionProfile{}, id)
	if res.Error != nil {
		return fmt.Errorf("SubscriptionProfile.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SubscriptionRulesetRepo persists reusable rulesets + their server-side
// fetch cache.
type SubscriptionRulesetRepo struct{ db *gorm.DB }

func NewSubscriptionRulesetRepo(db *gorm.DB) *SubscriptionRulesetRepo {
	return &SubscriptionRulesetRepo{db: db}
}

func (r *SubscriptionRulesetRepo) List(ctx context.Context) ([]model.SubscriptionRuleset, error) {
	var rows []model.SubscriptionRuleset
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("SubscriptionRuleset.List: %w", err)
	}
	return rows, nil
}

// ListEnabledRemote returns enabled remote rulesets — the set the
// refresh job re-fetches.
func (r *SubscriptionRulesetRepo) ListEnabledRemote(ctx context.Context) ([]model.SubscriptionRuleset, error) {
	var rows []model.SubscriptionRuleset
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND source_type = ?", true, model.RulesetSourceRemote).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("SubscriptionRuleset.ListEnabledRemote: %w", err)
	}
	return rows, nil
}

func (r *SubscriptionRulesetRepo) Get(ctx context.Context, id int64) (*model.SubscriptionRuleset, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *SubscriptionRulesetRepo) GetByKey(ctx context.Context, key string) (*model.SubscriptionRuleset, error) {
	return r.first(ctx, "key = ?", key)
}

func (r *SubscriptionRulesetRepo) first(ctx context.Context, query string, args ...any) (*model.SubscriptionRuleset, error) {
	var row model.SubscriptionRuleset
	if err := r.db.WithContext(ctx).Where(query, args...).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("SubscriptionRuleset.first: %w", err)
	}
	return &row, nil
}

func (r *SubscriptionRulesetRepo) Create(ctx context.Context, rs *model.SubscriptionRuleset) error {
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(rs).Error; err != nil {
		return fmt.Errorf("SubscriptionRuleset.Create: %w", err)
	}
	return nil
}

func (r *SubscriptionRulesetRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&model.SubscriptionRuleset{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("SubscriptionRuleset.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SaveCache records the result of a remote fetch. Always bumps
// last_fetched_at/last_status; content+etag only change on a real body.
func (r *SubscriptionRulesetRepo) SaveCache(ctx context.Context, id int64, content, etag, status string, fetchedAt time.Time) error {
	fields := map[string]any{
		"last_fetched_at": fetchedAt,
		"last_status":     status,
		"updated_at":      time.Now().UTC(),
	}
	if content != "" {
		fields["cached_content"] = content
	}
	if etag != "" {
		fields["cached_etag"] = etag
	}
	if err := r.db.WithContext(ctx).Model(&model.SubscriptionRuleset{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("SubscriptionRuleset.SaveCache: %w", err)
	}
	return nil
}

func (r *SubscriptionRulesetRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.SubscriptionRuleset{}, id)
	if res.Error != nil {
		return fmt.Errorf("SubscriptionRuleset.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
