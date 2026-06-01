// Package paymentcfg builds payment gateways from panel-managed
// settings (DB-first, env fallback), decrypting the stored secrets. It
// lives outside the payment package because it constructs the concrete
// alipay/stripe gateways — and those import payment, so a resolver in
// payment itself would cycle.
package paymentcfg

import (
	"context"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/payment"
	"github.com/cern/3xui-dashboard/internal/service/payment/alipay"
	"github.com/cern/3xui-dashboard/internal/service/payment/stripe"
)

// SettingsReader is the subset of repository.SettingRepo needed here.
// Declared locally so this package doesn't import repository.
type SettingsReader interface {
	GetString(ctx context.Context, key, fallback string) (string, error)
	GetInt(ctx context.Context, key string, fallback int64) (int64, error)
	GetSecret(ctx context.Context, key, fallback string) (string, error)
}

func getString(ctx context.Context, r SettingsReader, key, fb string) string {
	v, _ := r.GetString(ctx, key, fb)
	return v
}

func getSecret(ctx context.Context, r SettingsReader, key, fb string) string {
	v, err := r.GetSecret(ctx, key, fb)
	if err != nil {
		return fb
	}
	return v
}

func getInt(ctx context.Context, r SettingsReader, key string, fb int64) int64 {
	v, _ := r.GetInt(ctx, key, fb)
	return v
}

// AlipayConfig resolves the Alipay gateway config from settings,
// starting from fb so non-panel fields (ReturnURL) are preserved.
func AlipayConfig(ctx context.Context, r SettingsReader, fb config.Alipay) config.Alipay {
	out := fb
	out.AppID = getString(ctx, r, model.SettingAlipayAppID, fb.AppID)
	out.PrivateKey = getSecret(ctx, r, model.SettingAlipayPrivateKey, fb.PrivateKey)
	out.AlipayPublicKey = getString(ctx, r, model.SettingAlipayPublicKey, fb.AlipayPublicKey)
	out.Gateway = getString(ctx, r, model.SettingAlipayGateway, fb.Gateway)
	out.NotifyURL = getString(ctx, r, model.SettingAlipayNotifyURL, fb.NotifyURL)
	return out
}

// StripeConfig resolves the Stripe gateway config from settings,
// starting from fb so non-panel fields (Endpoint) are preserved.
func StripeConfig(ctx context.Context, r SettingsReader, fb config.Stripe) config.Stripe {
	out := fb
	out.SecretKey = getSecret(ctx, r, model.SettingStripeSecretKey, fb.SecretKey)
	out.WebhookSecret = getSecret(ctx, r, model.SettingStripeWebhookSecret, fb.WebhookSecret)
	out.Currency = getString(ctx, r, model.SettingStripeCurrency, fb.Currency)
	out.SuccessURL = getString(ctx, r, model.SettingStripeSuccessURL, fb.SuccessURL)
	out.CancelURL = getString(ctx, r, model.SettingStripeCancelURL, fb.CancelURL)
	out.SessionExpiryMinutes = int(getInt(ctx, r, model.SettingStripeSessionExpiryMinutes, int64(fb.SessionExpiryMinutes)))
	return out
}

// NewResolver returns a payment.Resolver that builds the live gateway
// set from settings on each call, falling back to the supplied env
// configs. alipay.New / stripe.New return nil for an unconfigured
// provider, so disabled gateways are simply absent from the map.
func NewResolver(r SettingsReader, alipayFB config.Alipay, stripeFB config.Stripe) payment.Resolver {
	return func(ctx context.Context) map[string]payment.Gateway {
		out := map[string]payment.Gateway{}
		if g := alipay.New(AlipayConfig(ctx, r, alipayFB)); g != nil {
			out[g.Provider()] = g
		}
		if g := stripe.New(StripeConfig(ctx, r, stripeFB)); g != nil {
			out[g.Provider()] = g
		}
		return out
	}
}
