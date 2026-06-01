package paymentcfg

import (
	"context"
	"testing"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
)

type fakeReader struct {
	strs    map[string]string
	ints    map[string]int64
	secrets map[string]string
}

func (f fakeReader) GetString(_ context.Context, key, fb string) (string, error) {
	if v, ok := f.strs[key]; ok {
		return v, nil
	}
	return fb, nil
}

func (f fakeReader) GetInt(_ context.Context, key string, fb int64) (int64, error) {
	if v, ok := f.ints[key]; ok {
		return v, nil
	}
	return fb, nil
}

func (f fakeReader) GetSecret(_ context.Context, key, fb string) (string, error) {
	if v, ok := f.secrets[key]; ok {
		return v, nil
	}
	return fb, nil
}

func TestAlipayConfig_DBOverridesFallbackAndPreservesNonPanelFields(t *testing.T) {
	r := fakeReader{
		strs:    map[string]string{model.SettingAlipayAppID: "db-app"},
		secrets: map[string]string{model.SettingAlipayPrivateKey: "db-priv"},
	}
	fb := config.Alipay{
		AppID: "env-app", PrivateKey: "env-priv", AlipayPublicKey: "env-pub",
		Gateway: "env-gw", NotifyURL: "env-notify", ReturnURL: "env-return",
	}

	got := AlipayConfig(context.Background(), r, fb)
	if got.AppID != "db-app" {
		t.Errorf("AppID = %q, want db override", got.AppID)
	}
	if got.PrivateKey != "db-priv" {
		t.Errorf("PrivateKey = %q, want db secret", got.PrivateKey)
	}
	if got.AlipayPublicKey != "env-pub" {
		t.Errorf("AlipayPublicKey = %q, want env fallback", got.AlipayPublicKey)
	}
	if got.ReturnURL != "env-return" {
		t.Errorf("ReturnURL = %q, want the non-panel field preserved from fallback", got.ReturnURL)
	}
}

func TestStripeConfig_FallsBackWhenEmpty(t *testing.T) {
	fb := config.Stripe{
		SecretKey: "env-sk", WebhookSecret: "env-wh", Currency: "eur",
		SuccessURL: "s", CancelURL: "c", SessionExpiryMinutes: 45, Endpoint: "http://test",
	}
	got := StripeConfig(context.Background(), fakeReader{}, fb)
	if got != fb {
		t.Errorf("empty reader should yield the fallback verbatim (incl. Endpoint), got %+v", got)
	}
}

func TestNewResolver_OnlyConfiguredGatewaysAppear(t *testing.T) {
	// Alipay fully configured via settings; Stripe left unconfigured.
	r := fakeReader{
		strs:    map[string]string{model.SettingAlipayAppID: "a", model.SettingAlipayPublicKey: "p"},
		secrets: map[string]string{model.SettingAlipayPrivateKey: "k"},
	}
	gws := NewResolver(r, config.Alipay{}, config.Stripe{})(context.Background())
	if _, ok := gws["alipay"]; !ok {
		t.Error("alipay should be present when fully configured")
	}
	if _, ok := gws["stripe"]; ok {
		t.Error("stripe should be absent when unconfigured")
	}
}
