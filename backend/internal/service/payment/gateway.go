// Package payment defines the cross-provider Gateway abstraction
// the billing service uses. Each payment provider (alipay, stripe,
// wechatpay …) lives in its own subpackage and registers itself
// through this interface.
//
// Keeping the interface small + provider-agnostic is deliberate:
// the billing service should never need to import a provider package
// directly. The registry is the only place that knows the concrete
// types.
package payment

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

// Status is the normalized payment state across providers. Each
// provider maps its own status vocabulary to one of these four.
type Status string

const (
	StatusPending Status = "pending" // user has not paid yet (alipay WAIT_BUYER_PAY)
	StatusPaid    Status = "paid"    // provider confirmed payment (alipay TRADE_SUCCESS/FINISHED)
	StatusFailed  Status = "failed"  // provider rejected or closed
	StatusExpired Status = "expired" // QR / session timed out
)

// ErrUnknownProvider is returned by the registry when callers ask
// for a provider that wasn't configured at boot.
var ErrUnknownProvider = errors.New("payment: unknown provider")

// CreateResult is the gateway's reply to a payment creation request.
// QRURL is what the portal renders as a QR; ProviderOrderID is what
// the gateway uses internally (alipay's trade_no) — we persist both.
type CreateResult struct {
	QRURL           string
	ExpiresAt       time.Time
	ProviderOrderID string
}

// Gateway is the per-provider implementation contract.
type Gateway interface {
	// Provider returns the wire name ("alipay", "stripe", ...). The
	// orders.payment_method column stores this verbatim.
	Provider() string

	// CreatePayment asks the provider to create a payment session for
	// the given order. Returns the QR/redirect URL + the provider's
	// own order ID. The billing service persists the QR url on the
	// order before returning to the client.
	CreatePayment(ctx context.Context, order *model.Order, planName string) (CreateResult, error)

	// Query asks the provider for the current state of a payment.
	// Used by the payment-poll job as a failsafe for dropped notifies.
	Query(ctx context.Context, providerOrderID string) (Status, error)

	// VerifyNotify validates the signature on an inbound notify
	// payload. The handler decodes the request body into params +
	// pulls the provider's "signature" field separately.
	VerifyNotify(params map[string]string, signature string) error
}

// Registry holds the enabled gateways keyed by provider name. App
// wiring builds the registry once at boot; the billing service +
// notify handler share the same instance.
type Registry struct {
	gateways map[string]Gateway
}

// NewRegistry builds an empty registry. Callers use Register to add
// providers.
func NewRegistry() *Registry {
	return &Registry{gateways: map[string]Gateway{}}
}

// Register adds a provider. Passing nil is a no-op so callers can
// write the idiomatic `registry.Register(alipay.New(cfg))` even when
// alipay.New returns nil for an unconfigured provider.
func (r *Registry) Register(g Gateway) {
	if g == nil {
		return
	}
	r.gateways[g.Provider()] = g
}

// Get returns the gateway for `provider`, or ErrUnknownProvider.
func (r *Registry) Get(provider string) (Gateway, error) {
	g, ok := r.gateways[provider]
	if !ok {
		return nil, ErrUnknownProvider
	}
	return g, nil
}

// EnabledProviders returns the list of registered provider names.
// Caller-friendly for the /payment-methods endpoint. "balance" is
// always included as the first element since balance-pay needs no
// gateway registration.
func (r *Registry) EnabledProviders() []string {
	out := []string{"balance"}
	for name := range r.gateways {
		out = append(out, name)
	}
	return out
}

// FormatYuan converts integer cents to a yuan-as-string with two
// decimal places — alipay's wire format for total_amount. Exported
// because both billing + alipay use it; defining it once here avoids
// drift between formatters.
func FormatYuan(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	if frac < 10 {
		return strconv.FormatInt(whole, 10) + ".0" + strconv.FormatInt(frac, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac, 10)
}
