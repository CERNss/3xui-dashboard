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

// GetForUpdate behaves like Get but holds a SELECT ... FOR UPDATE row
// lock. Only valid when r is the tx-bound repo passed to InTx; with the
// outer DB the lock is released immediately and provides no protection.
func (r *UserRepo) GetForUpdate(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&u, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetForUpdate: %w", err)
	}
	return &u, nil
}

// InTx runs fn inside a database transaction. The repo handed to fn is
// bound to the transaction connection, so any Get/Update/etc. calls on
// it participate in the same transaction. Returning a non-nil error
// from fn rolls back.
func (r *UserRepo) InTx(ctx context.Context, fn func(tx *UserRepo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&UserRepo{db: tx})
	})
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

// Create persists a new user. The caller is responsible for generating
// SubID. PasswordHash is required by the P5 account contract; when a
// legacy/internal path omits it, store the disabled sentinel rather
// than a blank credential.
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	if u.Status == "" {
		u.Status = model.UserStatusActive
	}
	if u.PasswordHash == "" {
		u.PasswordHash = model.DisabledPasswordHash
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

// OIDCProviderFilter controls provider listing.
type OIDCProviderFilter struct {
	EnabledOnly bool
}

// ListOIDCProviders returns configured OIDC providers in stable key order.
func (r *UserRepo) ListOIDCProviders(ctx context.Context, filter OIDCProviderFilter) ([]model.OIDCProvider, error) {
	var rows []model.OIDCProvider
	q := r.db.WithContext(ctx).Order("provider_key ASC")
	if filter.EnabledOnly {
		q = q.Where("enabled = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("UserRepo.ListOIDCProviders: %w", err)
	}
	return rows, nil
}

// GetOIDCProvider returns a configured provider by key, or (nil, nil)
// on miss.
func (r *UserRepo) GetOIDCProvider(ctx context.Context, providerKey string) (*model.OIDCProvider, error) {
	providerKey = strings.TrimSpace(providerKey)
	if providerKey == "" {
		return nil, nil
	}
	var p model.OIDCProvider
	if err := r.db.WithContext(ctx).
		Where("provider_key = ?", providerKey).
		First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetOIDCProvider: %w", err)
	}
	return &p, nil
}

// UpsertOIDCProvider stores provider configuration. It is useful for
// bootstrapping the legacy env/runtime OIDC config into the P5 provider
// table before linking identities that reference it.
func (r *UserRepo) UpsertOIDCProvider(ctx context.Context, p *model.OIDCProvider) error {
	if p == nil || strings.TrimSpace(p.ProviderKey) == "" {
		return fmt.Errorf("UserRepo.UpsertOIDCProvider: provider_key is required")
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if p.Scopes == nil {
		p.Scopes = model.StringSlice{}
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "provider_key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"display_name",
				"icon_url",
				"issuer",
				"client_id",
				"client_secret",
				"redirect_url",
				"scopes",
				"auth_url",
				"token_url",
				"jwks_url",
				"user_info_url",
				"enabled",
				"updated_at",
			}),
		}).
		Create(p).Error; err != nil {
		return fmt.Errorf("UserRepo.UpsertOIDCProvider: %w", err)
	}
	return nil
}

// ListOIDCIdentities returns all identities linked to userID.
func (r *UserRepo) ListOIDCIdentities(ctx context.Context, userID int64) ([]model.UserOIDCIdentity, error) {
	var rows []model.UserOIDCIdentity
	if userID <= 0 {
		return rows, nil
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("provider_key ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("UserRepo.ListOIDCIdentities: %w", err)
	}
	return rows, nil
}

// ListOIDCIdentitiesByUserIDs returns identities grouped by user id.
func (r *UserRepo) ListOIDCIdentitiesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.UserOIDCIdentity, error) {
	out := make(map[int64][]model.UserOIDCIdentity, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []model.UserOIDCIdentity
	if err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Order("user_id ASC, provider_key ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("UserRepo.ListOIDCIdentitiesByUserIDs: %w", err)
	}
	for _, row := range rows {
		out[row.UserID] = append(out[row.UserID], row)
	}
	return out, nil
}

// FindOIDCIdentity returns the identity matching provider+subject, or
// (nil, nil) on miss.
func (r *UserRepo) FindOIDCIdentity(ctx context.Context, providerKey, subject string) (*model.UserOIDCIdentity, error) {
	providerKey = strings.TrimSpace(providerKey)
	subject = strings.TrimSpace(subject)
	if providerKey == "" || subject == "" {
		return nil, nil
	}
	var row model.UserOIDCIdentity
	if err := r.db.WithContext(ctx).
		Where("provider_key = ? AND subject = ?", providerKey, subject).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.FindOIDCIdentity: %w", err)
	}
	return &row, nil
}

