package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

// purchaseViaStripe creates a payment_pending order through the
// full purchase API and returns the order ID + stripe session ID.
// Shared by the poll-job tests below.
func purchaseViaStripe(t *testing.T, h *harness, planID int64, userTok, idemKey string) (orderID int64, sessionID string) {
	t.Helper()
	var order struct {
		ID                     int64  `json:"id"`
		PaymentProviderOrderID string `json:"payment_provider_order_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase/stripe", token: userTok,
		body: map[string]any{"plan_id": planID, "idempotency_key": idemKey, "node_id": 1, "inbound_tag": "vless-1"},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: status=%d", got)
	}
	return order.ID, order.PaymentProviderOrderID
}

// TestPaymentPoll_StripePaid_Confirms exercises the failsafe path
// where Stripe's webhook never arrives (NAT, transient outage) but
// trade.query reveals the user did pay. The poll job MUST advance
// the order to completed.
//
// This is the headline behavior of PaymentPollJob — the previous
// constructor-only tests left it completely unverified.
func TestPaymentPoll_StripePaid_Confirms(t *testing.T) {
	mock := newMockStripeHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	orderID, sessionID := purchaseViaStripe(t, h, planID, userTok, "poll-paid-key")

	// User pays in the Stripe checkout — mock marks session as paid.
	// In production this would also trigger a webhook; in this test
	// we ONLY rely on the poll job to detect the state change.
	mock.stripe.SetPaid(sessionID)

	// Trigger the poll cycle.
	h.app.PaymentPollJob.RunOnce(context.Background())

	final := orderRow(t, h, orderID)
	if final.Status != "completed" {
		t.Errorf("poll should advance to completed; got %q", final.Status)
	}
	if final.ClientOwnershipID == nil {
		t.Error("client_ownership_id should be set after poll-driven confirm")
	}
}

// TestPaymentPoll_StillPending_Noop verifies the job doesn't
// prematurely advance an order whose Stripe session is still
// unpaid. Without this check, a polling bug could close an
// in-progress checkout.
func TestPaymentPoll_StillPending_Noop(t *testing.T) {
	mock := newMockStripeHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	orderID, _ := purchaseViaStripe(t, h, planID, userTok, "poll-pending-key")
	// Note: NOT calling SetPaid — session stays unpaid.

	h.app.PaymentPollJob.RunOnce(context.Background())

	final := orderRow(t, h, orderID)
	if final.Status != "payment_pending" {
		t.Errorf("unpaid session should leave order pending; got %q", final.Status)
	}
	if final.ClientOwnershipID != nil {
		t.Error("no client should be provisioned for unpaid session")
	}
}

// TestPaymentPoll_ExpiresOldOrders verifies the cleanup path: an
// abandoned QR / checkout older than the expiry window flips to
// payment_expired so the orders table doesn't accumulate stuck rows.
func TestPaymentPoll_ExpiresOldOrders(t *testing.T) {
	mock := newMockStripeHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	orderID, _ := purchaseViaStripe(t, h, planID, userTok, "poll-expire-key")

	// Move the explicit payment expiry into the past. Direct DB write
	// is the simplest path; the API doesn't expose time travel.
	if err := h.db.Model(&model.Order{}).
		Where("id = ?", orderID).
		Update("payment_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire order payment timestamp: %v", err)
	}

	h.app.PaymentPollJob.RunOnce(context.Background())

	final := orderRow(t, h, orderID)
	if final.Status != "payment_expired" {
		t.Errorf("backdated order should be marked expired; got %q", final.Status)
	}
	if final.ClientOwnershipID != nil {
		t.Error("expired order should NOT have client_ownership_id")
	}
}

// TestPaymentPoll_NoGateway_NoOp confirms the job is safe to run
// when no gateways are configured (operator running balance-only).
// Regression guard against a nil-deref in the gateway.Query loop.
//
// "Doesn't panic" is the entire contract — Go's test framework
// fails the test automatically if RunOnce panics, so no further
// assertion is needed.
func TestPaymentPoll_NoGateway_NoOp(t *testing.T) {
	h := setupHarness(t) // no payment-gateway opts
	h.app.PaymentPollJob.RunOnce(context.Background())
}
