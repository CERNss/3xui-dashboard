// Package billing orchestrates plan purchases: idempotency lookup →
// balance charge → ProvisionClient → completion (or refund on
// failure). Plan CRUD lives here too so the admin handler stays
// thin.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/client"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
	"github.com/cern/3xui-dashboard/internal/service/inbound"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// Errors callers branch on.
var (
	ErrInsufficientBalance  = errors.New("billing: insufficient balance")
	ErrUserNotFound         = errors.New("billing: user not found")
	ErrPlanNotFound         = errors.New("billing: plan not found")
	ErrPlanDisabled         = errors.New("billing: plan disabled")
	ErrInvalidInput         = errors.New("billing: invalid input")
	ErrIdempotencyConflict  = errors.New("billing: idempotency conflict")
	ErrOrderNotFound        = errors.New("billing: order not found")
	ErrNoProvisioningTarget = errors.New("billing: no provisioning target available")
	// ErrInvalidOrderState fires when an admin tries to refund an
	// order that isn't in a refundable state (e.g. already refunded,
	// or still pending payment).
	ErrInvalidOrderState = errors.New("billing: order not in refundable state")
)

// Service composes the repos + the client provisioning service.
type Service struct {
	plans    *repository.PlanRepo
	orders   *repository.OrderRepo
	users    *repository.UserRepo
	settings *repository.SettingRepo
	pools    *repository.ProvisioningPoolRepo
	client   *client.Service
	inbounds *inbound.Service
	bus      *event.Bus
	gateways *payment.Registry
	log      *slog.Logger
}

// New constructs the service. `gateways` MAY be nil — in that case
// the payment-via-gateway endpoints reject with ErrUnknownProvider,
// but balance-pay still works.
func New(plans *repository.PlanRepo, orders *repository.OrderRepo, users *repository.UserRepo, client *client.Service, bus *event.Bus, gateways *payment.Registry, lg *slog.Logger) *Service {
	return &Service{
		plans:    plans,
		orders:   orders,
		users:    users,
		client:   client,
		bus:      bus,
		gateways: gateways,
		log:      lg.With(slog.String("component", "service.billing")),
	}
}

// SetSettings attaches the runtime settings repository. Purchase and
// portal listing use it for new-user plan allowlists.
func (s *Service) SetSettings(settings *repository.SettingRepo) {
	s.settings = settings
}

// SetProvisioningPools attaches the pool repository after New so
// billing construction stays compact.
func (s *Service) SetProvisioningPools(pools *repository.ProvisioningPoolRepo) {
	s.pools = pools
}

// SetInboundService attaches the real upstream inbound service used
// by template-driven pools to create inbounds on demand.
func (s *Service) SetInboundService(inbounds *inbound.Service) {
	s.inbounds = inbounds
}

// Gateways exposes the registry so handlers can call EnabledProviders
// for /payment-methods. Returns nil if no providers were configured.
func (s *Service) Gateways() *payment.Registry { return s.gateways }

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
	fields = normalizePlanFields(fields)
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

