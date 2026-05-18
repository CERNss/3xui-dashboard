// Package billing orchestrates plan purchases: idempotency lookup →
// balance charge → ProvisionClient → completion (or refund on
// failure). Plan CRUD lives here too so the admin handler stays
// thin.
package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/client"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// Errors callers branch on.
var (
	ErrInsufficientBalance = errors.New("billing: insufficient balance")
	ErrUserNotFound        = errors.New("billing: user not found")
	ErrPlanNotFound        = errors.New("billing: plan not found")
	ErrPlanDisabled        = errors.New("billing: plan disabled")
	ErrInvalidInput        = errors.New("billing: invalid input")
)

// Service composes the repos + the client provisioning service.
type Service struct {
	plans    *repository.PlanRepo
	orders   *repository.OrderRepo
	users    *repository.UserRepo
	client   *client.Service
	bus      *event.Bus
	log      *slog.Logger
}

// New constructs the service.
func New(plans *repository.PlanRepo, orders *repository.OrderRepo, users *repository.UserRepo, client *client.Service, bus *event.Bus, lg *slog.Logger) *Service {
	return &Service{
		plans:  plans,
		orders: orders,
		users:  users,
		client: client,
		bus:    bus,
		log:    lg.With(slog.String("component", "service.billing")),
	}
}

// ---- Plan admin -----------------------------------------------------------

func (s *Service) CreatePlan(ctx context.Context, p *model.Plan) (*model.Plan, error) {
	if err := normalizePlan(p); err != nil {
		return nil, err
	}
	if err := s.plans.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) UpdatePlan(ctx context.Context, id int64, fields map[string]any) (*model.Plan, error) {
	if err := s.plans.Update(ctx, id, fields); err != nil {
		return nil, err
	}
	return s.plans.Get(ctx, id)
}

func (s *Service) DeletePlan(ctx context.Context, id int64) error {
	return s.plans.Delete(ctx, id)
}

func (s *Service) ListPlans(ctx context.Context, onlyEnabled bool) ([]model.Plan, error) {
	return s.plans.List(ctx, onlyEnabled)
}

// ---- Purchase -------------------------------------------------------------

// PurchaseInput is the API-side shape of a purchase request. Idempotency
// key is required — the caller (handler) generates one and retries are
// safe.
type PurchaseInput struct {
	UserID         int64
	PlanID         int64
	IdempotencyKey string
	NodeID         int64
	InboundTag     string
}

