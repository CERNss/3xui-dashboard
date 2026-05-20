package alipay

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

// Gateway adapts our Client to the payment.Gateway interface so the
// billing service treats alipay the same as any other provider.
type Gateway struct {
	client *Client
}

// New returns a payment.Gateway backed by alipay 当面付, or nil if
// the provided config isn't fully populated. Returning nil instead
// of erroring lets app wiring do `registry.Register(alipay.New(cfg))`
// without branching — the registry treats nil as "skip".
func New(cfg config.Alipay) payment.Gateway {
	if !cfg.Enabled() {
		return nil
	}
	return &Gateway{
		client: NewClient(cfg.AppID, cfg.PrivateKey, cfg.AlipayPublicKey, cfg.Gateway, cfg.NotifyURL),
	}
}

// Provider returns the string used in orders.payment_method.
func (g *Gateway) Provider() string { return "alipay" }

// CreatePayment calls alipay.trade.precreate, returns the QR + a
// 15-minute expires_at hint (alipay's default QR life is 2h but we
// expire eagerly to keep payment_pending rows from sticking).
//
// Both TimeExpire (wire) and ExpiresAt (DB) come from the same
// `now + 15m` so the QR-shown countdown matches the DB-side
// payment-poll expiry. We route through the client's `now` so tests
// can pin the clock; the wire string is rendered in Beijing TZ via
// formatBeijing since alipay's format has no offset.
func (g *Gateway) CreatePayment(ctx context.Context, order *model.Order, planName string) (payment.CreateResult, error) {
	expiresAt := g.client.now().Add(15 * time.Minute)
	req := PrecreateRequest{
		OutTradeNo:  strconv.FormatInt(order.ID, 10),
		TotalAmount: payment.FormatYuan(order.PriceCents),
		Subject:     planName,
		TimeExpire:  formatBeijing(expiresAt),
	}
	resp, err := g.client.Precreate(ctx, req)
	if err != nil {
		return payment.CreateResult{}, fmt.Errorf("alipay precreate: %w", err)
	}
	return payment.CreateResult{
		QRURL:           resp.QRCode,
		ExpiresAt:       expiresAt,
		ProviderOrderID: resp.OutTradeNo,
	}, nil
}

// Query maps alipay's trade_status vocabulary onto the normalized
// payment.Status. Unknown statuses bucket as "pending" so a real
// payment doesn't get marked failed by an unmapped state.
func (g *Gateway) Query(ctx context.Context, providerOrderID string) (payment.Status, error) {
	resp, err := g.client.TradeQuery(ctx, providerOrderID)
	if err != nil {
		// Distinguish "alipay says trade not found" (treat as pending —
		// possibly the precreate response hadn't landed yet) from real
		// errors (network, signature failure).
		if isNotFoundError(err) {
			return payment.StatusPending, nil
		}
		return "", err
	}
	switch resp.TradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		return payment.StatusPaid, nil
	case "WAIT_BUYER_PAY":
		return payment.StatusPending, nil
	case "TRADE_CLOSED":
		return payment.StatusFailed, nil
	default:
		return payment.StatusPending, nil
	}
}

// VerifyNotify validates the RSA2 signature on an inbound alipay
// notify form payload.
func (g *Gateway) VerifyNotify(params map[string]string, signature string) error {
	return VerifyRSA2(params, signature, g.client.alipayPublicKey)
}

// isNotFoundError detects alipay's "trade does not exist" responses
// — sub_code ACQ.TRADE_NOT_EXIST. The do() method bubbles this as
// a string-formatted error so we string-match the sub_code rather
// than introducing a new typed error.
func isNotFoundError(err error) bool {
	var alipayErr *AlipayError
	if errors.As(err, &alipayErr) {
		return alipayErr.SubCode == "ACQ.TRADE_NOT_EXIST"
	}
	return false
}

// AlipayError is reserved for future typed-error work; the current
// do() wraps codes as fmt.Errorf strings, which isNotFoundError
// covers via errors.As — both shapes return false here today, but
// the type is exported so callers can switch on it later.
type AlipayError struct {
	Code    string
	SubCode string
	Msg     string
}

func (e *AlipayError) Error() string { return e.Code + ":" + e.Msg }