// ListPlansForUser returns the portal catalog for one user. If the
// operator configured new_user_plan_ids and the user has no paid or
// completed order history, the list is narrowed to that starter set.
func (s *Service) ListPlansForUser(ctx context.Context, userID int64) ([]model.Plan, error) {
	rows, err := s.plans.List(ctx, true)
	if err != nil {
		return nil, err
	}
	allowed, restricted, err := s.newUserPlanPolicy(ctx, userID)
	if err != nil || !restricted {
		return rows, err
	}
	out := rows[:0]
	for _, p := range rows {
		if allowed[p.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}

// ---- Provisioning pools --------------------------------------------------

func (s *Service) ListInboundTemplates(ctx context.Context) ([]model.InboundTemplate, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	return s.pools.ListTemplates(ctx)
}

func (s *Service) CreateInboundTemplate(ctx context.Context, t *model.InboundTemplate) (*model.InboundTemplate, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	if err := normalizeInboundTemplate(t); err != nil {
		return nil, err
	}
	if err := s.pools.CreateTemplate(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) UpdateInboundTemplate(ctx context.Context, id int64, fields map[string]any) (*model.InboundTemplate, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	fields = normalizeInboundTemplateFields(fields)
	if err := s.pools.UpdateTemplate(ctx, id, fields); err != nil {
		return nil, err
	}
	updated, err := s.pools.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return updated, nil
}

func (s *Service) DeleteInboundTemplate(ctx context.Context, id int64) error {
	if s.pools == nil {
		return fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	return s.pools.DeleteTemplate(ctx, id)
}

func (s *Service) ListProvisioningPools(ctx context.Context) ([]model.ProvisioningPool, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	return s.pools.List(ctx)
}

func (s *Service) CreateProvisioningPool(ctx context.Context, p *model.ProvisioningPool) (*model.ProvisioningPool, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	if err := normalizeProvisioningPool(p); err != nil {
		return nil, err
	}
	if err := s.pools.Create(ctx, p); err != nil {
		return nil, err
	}
	created, err := s.pools.Get(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return p, nil
	}
	return created, nil
}

func (s *Service) UpdateProvisioningPool(ctx context.Context, id int64, fields map[string]any) (*model.ProvisioningPool, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	fields = normalizeProvisioningPoolFields(fields)
	if err := s.pools.Update(ctx, id, fields); err != nil {
		return nil, err
	}
	updated, err := s.pools.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return updated, nil
}

func (s *Service) DeleteProvisioningPool(ctx context.Context, id int64) error {
	if s.pools == nil {
		return fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	return s.pools.Delete(ctx, id)
}

func (s *Service) CreateProvisioningTarget(ctx context.Context, t *model.ProvisioningPoolTarget) (*model.ProvisioningPoolTarget, error) {
	if s.pools == nil {
		return nil, fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	if err := normalizeProvisioningTarget(t); err != nil {
		return nil, err
	}
	if err := s.validateProvisioningTargetConfig(ctx, t); err != nil {
		return nil, err
	}
	if err := s.pools.CreateTarget(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) UpdateProvisioningTarget(ctx context.Context, id int64, fields map[string]any) error {
	if s.pools == nil {
		return fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	fields = normalizeProvisioningTargetFields(fields)
	if len(fields) > 0 {
		target, err := s.pools.GetTarget(ctx, id)
		if err != nil {
			return err
		}
		if target == nil {
			return gorm.ErrRecordNotFound
		}
		overlayProvisioningTarget(target, fields)
		if err := s.validateProvisioningTargetConfig(ctx, target); err != nil {
			return err
		}
	}
	return s.pools.UpdateTarget(ctx, id, fields)
}

func (s *Service) DeleteProvisioningTarget(ctx context.Context, id int64) error {
	if s.pools == nil {
		return fmt.Errorf("%w: provisioning pools are not configured", ErrInvalidInput)
	}
	return s.pools.DeleteTarget(ctx, id)
}

// ---- Purchase -------------------------------------------------------------

// PurchaseInput is the API-side shape of a purchase request. Idempotency
// key is required — the caller (handler) generates one and retries are
// safe.
type PurchaseInput struct {
	UserID              int64
	PlanID              int64
	IdempotencyKey      string
	NodeID              int64 // optional when the plan has a provisioning pool
	InboundTag          string
	AllowExplicitTarget bool // internal/admin/auto-renew only
}

type provisioningTarget struct {
	NodeID     int64
	InboundTag string
}

type provisionability struct {
	Target provisioningTarget
	Reason string
}

// Purchase runs the full purchase flow. Behaviour:
//   - empty idempotency key  → ErrInvalidInput
//   - dupe idempotency key   → returns the original order unchanged
//   - plan missing/disabled  → ErrPlanNotFound / ErrPlanDisabled
//   - insufficient balance   → order recorded as failed → ErrInsufficientBalance
//   - provisioning failure   → balance refunded, order marked refunded
//   - success                → balance charged, ownership upserted,
//     order marked completed
//
// Emits order.created on first persistence, order.completed on
// success, order.failed when provisioning fails post-charge.
func (s *Service) Purchase(ctx context.Context, in PurchaseInput) (*model.Order, error) {
	if in.IdempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", ErrInvalidInput)
	}
	// Idempotency short-circuit.
	if existing, err := s.orders.GetByUserAndIdempotencyKey(ctx, in.UserID, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, validateIdempotentPurchase(existing, in, model.PaymentMethodBalance)
	}
	if existing, err := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("%w: idempotency_key is already used", ErrIdempotencyConflict)
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
	if err := s.ensureNewUserPlanAllowed(ctx, user.ID, plan.ID, in.AllowExplicitTarget); err != nil {
		return nil, err
	}
	target, err := s.resolveProvisioningTarget(ctx, plan, user.ID, in.NodeID, in.InboundTag, in.AllowExplicitTarget)
	if err != nil {
		return nil, err
	}

	// Record the order in pending state up front so the idempotency
	// key is reserved (concurrent dupes will collide on the unique
	// index and the retry path will find the existing row).
	order := &model.Order{
		UserID:                 user.ID,
		PlanID:                 plan.ID,
		IdempotencyKey:         in.IdempotencyKey,
		PriceCents:             plan.PriceCents,
		Status:                 model.OrderStatusPending,
		PaymentMethod:          model.PaymentMethodBalance,
		ProvisioningNodeID:     &target.NodeID,
		ProvisioningInboundTag: target.InboundTag,
	}
	if err := s.orders.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", int32(target.NodeID), advisoryLockKey(target.InboundTag)).Error; err != nil {
			return fmt.Errorf("billing.Purchase: capacity lock: %w", err)
		}
		if err := s.validateProvisioningTargetNow(ctx, plan, user.ID, target, 0); err != nil {
			return err
		}
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("OrderRepo.Create: %w", err)
		}
		return nil
	}); err != nil {
		if isUniqueViolation(err) {
			// Concurrent dupe — fetch and return that one.
			if existing, gErr := s.orders.GetByUserAndIdempotencyKey(ctx, in.UserID, in.IdempotencyKey); gErr == nil && existing != nil {
				return existing, validateIdempotentPurchase(existing, in, model.PaymentMethodBalance)
			}
			if existing, gErr := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); gErr == nil && existing != nil {
				return nil, fmt.Errorf("%w: idempotency_key is already used", ErrIdempotencyConflict)
			}
		}
		return nil, err
	}
	s.bus.PublishType(event.OrderCreated, payload.Order{
		OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
	})

	// Pre-flight: verify the resolved (NodeID, InboundTag) target is
	// actually provisionable before charging the user. Catches:
	//  - node is disabled / missing
	//  - inbound tag has been deleted on the panel
	//  - inbound is disabled (operator paused it)
	//  - WG inbound but WG_MASTER_KEY not set on this dashboard
	// All of these would otherwise cause a charge → provision-fail →
	// refund pair, leaving paired ledger entries for what's
	// effectively a no-op. Reject up front instead.
	if err := s.validateProvisioningTargetNow(ctx, plan, user.ID, target, order.ID); err != nil {
		_ = s.orders.MarkFailed(ctx, order.ID, "inbound preflight: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
			Reason: "inbound_unavailable",
		})
		return order, fmt.Errorf("billing.Purchase: preflight: %w", err)
	}

	// Charge.
	if _, have, err := s.users.ChargeBalanceIfEnough(ctx, user.ID, plan.PriceCents, model.BalanceReasonOrderCharge, "", &order.ID); err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			_ = s.orders.MarkFailed(ctx, order.ID, "insufficient balance")
			s.bus.PublishType(event.OrderFailed, payload.Order{
				OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
				Reason: "insufficient_balance",
			})
			return order, fmt.Errorf("%w: have=%d, need=%d", ErrInsufficientBalance, have, plan.PriceCents)
		}
		_ = s.orders.MarkFailed(ctx, order.ID, "charge failed: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents, Reason: "charge_failed",
		})
		return order, fmt.Errorf("billing.Purchase: charge: %w", err)
	}

	// Provision.
	planID := plan.ID
	ownership, err := s.client.ProvisionClient(ctx, user.ID, target.NodeID, target.InboundTag, client.PlanParams{
		PlanID:            &planID,
		DurationDays:      plan.DurationDays,
		TrafficLimitBytes: plan.TrafficLimitBytes,
		IPLimit:           plan.IPLimit,
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
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents, Reason: "provisioning_failed",
		})
		return order, fmt.Errorf("billing.Purchase: provision: %w", err)
	}

	if err := s.orders.MarkCompleted(ctx, order.ID, ownership.ID); err != nil {
		s.log.Error("mark completed failed", slog.Int64("order_id", order.ID), slog.String("error", err.Error()))
	}
	s.bus.PublishType(event.OrderCompleted, payload.Order{
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

// ---- Payment-gateway purchase ---------------------------------------------

// PurchaseViaPaymentInput is the same shape as PurchaseInput plus
// the chosen Provider. The balance is NOT debited; the order is
// held at payment_pending until the gateway confirms.
type PurchaseViaPaymentInput struct {
	UserID              int64
	PlanID              int64
	IdempotencyKey      string
	NodeID              int64 // optional when the plan has a provisioning pool
	InboundTag          string
	AllowExplicitTarget bool
	Provider            string // "alipay", "stripe", ...
}

// PurchaseViaPayment creates a payment_pending order and asks the
// chosen gateway to create a payment session. Returns the order
// with PaymentTargetURL + PaymentExpiresAt populated so the handler
// can hand the redirect/QR back to the portal.
func (s *Service) PurchaseViaPayment(ctx context.Context, in PurchaseViaPaymentInput) (*model.Order, error) {
	if in.IdempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", ErrInvalidInput)
	}
	if s.gateways == nil {
		return nil, payment.ErrUnknownProvider
	}
	gw, err := s.gateways.Get(in.Provider)
	if err != nil {
		return nil, err
	}

	// Idempotency short-circuit — return the existing order so a
	// retried POST gets back the same QR.
	if existing, err := s.orders.GetByUserAndIdempotencyKey(ctx, in.UserID, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, validateIdempotentPaymentPurchase(existing, in)
	}
	if existing, err := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("%w: idempotency_key is already used", ErrIdempotencyConflict)
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
	user, err := s.users.Get(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if err := s.ensureNewUserPlanAllowed(ctx, user.ID, plan.ID, in.AllowExplicitTarget); err != nil {
		return nil, err
	}
	target, err := s.resolveProvisioningTarget(ctx, plan, user.ID, in.NodeID, in.InboundTag, in.AllowExplicitTarget)
	if err != nil {
		return nil, err
	}

	// Capture the provisioning target on the order so the
	// confirmation path can create the client without asking the
	// caller again. Earlier versions stuffed this into ErrorMessage
	// but that overloaded the column.
	order := &model.Order{
		UserID:                 user.ID,
		PlanID:                 plan.ID,
		IdempotencyKey:         in.IdempotencyKey,
		PriceCents:             plan.PriceCents,
		Status:                 model.OrderStatusPaymentPending,
		PaymentMethod:          in.Provider,
		ProvisioningNodeID:     &target.NodeID,
		ProvisioningInboundTag: target.InboundTag,
	}
	if err := s.orders.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", int32(target.NodeID), advisoryLockKey(target.InboundTag)).Error; err != nil {
			return fmt.Errorf("billing.PurchaseViaPayment: capacity lock: %w", err)
		}
		if err := s.validateProvisioningTargetNow(ctx, plan, user.ID, target, 0); err != nil {
			return err
		}
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("OrderRepo.Create: %w", err)
		}
		return nil
	}); err != nil {
		if isUniqueViolation(err) {
			if existing, gErr := s.orders.GetByUserAndIdempotencyKey(ctx, in.UserID, in.IdempotencyKey); gErr == nil && existing != nil {
				return existing, validateIdempotentPaymentPurchase(existing, in)
			}
			if existing, gErr := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); gErr == nil && existing != nil {
				return nil, fmt.Errorf("%w: idempotency_key is already used", ErrIdempotencyConflict)
			}
		}
		return nil, err
	}
	s.bus.PublishType(event.OrderCreated, payload.Order{
		OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
	})

	// Ask the gateway for a QR.
	res, err := gw.CreatePayment(ctx, order, plan.Name)
	if err != nil {
		// Couldn't reach the gateway — mark the order failed so the
		// user can retry (the idempotency_key has been consumed).
		_ = s.orders.AdvanceStatusGuarded(ctx, order.ID, model.OrderStatusPaymentPending, model.OrderStatusPaymentFailed)
		s.bus.PublishType(event.OrderPaymentFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
			Reason: "gateway_create_failed: " + err.Error(),
		})
		return order, fmt.Errorf("billing.PurchaseViaPayment: %w", err)
	}

	if err := s.orders.SetPaymentMetadata(ctx, order.ID, res.ProviderOrderID, res.TargetURL, res.ExpiresAt); err != nil {
		return order, fmt.Errorf("billing.PurchaseViaPayment: persist metadata: %w", err)
	}
	order.PaymentProviderOrderID = res.ProviderOrderID
	order.PaymentTargetURL = res.TargetURL
	order.PaymentExpiresAt = &res.ExpiresAt
	return order, nil
}

