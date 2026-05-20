package stripe

import (
	"context"
	"fmt"
	"time"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// Gateway adapts our Client + webhook verifier to payment.Gateway.
// It also exposes Verify directly so the public webhook handler can
// run HMAC verification BEFORE parsing the body.
type Gateway struct {
	client        *Client
	webhookSecret string
	now           func() time.Time
}

// New returns a payment.Gateway backed by Stripe Checkout, or nil
// when the config isn't fully populated.
func New(cfg config.Stripe) payment.Gateway {
	if !cfg.Enabled() {
		return nil
	}
	return &Gateway{
		client:        NewClient(cfg.SecretKey, cfg.Currency, cfg.SuccessURL, cfg.CancelURL, cfg.SessionExpiryMinutes),
		webhookSecret: cfg.WebhookSecret,
		now:           time.Now,
	}
}

// Provider returns the string used in orders.payment_method.
func (g *Gateway) Provider() string { return "stripe" }

// CreatePayment maps to Stripe's CreateCheckoutSession. The
// `payment_qr_url` column on Order ends up holding the Checkout
// page URL — the frontend redirects there instead of rendering a
// QR. Naming preserved from alipay so we don't need a schema change.
func (g *Gateway) CreatePayment(ctx context.Context, order *model.Order, planName string) (payment.CreateResult, error) {
	sess, err := g.client.CreateCheckoutSession(ctx, order.ID, order.PriceCents, planName)
	if err != nil {
		return payment.CreateResult{}, fmt.Errorf("stripe checkout: %w", err)
	}
	return payment.CreateResult{
		QRURL:           sess.URL,
		ExpiresAt:       time.Unix(sess.ExpiresAt, 0),
		ProviderOrderID: sess.ID,
	}, nil
}

// Query maps Stripe's payment_status onto the normalized payment.Status.
func (g *Gateway) Query(ctx context.Context, providerOrderID string) (payment.Status, error) {
	sess, err := g.client.RetrieveCheckoutSession(ctx, providerOrderID)
	if err != nil {
		return "", err
	}
	switch sess.PaymentStatus {
	case "paid", "no_payment_required":
		return payment.StatusPaid, nil
	case "unpaid":
		// Distinguish "still waiting" from "expired" via the session
		// status field — Stripe sets status=expired on timeout.
		if sess.Status == "expired" {
			return payment.StatusExpired, nil
		}
		return payment.StatusPending, nil
	default:
		return payment.StatusPending, nil
	}
}

// VerifyNotify isn't used for Stripe — Stripe signs the raw body,
// not param maps. The webhook handler calls VerifyWebhookRaw
// directly. We satisfy the interface so the registry treats Stripe
// the same as alipay; calling this method returns an error so a
// mistake in wiring is loud.
func (g *Gateway) VerifyNotify(params map[string]string, signature string) error {
	return fmt.Errorf("stripe: use VerifyWebhookRaw — Stripe signs the raw body, not params")
}

// VerifyWebhookRaw satisfies the payment.RawBodyVerifier interface.
// The public webhook handler asserts that interface and calls this
// method with the bytes it read from the request body BEFORE any
// parsing — that's the same bytes Stripe signed.
func (g *Gateway) VerifyWebhookRaw(rawBody []byte, sigHeader string) error {
	return VerifyWebhook(rawBody, sigHeader, g.webhookSecret, g.now())
}

// Compile-time interface assertion — fails build if Gateway stops
// implementing RawBodyVerifier.
var _ payment.RawBodyVerifier = (*Gateway)(nil)
