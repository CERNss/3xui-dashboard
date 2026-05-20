package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// PaymentPollJob is the failsafe for payment-gateway confirmations.
// The notify endpoint handles the happy path; this job catches the
// 1-in-1000 dropped notify (NAT, transient outage on our side
// during the 26h alipay retry window, etc.) and also reaps
// payment_pending orders that the user just abandoned.
//
// Cadence: @every 30s. Each tick:
//
//   1. Find payment_pending orders < expiryWindow old, ask the
//      gateway what they look like, advance state accordingly.
//   2. Find payment_pending orders older than expiryWindow, mark
//      them payment_expired (alipay's QR has expired by then
//      anyway).
//
// Idempotent w.r.t. notify: ConfirmPayment uses a guarded status
// transition, so a race between this job and the notify handler
// results in exactly one of them advancing the order.
type PaymentPollJob struct {
	billing      *billing.Service
	gateways     *payment.Registry
	expiryWindow time.Duration
	log          *slog.Logger
}

// NewPaymentPollJob constructs the job. `expiryWindow` is the age
// past which a payment_pending order is considered abandoned (we
// recommend 15 minutes — long enough for a user to walk through
// the alipay app, short enough that orders don't sit in the table
// forever). Zero defaults to 15 minutes.
func NewPaymentPollJob(b *billing.Service, gw *payment.Registry, expiryWindow time.Duration, lg *slog.Logger) *PaymentPollJob {
	if expiryWindow <= 0 {
		expiryWindow = 15 * time.Minute
	}
	return &PaymentPollJob{
		billing:      b,
		gateways:     gw,
		expiryWindow: expiryWindow,
		log:          lg.With(slog.String("component", "job.payment_poll")),
	}
}

// RunOnce runs one poll cycle. Safe to call repeatedly — that's the
// whole point of having it; the scheduler triggers it on @every 30s.
// Signature matches scheduler.Add's `func(context.Context)` shape;
// internal errors are logged, not returned, so a transient outage
// in one tick doesn't block the next.
func (j *PaymentPollJob) RunOnce(ctx context.Context) {
	if j.gateways == nil {
		return // no payment providers configured; nothing to poll
	}
	if err := j.pollPending(ctx); err != nil {
		j.log.Error("pollPending failed", slog.String("error", err.Error()))
	}
	if err := j.expireOld(ctx); err != nil {
		j.log.Error("expireOld failed", slog.String("error", err.Error()))
	}
}

func (j *PaymentPollJob) pollPending(ctx context.Context) error {
	rows, err := j.billing.ListPendingPayments(ctx, j.expiryWindow)
	if err != nil {
		return err
	}
	for _, o := range rows {
		gw, err := j.gateways.Get(o.PaymentMethod)
		if err != nil {
			// Gateway was removed from config between order
			// creation and this poll — skip; admin can manually
			// reconcile from logs.
			continue
		}
		status, err := gw.Query(ctx, o.PaymentProviderOrderID)
		if err != nil {
			j.log.Warn("gateway query failed",
				slog.Int64("order_id", o.ID),
				slog.String("provider", o.PaymentMethod),
				slog.String("error", err.Error()),
			)
			continue
		}
		switch status {
		case payment.StatusPaid:
			if _, err := j.billing.ConfirmPayment(ctx, o.PaymentProviderOrderID); err != nil {
				j.log.Error("ConfirmPayment via poll failed",
					slog.Int64("order_id", o.ID), slog.String("error", err.Error()))
			}
		case payment.StatusFailed:
			if err := j.billing.FailPayment(ctx, o.PaymentProviderOrderID, "gateway reports failed"); err != nil {
				j.log.Error("FailPayment via poll failed",
					slog.Int64("order_id", o.ID), slog.String("error", err.Error()))
			}
		}
		// StatusPending / StatusExpired (the gateway's view) → no-op;
		// our own expireOld() handles QR aging based on local time.
	}
	return nil
}

func (j *PaymentPollJob) expireOld(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-j.expiryWindow)
	rows, err := j.billing.ListExpiredPendingPayments(ctx, cutoff)
	if err != nil {
		return err
	}
	for _, o := range rows {
		if err := j.billing.ExpirePayment(ctx, o.ID); err != nil {
			j.log.Error("ExpirePayment failed",
				slog.Int64("order_id", o.ID), slog.String("error", err.Error()))
		}
	}
	return nil
}