// ConfirmPayment is called by the notify endpoint AND the poll job.
// Idempotent: if the order is already past payment_pending, returns
// the current order without doing anything. On the winning call,
// transitions payment_pending → completed via paid + provisions.
func (s *Service) ConfirmPayment(ctx context.Context, providerOrderID string) (*model.Order, error) {
	if providerOrderID == "" {
		return nil, fmt.Errorf("%w: provider_order_id required", ErrInvalidInput)
	}
	order, err := s.orders.GetByProviderOrderID(ctx, providerOrderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != model.OrderStatusPaymentPending {
		// Already advanced — idempotent no-op.
		return order, nil
	}

	// Guarded transition: if a concurrent caller already advanced
	// the order, AdvanceStatusGuarded returns ErrRecordNotFound and
	// we re-read + return without re-provisioning.
	if err := s.orders.AdvanceStatusGuarded(ctx, order.ID, model.OrderStatusPaymentPending, model.OrderStatusPaid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.orders.Get(ctx, order.ID)
		}
		return nil, fmt.Errorf("billing.ConfirmPayment: advance to paid: %w", err)
	}
	s.bus.PublishType(event.OrderPaymentConfirmed, payload.Order{
		OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents,
	})

	plan, err := s.plans.Get(ctx, order.PlanID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		// Plan deleted between purchase + confirm. Mark refunded so
		// admin can chase it — provisioning would crash without a
		// plan to source duration/traffic from.
		_ = s.orders.MarkRefunded(ctx, order.ID, "plan deleted before confirmation")
		return order, ErrPlanNotFound
	}

	if order.ProvisioningNodeID == nil {
		_ = s.orders.MarkRefunded(ctx, order.ID, "missing provisioning target")
		return order, fmt.Errorf("billing.ConfirmPayment: order has no provisioning_node_id")
	}
	target := provisioningTarget{NodeID: *order.ProvisioningNodeID, InboundTag: order.ProvisioningInboundTag}
	if err := s.validateProvisioningTargetNow(ctx, plan, order.UserID, target, order.ID); err != nil {
		_ = s.orders.MarkRefunded(ctx, order.ID, "provisioning target unavailable: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents,
			Reason: "provisioning_target_unavailable_after_payment",
		})
		return order, fmt.Errorf("billing.ConfirmPayment: target unavailable: %w", err)
	}
	planID := plan.ID
	ownership, err := s.client.ProvisionClient(ctx, order.UserID, target.NodeID, target.InboundTag, client.PlanParams{
		PlanID:            &planID,
		DurationDays:      plan.DurationDays,
		TrafficLimitBytes: plan.TrafficLimitBytes,
		IPLimit:           plan.IPLimit,
	})
	if err != nil {
		_ = s.orders.MarkRefunded(ctx, order.ID, "provisioning failed: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents,
			Reason: "provisioning_failed_after_payment",
		})
		return order, fmt.Errorf("billing.ConfirmPayment: provision: %w", err)
	}

	if err := s.orders.MarkCompleted(ctx, order.ID, ownership.ID); err != nil {
		s.log.Error("mark completed failed after payment",
			slog.Int64("order_id", order.ID), slog.String("error", err.Error()))
	}
	s.bus.PublishType(event.OrderCompleted, payload.Order{
		OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents,
	})
	return s.orders.Get(ctx, order.ID)
}

