package public

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/payment"
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

func (h *PaymentNotifyHandler) gateway(provider string) (payment.Gateway, error) {
	reg := h.billing.Gateways()
	if reg == nil {
		return nil, payment.ErrUnknownProvider
	}
	return reg.Get(provider)
}
