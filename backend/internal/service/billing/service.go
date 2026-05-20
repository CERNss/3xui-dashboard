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
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/client"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// Errors callers branch on.
var (
	ErrInsufficientBalance = errors.New("billing: insufficient balance")
	ErrUserNotFound        = errors.New("billing: user not found")
	ErrPlanNotFound        = errors.New("billing: plan not found")
	ErrPlanDisabled        = errors.New("billing: plan disabled")
	ErrInvalidInput        = errors.New("billing: invalid input")
	ErrOrderNotFound       = errors.New("billing: order not found")
	// ErrInvalidOrderState fires when an admin tries to refund an
	// order that isn't in a refundable state (e.g. already refunded,
	// or still pending payment).
	ErrInvalidOrderState   = errors.New("billing: order not in refundable state")
)

// Service composes the repos + the client provisioning service.
type Service struct {
	plans     *repository.PlanRepo
	orders    *repository.OrderRepo
	users     *repository.UserRepo
	client    *client.Service
	bus       *event.Bus
	gateways  *payment.Registry
	log       *slog.Logger
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
	s.bus.PublishType(event.OrderCreated, payload.Order{
		OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
	})

	if user.BalanceCents < plan.PriceCents {
		_ = s.orders.MarkFailed(ctx, order.ID, "insufficient balance")
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
			Reason: "insufficient_balance",
		})
		return order, fmt.Errorf("%w: have=%d, need=%d", ErrInsufficientBalance, user.BalanceCents, plan.PriceCents)
	}

	// Pre-flight: verify the resolved (NodeID, InboundTag) target is
	// actually provisionable before charging the user. Catches:
	//  - node is disabled / missing
	//  - inbound tag has been deleted on the panel
	//  - inbound is disabled (operator paused it)
	//  - WG inbound but WG_MASTER_KEY not set on this dashboard
	// All of these would otherwise cause a charge → provision-fail →
	// refund pair, leaving paired ledger entries for what's
	// effectively a no-op. Reject up front instead.
	if err := s.client.PreflightProvision(ctx, in.NodeID, in.InboundTag); err != nil {
		_ = s.orders.MarkFailed(ctx, order.ID, "inbound preflight: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
			OrderID: order.ID, UserID: user.ID, PlanID: plan.ID, PriceCents: plan.PriceCents,
			Reason: "inbound_unavailable",
		})
		return order, fmt.Errorf("billing.Purchase: preflight: %w", err)
	}

	// Charge.
	if _, err := s.users.AdjustBalance(ctx, user.ID, -plan.PriceCents, model.BalanceReasonOrderCharge, "", &order.ID); err != nil {
		_ = s.orders.MarkFailed(ctx, order.ID, "charge failed: "+err.Error())
		s.bus.PublishType(event.OrderFailed, payload.Order{
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
	UserID         int64
	PlanID         int64
	IdempotencyKey string
	NodeID         int64
	InboundTag     string
	Provider       string // "alipay", "stripe", ...
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
	if existing, err := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
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

	// Capture the provisioning target on the order so the
	// confirmation path can create the client without asking the
	// caller again. Dedicated columns (migration 0007) — earlier
	// versions stuffed this into ErrorMessage but that overloaded
	// the column.
	nodeID := in.NodeID
	order := &model.Order{
		UserID:                 user.ID,
		PlanID:                 plan.ID,
		IdempotencyKey:         in.IdempotencyKey,
		PriceCents:             plan.PriceCents,
		Status:                 model.OrderStatusPaymentPending,
		PaymentMethod:          in.Provider,
		ProvisioningNodeID:     &nodeID,
		ProvisioningInboundTag: in.InboundTag,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		if isUniqueViolation(err) {
			if existing, gErr := s.orders.GetByIdempotencyKey(ctx, in.IdempotencyKey); gErr == nil && existing != nil {
				return existing, nil
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
	planID := plan.ID
	ownership, err := s.client.ProvisionClient(ctx, order.UserID, *order.ProvisioningNodeID, order.ProvisioningInboundTag, client.PlanParams{
		PlanID:            &planID,
		DurationDays:      plan.DurationDays,
		TrafficLimitBytes: plan.TrafficLimitBytes,
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

// ListExpiredPendingPayments returns payment_pending orders past
// `cutoff` so the poll job can mark them payment_expired.
func (s *Service) ListExpiredPendingPayments(ctx context.Context, cutoff time.Time) ([]model.Order, error) {
	return s.orders.ListExpiredPending(ctx, cutoff)
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
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}