// FailPayment marks a payment_pending order as payment_failed. Used
// by the poll job when the gateway reports a closed/cancelled trade.
func (s *Service) FailPayment(ctx context.Context, providerOrderID, reason string) error {
	order, err := s.orders.GetByProviderOrderID(ctx, providerOrderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrOrderNotFound
	}
	if order.Status != model.OrderStatusPaymentPending {
		return nil
	}
	if err := s.orders.AdvanceStatusGuarded(ctx, order.ID, model.OrderStatusPaymentPending, model.OrderStatusPaymentFailed); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	s.bus.PublishType(event.OrderPaymentFailed, payload.Order{
		OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents, Reason: reason,
	})
	return nil
}

// ExpirePayment marks a payment_pending order as payment_expired.
// Same guard as FailPayment so a concurrent ConfirmPayment can't be
// clobbered.
func (s *Service) ExpirePayment(ctx context.Context, orderID int64) error {
	if err := s.orders.AdvanceStatusGuarded(ctx, orderID, model.OrderStatusPaymentPending, model.OrderStatusPaymentExpired); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if order, err := s.orders.Get(ctx, orderID); err == nil && order != nil {
		s.bus.PublishType(event.OrderPaymentExpired, payload.Order{
			OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents,
		})
	}
	return nil
}

