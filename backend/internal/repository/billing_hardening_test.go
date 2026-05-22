package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestOrderRepo_ListExpiredPending_UsesPaymentExpiresAt(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	fallbackCutoff := now.Add(-15 * time.Minute)
	expiredAt := now.Add(-time.Minute)
	futureExpiresAt := now.Add(time.Minute)
	oldCreatedAt := now.Add(-30 * time.Minute)

	user := seedBillingUser(t, db, "alice@example.com")
	plan := &model.Plan{Name: "p", PriceCents: 100, Enabled: true}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	seedOrder := func(key string, createdAt time.Time, expiresAt *time.Time) int64 {
		t.Helper()
		o := &model.Order{
			UserID:           user.ID,
			PlanID:           plan.ID,
			IdempotencyKey:   key,
			PriceCents:       100,
			Status:           model.OrderStatusPaymentPending,
			PaymentMethod:    "stripe",
			PaymentExpiresAt: expiresAt,
			CreatedAt:        createdAt,
		}
		if err := db.Create(o).Error; err != nil {
			t.Fatalf("seed order %s: %v", key, err)
		}
		return o.ID
	}

	explicitExpiredID := seedOrder("explicit-expired", now, &expiredAt)
	explicitFutureID := seedOrder("explicit-future", oldCreatedAt, &futureExpiresAt)
	fallbackExpiredID := seedOrder("fallback-expired", oldCreatedAt, nil)

	got, err := NewOrderRepo(db).ListExpiredPending(ctx, now, fallbackCutoff)
	if err != nil {
		t.Fatalf("ListExpiredPending: %v", err)
	}
	ids := map[int64]bool{}
	for _, o := range got {
		ids[o.ID] = true
	}
	if !ids[explicitExpiredID] {
		t.Error("order with past payment_expires_at should expire")
	}
	if ids[explicitFutureID] {
		t.Error("order with future payment_expires_at should not expire even when created_at is old")
	}
	if !ids[fallbackExpiredID] {
		t.Error("order without payment_expires_at should fall back to created_at cutoff")
	}
}

func TestUserRepo_ChargeBalanceIfEnough(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	user := seedBillingUser(t, db, "alice@example.com")
	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("balance_cents", 500).Error; err != nil {
		t.Fatalf("seed balance: %v", err)
	}

	repo := NewUserRepo(db)
	newBalance, have, err := repo.ChargeBalanceIfEnough(ctx, user.ID, 300, model.BalanceReasonOrderCharge, "order", nil)
	if err != nil {
		t.Fatalf("ChargeBalanceIfEnough sufficient: %v", err)
	}
	if have != 500 || newBalance != 200 {
		t.Fatalf("have/newBalance = %d/%d, want 500/200", have, newBalance)
	}

	_, have, err = repo.ChargeBalanceIfEnough(ctx, user.ID, 300, model.BalanceReasonOrderCharge, "order", nil)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("insufficient charge err = %v, want ErrInsufficientBalance", err)
	}
	if have != 200 {
		t.Fatalf("insufficient charge have = %d, want locked balance 200", have)
	}
	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.BalanceCents != 200 {
		t.Fatalf("balance after rejected charge = %d, want 200", reloaded.BalanceCents)
	}
	var logs int64
	if err := db.Model(&model.BalanceLog{}).Where("user_id = ?", user.ID).Count(&logs).Error; err != nil {
		t.Fatalf("count balance logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("balance log count = %d, want only successful debit logged", logs)
	}
}

func TestProvisioningPoolRepo_ListCandidatesCountsPendingOrders(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()

	user := seedBillingUser(t, db, "alice@example.com")
	node := &model.Node{
		Name:     "node-1",
		Scheme:   "https",
		Host:     "node.example.com",
		Port:     443,
		BasePath: "",
		APIToken: "token",
		Enabled:  true,
		Status:   model.NodeStatusOnline,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	pool := &model.ProvisioningPool{Name: "pool", Enabled: true, AllowedProtocols: model.StringSlice{"vless"}}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	target := &model.ProvisioningPoolTarget{
		PoolID:     pool.ID,
		NodeID:     node.ID,
		InboundTag: "vless-1",
		Protocol:   "vless",
		MaxClients: 1,
		Priority:   100,
		Enabled:    true,
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("seed target: %v", err)
	}
	plan := &model.Plan{Name: "p", PriceCents: 100, Enabled: true, ProvisioningPoolID: &pool.ID}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	nodeID := node.ID
	order := &model.Order{
		UserID:                 user.ID,
		PlanID:                 plan.ID,
		IdempotencyKey:         "pending-1",
		PriceCents:             100,
		Status:                 model.OrderStatusPaymentPending,
		PaymentMethod:          "stripe",
		ProvisioningNodeID:     &nodeID,
		ProvisioningInboundTag: "vless-1",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	repo := NewProvisioningPoolRepo(db)
	candidates, err := repo.ListCandidates(ctx, pool.ID)
	if err != nil {
		t.Fatalf("ListCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("ListCandidates returned %d candidates, want full target hidden", len(candidates))
	}

	candidates, err = repo.ListCandidatesForUserExcludingOrder(ctx, pool.ID, user.ID, order.ID)
	if err != nil {
		t.Fatalf("ListCandidatesForUserExcludingOrder: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("ListCandidatesForUserExcludingOrder returned %d candidates, want own reservation ignored", len(candidates))
	}
	if candidates[0].UsedClients != 0 {
		t.Fatalf("UsedClients = %d, want own reservation excluded", candidates[0].UsedClients)
	}
}

func seedBillingUser(t *testing.T, db *gorm.DB, email string) *model.User {
	t.Helper()
	u := &model.User{Email: &email, SubID: "sub-" + email, Status: model.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}
