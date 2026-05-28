package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// ProvisioningPoolRepo persists admin-defined auto-provisioning pools.
type ProvisioningPoolRepo struct{ db *gorm.DB }

func NewProvisioningPoolRepo(db *gorm.DB) *ProvisioningPoolRepo {
	return &ProvisioningPoolRepo{db: db}
}

func (r *ProvisioningPoolRepo) List(ctx context.Context) ([]model.ProvisioningPool, error) {
	var rows []model.ProvisioningPool
	if err := r.db.WithContext(ctx).
		Order("id ASC").
		Preload("Targets", func(db *gorm.DB) *gorm.DB {
			return db.Table("provisioning_pool_targets AS ppt").
				Select("ppt.*, n.name AS node_name, COUNT(co.id)::int AS used_clients").
				Joins("LEFT JOIN nodes n ON n.id = ppt.node_id").
				Joins("LEFT JOIN client_ownerships co ON co.node_id = ppt.node_id AND co.inbound_tag = ppt.inbound_tag AND co.enabled = TRUE").
				Group("ppt.id, n.name").
				Order("ppt.priority ASC, ppt.id ASC")
		}).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ProvisioningPool.List: %w", err)
	}
	return rows, nil
}

func (r *ProvisioningPoolRepo) Get(ctx context.Context, id int64) (*model.ProvisioningPool, error) {
	var row model.ProvisioningPool
	if err := r.db.WithContext(ctx).
		Preload("Targets", func(db *gorm.DB) *gorm.DB {
			return db.Order("priority ASC, id ASC")
		}).
		First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("ProvisioningPool.Get: %w", err)
	}
	return &row, nil
}

func (r *ProvisioningPoolRepo) Create(ctx context.Context, p *model.ProvisioningPool) error {
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt", "Targets").Create(p).Error; err != nil {
		return fmt.Errorf("ProvisioningPool.Create: %w", err)
	}
	return nil
}