// ListPendingPayments exposes the open payment_pending orders to
// the payment-poll job. maxAge filters out orders past expiry so
// the job doesn't keep poking abandoned QRs.
func (s *Service) ListPendingPayments(ctx context.Context, maxAge time.Duration) ([]model.Order, error) {
	return s.orders.ListPaymentPending(ctx, maxAge)
}

// ListExpiredPendingPayments returns payment_pending orders past their
// explicit payment expiry, falling back to created_at for older rows.
func (s *Service) ListExpiredPendingPayments(ctx context.Context, now, fallbackCutoff time.Time) ([]model.Order, error) {
	return s.orders.ListExpiredPending(ctx, now, fallbackCutoff)
}

// ---- Order listing --------------------------------------------------------

// GetOrderForUser returns one order, refusing to return another
// user's order. Used by the portal poll endpoint that flips the
// alipay QR modal to "支付成功" when status advances to completed.
func (s *Service) GetOrderForUser(ctx context.Context, userID, orderID int64) (*model.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil || order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

func (s *Service) ListOrdersByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Order, error) {
	return s.orders.ListByUser(ctx, userID, limit, offset)
}

func (s *Service) ListOrdersAdmin(ctx context.Context, filter repository.OrderFilter, limit, offset int) ([]model.Order, error) {
	return s.orders.ListAdmin(ctx, filter, limit, offset)
}

// RefundOrder is the admin-initiated manual refund flow. Credits
// the user's balance, marks the order refunded, and emits
// OrderRefunded so notification channels can fan out. Does NOT
// touch the panel-side client — if the admin wants to disable
// the underlying access, they use the client-management UI
// separately. This is intentional: refund ≠ revoke (sometimes
// an op refunds a partial charge and leaves access in place).
//
// Refundable states:
//   - completed: full refund, credit = PriceCents
//   - paid: payment confirmed but provisioning hadn't run — same
//     credit, since the user paid PriceCents
//
// Already-refunded / failed / pending orders return
// ErrInvalidOrderState. Idempotent: a repeat call on an
// already-refunded order returns ErrInvalidOrderState.
func (s *Service) RefundOrder(ctx context.Context, orderID int64, reason string) (*model.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	switch order.Status {
	case model.OrderStatusCompleted, model.OrderStatusPaid:
		// refundable
	default:
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidOrderState, order.Status)
	}

	// Credit the user.
	if _, err := s.users.AdjustBalance(ctx, order.UserID, order.PriceCents, model.BalanceReasonOrderRefund, reason, &order.ID); err != nil {
		return nil, fmt.Errorf("billing.RefundOrder: credit user: %w", err)
	}
	if err := s.orders.MarkRefunded(ctx, order.ID, reason); err != nil {
		return nil, fmt.Errorf("billing.RefundOrder: mark refunded: %w", err)
	}
	s.bus.PublishType(event.OrderRefunded, payload.Order{
		OrderID: order.ID, UserID: order.UserID, PlanID: order.PlanID, PriceCents: order.PriceCents, Reason: "admin_refund",
	})
	s.log.Info("admin refunded order",
		slog.Int64("order_id", order.ID),
		slog.Int64("user_id", order.UserID),
		slog.Int64("amount", order.PriceCents),
		slog.String("reason", reason),
	)
	return s.orders.Get(ctx, order.ID)
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
	if p.IPLimit < 0 {
		return fmt.Errorf("%w: ip_limit must be >= 0", ErrInvalidInput)
	}
	return nil
}

func normalizePlanFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		switch k {
		case "name":
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		case "description", "duration_days", "traffic_limit_bytes", "price_cents", "ip_limit", "enabled", "provisioning_pool_id":
			out[k] = v
		}
	}
	return out
}

func normalizeInboundTemplate(t *model.InboundTemplate) error {
	t.Name = strings.TrimSpace(t.Name)
	t.Description = strings.TrimSpace(t.Description)
	t.Protocol = strings.ToLower(strings.TrimSpace(t.Protocol))
	t.Remark = strings.TrimSpace(t.Remark)
	t.Listen = strings.TrimSpace(t.Listen)
	t.TrafficReset = strings.TrimSpace(t.TrafficReset)
	if t.TrafficReset == "" {
		t.TrafficReset = "never"
	}
	if t.Name == "" {
		return fmt.Errorf("%w: template name is required", ErrInvalidInput)
	}
	if t.Protocol == "" {
		return fmt.Errorf("%w: template protocol is required", ErrInvalidInput)
	}
	if t.Total < 0 || t.ExpiryTime < 0 {
		return fmt.Errorf("%w: template total and expiry_time must be >= 0", ErrInvalidInput)
	}
	if err := ensureJSONString(&t.Settings, "{}"); err != nil {
		return fmt.Errorf("%w: settings must be valid JSON: %v", ErrInvalidInput, err)
	}
	if err := ensureJSONString(&t.StreamSettings, "{}"); err != nil {
		return fmt.Errorf("%w: stream_settings must be valid JSON: %v", ErrInvalidInput, err)
	}
	if err := ensureJSONString(&t.Sniffing, "{}"); err != nil {
		return fmt.Errorf("%w: sniffing must be valid JSON: %v", ErrInvalidInput, err)
	}
	// settings.clients[] is intentionally NOT required here: a template
	// is the wire-shape preset for "build me an inbound", not a real
	// inbound. Clients are populated when an actual inbound is created
	// on a node — or, more commonly, when a user purchases and the
	// service calls ProvisionClient on an existing inbound.
	return nil
}

func normalizeInboundTemplateFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		switch k {
		case "name":
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		case "description", "remark", "listen", "trafficReset", "traffic_reset":
			col := k
			if k == "trafficReset" {
				col = "traffic_reset"
			}
			out[col] = strings.TrimSpace(fmt.Sprint(v))
		case "protocol":
			out[k] = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		case "settings", "sniffing":
			if s, err := normalizedJSONString(fmt.Sprint(v), "{}"); err == nil {
				out[k] = s
			}
		case "streamSettings", "stream_settings":
			if s, err := normalizedJSONString(fmt.Sprint(v), "{}"); err == nil {
				out["stream_settings"] = s
			}
		case "enabled", "total", "expiryTime", "expiry_time":
			col := k
			if k == "expiryTime" {
				col = "expiry_time"
			}
			out[col] = v
		}
	}
	return out
}

func normalizeProvisioningPool(p *model.ProvisioningPool) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	if p.Name == "" {
		return fmt.Errorf("%w: pool name is required", ErrInvalidInput)
	}
	for i := range p.AllowedProtocols {
		p.AllowedProtocols[i] = strings.ToLower(strings.TrimSpace(p.AllowedProtocols[i]))
	}
	return nil
}

func normalizeProvisioningTarget(t *model.ProvisioningPoolTarget) error {
	t.InboundTag = strings.TrimSpace(t.InboundTag)
	t.Protocol = strings.ToLower(strings.TrimSpace(t.Protocol))
	if t.PoolID <= 0 || t.NodeID <= 0 || t.InboundTag == "" {
		return fmt.Errorf("%w: pool_id, node_id and inbound_tag are required", ErrInvalidInput)
	}
	if t.MaxClients < 0 {
		return fmt.Errorf("%w: max_clients must be >= 0", ErrInvalidInput)
	}
	if t.Priority < 0 {
		return fmt.Errorf("%w: priority must be >= 0", ErrInvalidInput)
	}
	return nil
}

func normalizeProvisioningPoolFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		switch k {
		case "name":
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		case "description":
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		case "allowed_protocols":
			out[k] = normalizeStringSlice(v)
		case "enabled":
			out[k] = v
		}
	}
	return out
}

func normalizeProvisioningTargetFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		switch k {
		case "inbound_tag":
			out[k] = strings.TrimSpace(fmt.Sprint(v))
		case "protocol":
			out[k] = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		case "node_id", "max_clients", "priority", "enabled":
			out[k] = v
		}
	}
	return out
}