// LinkOIDCIdentityToUser creates or refreshes a provider identity for
// the user. It refuses to move an existing provider subject to another
// user, and refuses to replace a user's already-linked identity for the
// same provider.
func (r *UserRepo) LinkOIDCIdentityToUser(ctx context.Context, identity *model.UserOIDCIdentity) error {
	if identity == nil {
		return fmt.Errorf("UserRepo.LinkOIDCIdentityToUser: identity is required")
	}
	if identity.UserID <= 0 || strings.TrimSpace(identity.ProviderKey) == "" || strings.TrimSpace(identity.Subject) == "" {
		return fmt.Errorf("UserRepo.LinkOIDCIdentityToUser: user_id, provider_key, and subject are required")
	}
	identity.ProviderKey = strings.TrimSpace(identity.ProviderKey)
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.ProviderEmail = strings.TrimSpace(strings.ToLower(identity.ProviderEmail))
	now := time.Now().UTC()
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = now
	}
	identity.UpdatedAt = now

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := &UserRepo{db: tx}
		existing, err := txRepo.FindOIDCIdentity(ctx, identity.ProviderKey, identity.Subject)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.UserID != identity.UserID {
				return ErrOIDCIdentityConflict
			}
			return tx.Model(&model.UserOIDCIdentity{}).
				Where("id = ?", existing.ID).
				Updates(map[string]any{
					"provider_email":          identity.ProviderEmail,
					"provider_email_verified": identity.ProviderEmailVerified,
					"updated_at":              identity.UpdatedAt,
				}).Error
		}

		var sameProvider model.UserOIDCIdentity
		err = tx.
			Where("user_id = ? AND provider_key = ?", identity.UserID, identity.ProviderKey).
			First(&sameProvider).Error
		if err == nil {
			return ErrOIDCIdentityConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(identity).Error
	})
	if err != nil {
		return fmt.Errorf("UserRepo.LinkOIDCIdentityToUser: %w", err)
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

// ChargeBalanceIfEnough debits amountCents while holding the user row
// lock and writes the matching balance_logs row only if the balance is
// sufficient. The returned have value is the locked balance before the
// debit, useful for surfacing precise insufficient-balance errors.
func (r *UserRepo) ChargeBalanceIfEnough(ctx context.Context, userID, amountCents int64, reason, note string, orderID *int64) (newBalance, have int64, err error) {
	if amountCents < 0 {
		return 0, 0, fmt.Errorf("UserRepo.ChargeBalanceIfEnough: amount_cents must be >= 0")
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID).Error; err != nil {
			return err
		}
		have = u.BalanceCents
		if have < amountCents {
			return ErrInsufficientBalance
		}
		newBalance = have - amountCents
		if err := tx.Model(&u).Updates(map[string]any{
			"balance_cents": newBalance,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		log := model.BalanceLog{
			UserID:            userID,
			DeltaCents:        -amountCents,
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
		return 0, have, fmt.Errorf("UserRepo.ChargeBalanceIfEnough: %w", err)
	}
	return newBalance, have, nil
}

var ErrInsufficientBalance = errors.New("repository: insufficient balance")

var ErrOIDCIdentityConflict = errors.New("repository: oidc identity conflict")
