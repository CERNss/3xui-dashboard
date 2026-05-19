package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cern/3xui-dashboard/internal/model"
)

// UserRepo is the persistence side of the portal user account. The
// auth/registration service composes higher-level methods on top.
type UserRepo struct{ db *gorm.DB }

// NewUserRepo returns a repository bound to db.
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

// Get returns the user by id. Returns (nil, nil) on miss.
func (r *UserRepo) Get(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.Get: %w", err)
	}
	return &u, nil
}

// GetByEmail returns the user matching email (case-insensitive),
// or (nil, nil) on miss.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	var u model.User
	if err := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?)", email).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetByEmail: %w", err)
	}
	return &u, nil
}

// GetByOIDCSubject returns the user matching oidc_subject, or
// (nil, nil) on miss.
func (r *UserRepo) GetByOIDCSubject(ctx context.Context, sub string) (*model.User, error) {
	if sub == "" {
		return nil, nil
	}
	var u model.User
	if err := r.db.WithContext(ctx).
		Where("oidc_subject = ?", sub).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetByOIDCSubject: %w", err)
	}
	return &u, nil
}

// GetBySubID returns the user owning a public sub_id link, or
// (nil, nil) on miss. Used by the central /sub/:subId handler.
func (r *UserRepo) GetBySubID(ctx context.Context, subID string) (*model.User, error) {
	if subID == "" {
		return nil, nil
	}
	var u model.User
	if err := r.db.WithContext(ctx).
		Where("sub_id = ?", subID).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetBySubID: %w", err)
	}
	return &u, nil
}

// Create persists a new user. The caller is responsible for
// generating SubID and PasswordHash (or setting OIDCSubject).
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	if u.Status == "" {
		u.Status = model.UserStatusActive
	}
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("UserRepo.Create: %w", err)
	}
	return nil
}

// Update applies a partial patch by primary key.
func (r *UserRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("UserRepo.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List returns every user, paged. Order is id ASC.
func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	var rows []model.User
	q := r.db.WithContext(ctx).Order("id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("UserRepo.List: %w", err)
	}
	return rows, nil
}

// Delete removes a user. Cascade on FK takes care of ownerships.
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.User{}, id)
	if res.Error != nil {
		return fmt.Errorf("UserRepo.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AdjustBalance atomically adds delta to balance_cents and writes a
// balance_logs row. Returns the new balance.
//
// Concurrency: the user row is read with SELECT ... FOR UPDATE so
// concurrent calls on the same user serialize through the row lock.
// Without the lock, a parallel charge + refund could each read the
// same starting balance and persist conflicting "balance_after"
// values, leaking money.
func (r *UserRepo) AdjustBalance(ctx context.Context, userID, delta int64, reason, note string, orderID *int64) (int64, error) {
	var newBalance int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID).Error; err != nil {
			return err
		}
		newBalance = u.BalanceCents + delta
		if err := tx.Model(&u).Updates(map[string]any{
			"balance_cents": newBalance,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		log := model.BalanceLog{
			UserID:            userID,
			DeltaCents:        delta,
			BalanceAfterCents: newBalance,
			Reason:            reason,
			Note:              note,
			OrderID:           orderID,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("UserRepo.AdjustBalance: %w", err)
	}
	return newBalance, nil
}