func (s *Service) validateProvisioningTargetConfig(ctx context.Context, t *model.ProvisioningPoolTarget) error {
	in, err := s.client.PreflightInbound(ctx, t.NodeID, t.InboundTag)
	if err != nil {
		return fmt.Errorf("%w: target preflight: %v", ErrInvalidInput, err)
	}
	protocol := strings.ToLower(strings.TrimSpace(in.Protocol))
	if t.Protocol == "" {
		t.Protocol = protocol
		return nil
	}
	if strings.ToLower(strings.TrimSpace(t.Protocol)) != protocol {
		return fmt.Errorf("%w: target protocol %q does not match inbound protocol %q", ErrInvalidInput, t.Protocol, protocol)
	}
	t.Protocol = protocol
	return nil
}

func overlayProvisioningTarget(t *model.ProvisioningPoolTarget, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "node_id":
			if n, ok := numericInt64(v); ok {
				t.NodeID = n
			}
		case "inbound_tag":
			t.InboundTag = strings.TrimSpace(fmt.Sprint(v))
		case "protocol":
			t.Protocol = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		case "max_clients":
			if n, ok := numericInt(v); ok {
				t.MaxClients = n
			}
		case "priority":
			if n, ok := numericInt(v); ok {
				t.Priority = n
			}
		case "enabled":
			if b, ok := v.(bool); ok {
				t.Enabled = b
			}
		}
	}
}

func numericInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case float32:
		return int(x), true
	default:
		var n int
		if _, err := fmt.Sscan(strings.TrimSpace(fmt.Sprint(v)), &n); err == nil {
			return n, true
		}
		return 0, false
	}
}

func numericInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(x), true
	case float32:
		return int64(x), true
	default:
		var n int64
		if _, err := fmt.Sscan(strings.TrimSpace(fmt.Sprint(v)), &n); err == nil {
			return n, true
		}
		return 0, false
	}
}

func (s *Service) resolveProvisioningTarget(ctx context.Context, plan *model.Plan, userID, nodeID int64, inboundTag string, allowExplicit bool) (provisioningTarget, error) {
	inboundTag = strings.TrimSpace(inboundTag)
	if allowExplicit && nodeID > 0 && inboundTag != "" {
		return provisioningTarget{NodeID: nodeID, InboundTag: inboundTag}, nil
	}
	if plan.ProvisioningPoolID == nil {
		return provisioningTarget{}, fmt.Errorf("%w: plan has no provisioning_pool_id", ErrNoProvisioningTarget)
	}
	if s.pools == nil {
		return provisioningTarget{}, fmt.Errorf("%w: provisioning pools are not configured", ErrNoProvisioningTarget)
	}
	candidates, err := s.pools.ListCandidatesForUser(ctx, *plan.ProvisioningPoolID, userID)
	if err != nil {
		return provisioningTarget{}, err
	}
	if target, ok := s.firstProvisionableCandidate(ctx, *plan.ProvisioningPoolID, candidates); ok {
		return target, nil
	}
	return provisioningTarget{}, fmt.Errorf("%w: pool_id=%d", ErrNoProvisioningTarget, *plan.ProvisioningPoolID)
}

func (s *Service) firstProvisionableCandidate(ctx context.Context, poolID int64, candidates []repository.Candidate) (provisioningTarget, bool) {
	for _, c := range candidates {
		pv := s.checkCandidateProvisionability(ctx, c)
		if pv.Reason != "" {
			s.log.Warn("skip provisioning candidate after preflight",
				slog.Int64("pool_id", poolID),
				slog.Int64("node_id", pv.Target.NodeID),
				slog.String("inbound", pv.Target.InboundTag),
				slog.String("err", pv.Reason),
			)
			continue
		}
		return pv.Target, true
	}
	return provisioningTarget{}, false
}

func (s *Service) validateProvisioningTargetNow(ctx context.Context, plan *model.Plan, userID int64, target provisioningTarget, excludeOrderID int64) error {
	target.InboundTag = strings.TrimSpace(target.InboundTag)
	if target.NodeID <= 0 || target.InboundTag == "" {
		return fmt.Errorf("%w: missing provisioning target", ErrNoProvisioningTarget)
	}
	if plan.ProvisioningPoolID == nil {
		if err := s.client.PreflightProvision(ctx, target.NodeID, target.InboundTag); err != nil {
			return err
		}
		return nil
	}
	if s.pools == nil {
		return fmt.Errorf("%w: provisioning pools are not configured", ErrNoProvisioningTarget)
	}
	candidates, err := s.pools.ListCandidatesForUserExcludingOrder(ctx, *plan.ProvisioningPoolID, userID, excludeOrderID)
	if err != nil {
		return err
	}
	for _, c := range candidates {
		if c.NodeID != target.NodeID || c.InboundTag != target.InboundTag {
			continue
		}
		if pv := s.checkCandidateProvisionability(ctx, c); pv.Reason != "" {
			return fmt.Errorf("%w: %s", ErrNoProvisioningTarget, pv.Reason)
		}
		return nil
	}
	return fmt.Errorf("%w: target disabled, full, or no longer in plan pool", ErrNoProvisioningTarget)
}

