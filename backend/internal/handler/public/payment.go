package public

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/payment"
	"github.com/cern/3xui-dashboard/internal/service/payment/stripe"
)

// PaymentNotifyHandler exposes the public async-notify endpoints
// payment providers POST to confirm payments. No JWT — the
// signature scheme is the auth. Each provider gets its own route so
// the URL alone is enough to pick the gateway.
type PaymentNotifyHandler struct {
	billing *billing.Service
	log     *slog.Logger
}

// NewPaymentNotifyHandler wires the handler.
func NewPaymentNotifyHandler(b *billing.Service, lg *slog.Logger) *PaymentNotifyHandler {
	return &PaymentNotifyHandler{billing: b, log: lg.With(slog.String("component", "handler.payment_notify"))}
}

// RegisterRoutes mounts the per-provider notify routes on the
// engine root. Routes are intentionally NOT under /api/* — alipay
// requires plain HTTP semantics + plain-text "success" body, and
// keeping them off /api keeps them out of any future API auth /
// rate-limit middleware we add downstream.
func (h *PaymentNotifyHandler) RegisterRoutes(rg *gin.Engine) {
	rg.POST("/api/public/payment/alipay/notify", h.alipayNotify)
	rg.POST("/api/public/payment/stripe/webhook", h.stripeWebhook)
}

// alipayNotify handles alipay async notifies. Per alipay's contract:
//
//   - Body is application/x-www-form-urlencoded
//   - Must verify `sign` (RSA2) against canonical-string of the rest
//   - On success respond with literal "success" (anything else
//     triggers up to 8 retries over 26 hours — which is fine for
//     genuine outages but spammy if our handler 500s on every call)
//   - On signature failure respond 400 and DO NOT advance any order
func (h *PaymentNotifyHandler) alipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form: %v", err)
		return
	}
	params := map[string]string{}
	for k, v := range c.Request.PostForm {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	signature := params["sign"]
	if signature == "" {
		h.log.Warn("alipay notify missing signature", slog.String("remote", c.ClientIP()))
		c.String(http.StatusBadRequest, "missing sign")
		return
	}

	gw, err := h.gateway("alipay")
	if err != nil {
		// Alipay gateway not configured but someone POSTed to the
		// route. Respond 503 so alipay's retry kicks in if the
		// config is restored within 26h.
		c.String(http.StatusServiceUnavailable, "alipay not configured")
		return
	}
	if err := gw.VerifyNotify(params, signature); err != nil {
		h.log.Warn("alipay notify signature failed",
			slog.String("remote", c.ClientIP()),
			slog.String("error", err.Error()),
		)
		c.String(http.StatusBadRequest, "bad signature")
		return
	}

	tradeStatus := params["trade_status"]
	outTradeNo := params["out_trade_no"]
	// We use the order ID as out_trade_no, but alipay's
	// payment_provider_order_id (== out_trade_no) is the stable
	// identifier the order table indexes on.
	switch tradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		if _, err := h.billing.ConfirmPayment(c.Request.Context(), outTradeNo); err != nil {
			if errors.Is(err, billing.ErrOrderNotFound) {
				// Permanent miss — respond `success` so alipay
				// stops retrying. We logged the warning above; manual
				// reconciliation is the operator's job at that point.
				h.log.Warn("alipay notify for unknown order",
					slog.String("out_trade_no", outTradeNo))
				c.String(http.StatusOK, "success")
				return
			}
			h.log.Error("ConfirmPayment failed",
				slog.String("out_trade_no", outTradeNo),
				slog.String("error", err.Error()),
			)
			c.String(http.StatusInternalServerError, "internal error")
			return
		}
	case "TRADE_CLOSED":
		_ = h.billing.FailPayment(c.Request.Context(), outTradeNo, "alipay reports TRADE_CLOSED")
	}
	// Any other trade_status (e.g. WAIT_BUYER_PAY) → we just ack.
	c.String(http.StatusOK, "success")
}

// stripeWebhook handles Stripe webhook events. Per Stripe's contract:
//
//   - Body is application/json
//   - Must verify Stripe-Signature HMAC against the RAW body — Gin's
//     JSON binding would mutate whitespace, so we read raw bytes
//     FIRST and verify before parsing
//   - 200 with any body = accepted; Stripe stops retrying
//   - Non-2xx triggers retry with exponential backoff for ~3 days
//
// Events we handle:
//
//   - checkout.session.completed     → ConfirmPayment
//   - checkout.session.async_payment_failed → FailPayment
//   - checkout.session.expired       → ExpirePayment (via lookup)
//
// All other event types are ack'd (200 {}) so Stripe doesn't retry.
func (h *PaymentNotifyHandler) stripeWebhook(c *gin.Context) {
	// Read raw body BEFORE any binding — Gin won't have touched it
	// since we have no middleware that reads the body here.
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "read body: %v", err)
		return
	}
	gw, err := h.gateway("stripe")
	if err != nil {
		c.String(http.StatusServiceUnavailable, "stripe not configured")
		return
	}
	// Type-assert to the concrete stripe.Gateway so we can call
	// VerifyWebhookRaw — the payment.Gateway interface doesn't
	// expose it (alipay signs params, stripe signs raw body).
	sg, ok := gw.(*stripe.Gateway)
	if !ok {
		h.log.Error("stripe gateway has wrong type")
		c.String(http.StatusInternalServerError, "stripe wiring broken")
		return
	}
	sigHeader := c.GetHeader("Stripe-Signature")
	if err := sg.VerifyWebhookRaw(rawBody, sigHeader); err != nil {
		h.log.Warn("stripe webhook signature failed",
			slog.String("remote", c.ClientIP()),
			slog.String("error", err.Error()),
		)
		c.String(http.StatusBadRequest, "bad signature")
		return
	}

	// Parse only the fields we care about — Stripe's event payloads
	// are huge but we just need event.type + the session object.
	var ev struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID                string `json:"id"`
				ClientReferenceID string `json:"client_reference_id"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		c.String(http.StatusBadRequest, "bad json: %v", err)
		return
	}
	sessionID := ev.Data.Object.ID

	switch ev.Type {
	case "checkout.session.completed":
		if _, err := h.billing.ConfirmPayment(c.Request.Context(), sessionID); err != nil {
			if errors.Is(err, billing.ErrOrderNotFound) {
				// Permanent miss — ack so stripe stops retrying.
				h.log.Warn("stripe webhook for unknown session",
					slog.String("session_id", sessionID))
				c.JSON(http.StatusOK, gin.H{})
				return
			}
			h.log.Error("ConfirmPayment failed",
				slog.String("session_id", sessionID),
				slog.String("error", err.Error()),
			)
			c.String(http.StatusInternalServerError, "internal error")
			return
		}
	case "checkout.session.async_payment_failed":
		_ = h.billing.FailPayment(c.Request.Context(), sessionID, "stripe reports async_payment_failed")
	case "checkout.session.expired":
		_ = h.billing.FailPayment(c.Request.Context(), sessionID, "stripe reports session expired")
	}
	// Any other event type → ack and move on.
	c.JSON(http.StatusOK, gin.H{})
}

func (h *PaymentNotifyHandler) gateway(provider string) (payment.Gateway, error) {
	reg := h.billing.Gateways()
	if reg == nil {
		return nil, payment.ErrUnknownProvider
	}
	return reg.Get(provider)
}