func (r *ProvisioningPoolRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.ProvisioningPool{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("ProvisioningPool.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ProvisioningPoolRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.ProvisioningPool{}, id)
	if res.Error != nil {
		return fmt.Errorf("ProvisioningPool.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ProvisioningPoolRepo) ListTemplates(ctx context.Context) ([]model.InboundTemplate, error) {
	var rows []model.InboundTemplate
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("InboundTemplate.List: %w", err)
	}
	return rows, nil
}

func (r *ProvisioningPoolRepo) GetTemplate(ctx context.Context, id int64) (*model.InboundTemplate, error) {
	var row model.InboundTemplate
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("InboundTemplate.Get: %w", err)
	}
	return &row, nil
}

func (r *ProvisioningPoolRepo) CreateTemplate(ctx context.Context, t *model.InboundTemplate) error {
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(t).Error; err != nil {
		return fmt.Errorf("InboundTemplate.Create: %w", err)
	}
	return nil
}

func (r *ProvisioningPoolRepo) UpdateTemplate(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.InboundTemplate{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("InboundTemplate.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ProvisioningPoolRepo) DeleteTemplate(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.InboundTemplate{}, id)
	if res.Error != nil {
		return fmt.Errorf("InboundTemplate.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ProvisioningPoolRepo) CreateTarget(ctx context.Context, t *model.ProvisioningPoolTarget) error {
	if err := r.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(t).Error; err != nil {
		return fmt.Errorf("ProvisioningPool.CreateTarget: %w", err)
	}
	return nil
}

func (r *ProvisioningPoolRepo) GetTarget(ctx context.Context, id int64) (*model.ProvisioningPoolTarget, error) {
	var row model.ProvisioningPoolTarget
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("ProvisioningPool.GetTarget: %w", err)
	}
	return &row, nil
}

func (r *ProvisioningPoolRepo) UpdateTarget(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.ProvisioningPoolTarget{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("ProvisioningPool.UpdateTarget: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ProvisioningPoolRepo) DeleteTarget(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.ProvisioningPoolTarget{}, id)
	if res.Error != nil {
		return fmt.Errorf("ProvisioningPool.DeleteTarget: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Candidate is a target joined with current enabled ownership count.
type Candidate struct {
	ID               int64
	PoolID           int64
	NodeID           int64
	NodeName         string
	InboundTag       string
	Protocol         string
	MaxClients       int
	Priority         int
	UsedClients      int
	AllowedProtocols model.StringSlice
}

// Occupancy returns enabled ownerships plus unresolved orders that
// have already reserved the same target. Counting pending orders
// closes the common "two buyers see the final slot" gap for both
// balance purchases and gateway payments awaiting confirmation.
func (r *ProvisioningPoolRepo) Occupancy(ctx context.Context, nodeID int64, inboundTag string) (int, error) {
	var used int64
	err := r.db.WithContext(ctx).
		Table("client_ownerships AS co").
		Where("co.node_id = ? AND co.inbound_tag = ? AND co.enabled = TRUE", nodeID, inboundTag).
		Count(&used).Error
	if err != nil {
		return 0, fmt.Errorf("ProvisioningPool.Occupancy ownerships: %w", err)
	}

	var reserved int64
	err = r.db.WithContext(ctx).
		Table("orders AS o").
		Where("o.provisioning_node_id = ? AND o.provisioning_inbound_tag = ? AND o.status IN ?",
			nodeID, inboundTag, unresolvedProvisioningOrderStatuses()).
		Count(&reserved).Error
	if err != nil {
		return 0, fmt.Errorf("ProvisioningPool.Occupancy orders: %w", err)
	}
	return int(used + reserved), nil
}

// ListCandidates returns enabled targets for an enabled pool, ordered
// by target priority and current dashboard-side occupancy.
func (r *ProvisioningPoolRepo) ListCandidates(ctx context.Context, poolID int64) ([]Candidate, error) {
	return r.ListCandidatesForUserExcludingOrder(ctx, poolID, 0, 0)
}

func (r *ProvisioningPoolRepo) ListCandidatesForUser(ctx context.Context, poolID, userID int64) ([]Candidate, error) {
	return r.ListCandidatesForUserExcludingOrder(ctx, poolID, userID, 0)
}

// ListCandidatesForUserExcludingOrder is the same query as
// ListCandidates, but can ignore the buyer's existing ownership and
// one order reservation. That lets renewals extend an existing client
// even when max_clients is already reached by that same client.
func (r *ProvisioningPoolRepo) ListCandidatesForUserExcludingOrder(ctx context.Context, poolID, userID, excludeOrderID int64) ([]Candidate, error) {
	var rows []Candidate
	ownershipJoin := "LEFT JOIN client_ownerships co ON co.node_id = ppt.node_id AND co.inbound_tag = ppt.inbound_tag AND co.enabled = TRUE"
	ownershipArgs := []any{}
	if userID > 0 {
		ownershipJoin += " AND co.user_id <> ?"
		ownershipArgs = append(ownershipArgs, userID)
	}
	orderJoin := "LEFT JOIN orders o ON o.provisioning_node_id = ppt.node_id AND o.provisioning_inbound_tag = ppt.inbound_tag AND o.status IN ?"
	orderArgs := []any{unresolvedProvisioningOrderStatuses()}
	if excludeOrderID > 0 {
		orderJoin += " AND o.id <> ?"
		orderArgs = append(orderArgs, excludeOrderID)
	}
	err := r.db.WithContext(ctx).
		Table("provisioning_pool_targets AS ppt").
		Select(strings.Join([]string{
			"ppt.id",
			"ppt.pool_id",
			"ppt.node_id",
			"n.name AS node_name",
			"ppt.inbound_tag",
			"ppt.protocol",
			"ppt.max_clients",
			"ppt.priority",
			"(COUNT(DISTINCT co.id) + COUNT(DISTINCT o.id))::int AS used_clients",
			"p.allowed_protocols AS allowed_protocols",
		}, ", ")).
		Joins("JOIN provisioning_pools p ON p.id = ppt.pool_id AND p.enabled = TRUE").
		Joins("JOIN nodes n ON n.id = ppt.node_id AND n.enabled = TRUE").
		Joins(ownershipJoin, ownershipArgs...).
		Joins(orderJoin, orderArgs...).
		Where("ppt.pool_id = ? AND ppt.enabled = TRUE", poolID).
		Group("ppt.id, n.name, p.allowed_protocols").
		Having("ppt.max_clients = 0 OR (COUNT(DISTINCT co.id) + COUNT(DISTINCT o.id)) < ppt.max_clients").
		Order("ppt.priority ASC, (COUNT(DISTINCT co.id) + COUNT(DISTINCT o.id)) ASC, ppt.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("ProvisioningPool.ListCandidates: %w", err)
	}
	return rows, nil
}

func unresolvedProvisioningOrderStatuses() []string {
	return []string{
		model.OrderStatusPending,
		model.OrderStatusPaymentPending,
		model.OrderStatusPaid,
	}
}