// Purchase runs the full purchase flow. Behaviour:
//   - empty idempotency key  → ErrInvalidInput
//   - dupe idempotency key   → returns the original order unchanged
//   - plan missing/disabled  → ErrPlanNotFound / ErrPlanDisabled
//   - insufficient balance   → order recorded as failed → ErrInsufficientBalance
//   - provisioning failure   → balance refunded, order marked refunded
//   - success                → balance charged, ownership upserted,
//                              order marked completed
// Emits order.created on first persistence, order.completed on
// success, order.failed when provisioning fails post-charge.
func (s *Service) Purchase(ctx context.Context, in PurchaseInput) (*model.Order, error) {
	if in.IdempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", ErrInvalidInput)
	}
	// Idempotency short-circuit.
	if existing, err := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	user, err := s.users.Get(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	plan, err := s.plans.Get(ctx, in.PlanID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}
	if !plan.Enabled {
		return nil, ErrPlanDisabled
	}

	// Record the order in pending state up front so the idempotency
	// key is reserved (concurrent dupes will collide on the unique
	// index and the retry path will find the existing row).
	order := &model.Order{
		UserID:         user.ID,
		PlanID:         plan.ID,
		IdempotencyKey: in.IdempotencyKey,
		PriceCents:     plan.PriceCents,
		Status:         model.OrderStatusPending,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		if isUniqueViolation(err) {
			// Concurrent dupe — fetch and return that one.
			if existing, gErr := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); gErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	s.bus.PublishType(event.OrderCreated, OrderEventPayload{
		OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
	})

	if user.BalanceCents < plan.PriceCents {
		_ = s.orders.MarkFailed(ctx, order.ID, "insufficient balance")
		s.bus.PublishType(event.OrderFailed, OrderEventPayload{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
			Reason: "insufficient_balance",
		})
		return order, fmt.Errorf("%w: have=%d, need=%d", ErrInsufficientBalance, user.BalanceCents, plan.PriceCents)
	}

	// Charge.
	if _, err := s.users.AdjustBalance(ctx, user.ID, -plan.PriceCents, model.BalanceReasonOrderCharge, "", &order.ID); err != nil {
		_ = s.orders.MarkFailed(ctx, order.ID, "charge failed: "+err.Error())
		s.bus.PublishType(event.OrderFailed, OrderEventPayload{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents, Reason: "charge_failed",
		})
		return order, fmt.Errorf("billing.Purchase: charge: %w", err)
	}

	// Provision.
	planID := plan.ID
	ownership, err := s.client.ProvisionClient(ctx, user.ID, in.NodeID, in.InboundTag, client.PlanParams{
		PlanID:            &planID,
		DurationDays:      plan.DurationDays,
		TrafficLimitBytes: plan.TrafficLimitBytes,
	})
	if err != nil {
		// Refund.
		if _, refundErr := s.users.AdjustBalance(ctx, user.ID, plan.PriceCents, model.BalanceReasonOrderRefund, err.Error(), &order.ID); refundErr != nil {
			s.log.Error("refund failed after provisioning failure",
				slog.Int64("order_id", order.ID),
				slog.String("refund_err", refundErr.Error()),
				slog.String("provision_err", err.Error()),
			)
		}
		_ = s.orders.MarkRefunded(ctx, order.ID, err.Error())
		s.bus.PublishType(event.OrderFailed, OrderEventPayload{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents, Reason: "provisioning_failed",
		})
		return order, fmt.Errorf("billing.Purchase: provision: %w", err)
	}

	if err := s.orders.MarkCompleted(ctx, order.ID, ownership.ID); err != nil {
		s.log.Error("mark completed failed", slog.Int64("order_id", order.ID), slog.String("error", err.Error()))
	}
	s.bus.PublishType(event.OrderCompleted, OrderEventPayload{
		OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
	})
	// Refresh the in-memory order with the completion fields before
	// returning so the handler doesn't see stale status=pending.
	if reloaded, err := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil && reloaded != nil {
		return reloaded, nil
	}
	order.Status = model.OrderStatusCompleted
	return order, nil
}

// ---- Order listing --------------------------------------------------------

func (s *Service) ListOrdersByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Order, error) {
	return s.orders.ListByUser(ctx, userID, limit, offset)
}

func (s *Service) ListOrdersAdmin(ctx context.Context, filter repository.OrderFilter, limit, offset int) ([]model.Order, error) {
	return s.orders.ListAdmin(ctx, filter, limit, offset)
}

// ---- helpers --------------------------------------------------------------

func normalizePlan(p *model.Plan) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if p.PriceCents < 0 {
		return fmt.Errorf("%w: price_cents must be >= 0", ErrInvalidInput)
	}
	if p.DurationDays < 0 {
		return fmt.Errorf("%w: duration_days must be >= 0", ErrInvalidInput)
	}
	if p.TrafficLimitBytes < 0 {
		return fmt.Errorf("%w: traffic_limit_bytes must be >= 0", ErrInvalidInput)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}

// OrderEventPayload is the per-event shape on event.OrderCreated /
// .OrderCompleted / .OrderFailed.
type OrderEventPayload struct {
	OrderID    int64  `json:"order_id"`
	UserID     int64  `json:"user_id"`
	PlanID     int64  `json:"plan_id"`
	PriceCents int64  `json:"price_cents"`
	Reason     string `json:"reason,omitempty"` // set on OrderFailed only
}