func (s *Service) checkCandidateProvisionability(ctx context.Context, c repository.Candidate) provisionability {
	target := provisioningTarget{NodeID: c.NodeID, InboundTag: c.InboundTag}
	if c.MaxClients > 0 && c.UsedClients >= c.MaxClients {
		return provisionability{Target: target, Reason: fmt.Sprintf("capacity full (%d/%d)", c.UsedClients, c.MaxClients)}
	}
	in, err := s.client.PreflightInbound(ctx, c.NodeID, c.InboundTag)
	if err != nil {
		return provisionability{Target: target, Reason: err.Error()}
	}
	if !protocolAllowed(in.Protocol, c.AllowedProtocols) {
		return provisionability{Target: target, Reason: fmt.Sprintf("protocol %q is not allowed", in.Protocol)}
	}
	return provisionability{Target: target}
}

func normalizeStringSlice(v any) model.StringSlice {
	out := model.StringSlice{}
	appendOne := func(raw any) {
		s := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
		if s == "" {
			return
		}
		for _, existing := range out {
			if existing == s {
				return
			}
		}
		out = append(out, s)
	}
	switch vv := v.(type) {
	case model.StringSlice:
		for _, item := range vv {
			appendOne(item)
		}
	case []string:
		for _, item := range vv {
			appendOne(item)
		}
	case []any:
		for _, item := range vv {
			appendOne(item)
		}
	case nil:
	default:
		for _, item := range strings.Split(fmt.Sprint(vv), ",") {
			appendOne(item)
		}
	}
	return out
}

func ensureJSONString(target *string, fallback string) error {
	s, err := normalizedJSONString(*target, fallback)
	if err != nil {
		return err
	}
	*target = s
	return nil
}

func normalizedJSONString(raw string, fallback string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fallback
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func protocolAllowed(protocol string, allowed model.StringSlice) bool {
	if len(allowed) == 0 {
		return true
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	for _, p := range allowed {
		if strings.ToLower(strings.TrimSpace(p)) == protocol {
			return true
		}
	}
	return false
}

func (s *Service) newUserPlanPolicy(ctx context.Context, userID int64) (map[int64]bool, bool, error) {
	if s.settings == nil || s.orders == nil || userID <= 0 {
		return nil, false, nil
	}
	raw, err := s.settings.GetString(ctx, model.SettingNewUserPlanIDs, "")
	if err != nil {
		return nil, false, err
	}
	allowed := parsePlanIDSet(raw)
	if len(allowed) == 0 {
		return nil, false, nil
	}
	hasHistory, err := s.orders.UserHasAccessHistory(ctx, userID)
	if err != nil || hasHistory {
		return nil, false, err
	}
	return allowed, true, nil
}

func (s *Service) ensureNewUserPlanAllowed(ctx context.Context, userID, planID int64, bypass bool) error {
	if bypass || planID <= 0 {
		return nil
	}
	allowed, restricted, err := s.newUserPlanPolicy(ctx, userID)
	if err != nil || !restricted {
		return err
	}
	if allowed[planID] {
		return nil
	}
	return fmt.Errorf("%w: plan is not available for new users", ErrPlanDisabled)
}

func parsePlanIDSet(raw string) map[int64]bool {
	out := map[int64]bool{}
	for _, part := range strings.Split(raw, ",") {
		var id int64
		if _, err := fmt.Sscan(strings.TrimSpace(part), &id); err == nil && id > 0 {
			out[id] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func advisoryLockKey(s string) int32 {
	var h uint32 = 2166136261
	for _, b := range []byte(s) {
		h ^= uint32(b)
		h *= 16777619
	}
	return int32(h)
}

func validateIdempotentPurchase(existing *model.Order, in PurchaseInput, method string) error {
	if existing.UserID != in.UserID ||
		existing.PlanID != in.PlanID ||
		existing.PaymentMethod != method {
		return fmt.Errorf("%w: idempotency_key does not match original purchase", ErrIdempotencyConflict)
	}
	if in.NodeID > 0 || strings.TrimSpace(in.InboundTag) != "" {
		if !sameProvisioningTarget(existing, in.NodeID, in.InboundTag) {
			return fmt.Errorf("%w: idempotency_key does not match original purchase", ErrIdempotencyConflict)
		}
	}
	return nil
}

func validateIdempotentPaymentPurchase(existing *model.Order, in PurchaseViaPaymentInput) error {
	if existing.UserID != in.UserID ||
		existing.PlanID != in.PlanID ||
		existing.PaymentMethod != in.Provider {
		return fmt.Errorf("%w: idempotency_key does not match original purchase", ErrIdempotencyConflict)
	}
	if in.NodeID > 0 || strings.TrimSpace(in.InboundTag) != "" {
		if !sameProvisioningTarget(existing, in.NodeID, in.InboundTag) {
			return fmt.Errorf("%w: idempotency_key does not match original purchase", ErrIdempotencyConflict)
		}
	}
	return nil
}

func sameProvisioningTarget(order *model.Order, nodeID int64, inboundTag string) bool {
	return order.ProvisioningNodeID != nil &&
		*order.ProvisioningNodeID == nodeID &&
		order.ProvisioningInboundTag == inboundTag
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}
