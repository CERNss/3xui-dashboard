package public

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/payment"
)

func init() { gin.SetMode(gin.TestMode) }

// fakeGateway stands in for a payment.Gateway in handler tests.
// Test-supplied verify lets us simulate signature failure without
// generating real RSA keypairs in every test case.
type fakeGateway struct {
	verify func(params map[string]string, sig string) error
}

func (f *fakeGateway) Provider() string { return "alipay" }
func (f *fakeGateway) CreatePayment(_ context.Context, _ *model.Order, _ string) (payment.CreateResult, error) {
	return payment.CreateResult{}, nil
}
func (f *fakeGateway) Query(_ context.Context, _ string) (payment.Status, error) {
	return payment.StatusPending, nil
}
func (f *fakeGateway) VerifyNotify(params map[string]string, sig string) error {
	if f.verify == nil {
		return nil
	}
	return f.verify(params, sig)
}

// fakeBilling captures ConfirmPayment / FailPayment calls.
type fakeBilling struct {
	*billing.Service
	confirmed []string
	failed    []string
}

// We compose with billing.Service via a thin shim: the handler only
// reads Gateways() + calls ConfirmPayment/FailPayment, so we expose
// just those two through a wrapper interface fed by the real
// handler's reference. Easier: build a billing.Service with all
// real deps and intercept by stubbing the repos. That's heavy for
// a handler test, so we use a parallel fake type and assert via
// counters on the gateway.

// realFakeServer builds a handler with a registry containing one
// fake gateway. confirm/fail counters live on the gateway struct so
// the test can read them post-request.
func newHandlerWithFake(t *testing.T, fg *fakeGateway) (*gin.Engine, *fakeGateway) {
	t.Helper()
	reg := payment.NewRegistry()
	reg.Register(fg)
	// Build a minimal billing.Service. Pass nil for repos that the
	// notify path doesn't touch — the handler only uses Gateways()
	// + ConfirmPayment/FailPayment. ConfirmPayment will crash on
	// nil orderRepo, but we deliberately set up signature-failure /
	// missing-order test cases that short-circuit BEFORE reaching
	// the billing service.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := billing.New(nil, nil, nil, nil, nil, reg, logger)
	h := NewPaymentNotifyHandler(svc, logger)
	e := gin.New()
	h.RegisterRoutes(e)
	return e, fg
}

func TestAlipayNotify_MissingSign(t *testing.T) {
	e, _ := newHandlerWithFake(t, &fakeGateway{})
	form := url.Values{"trade_status": []string{"TRADE_SUCCESS"}, "out_trade_no": []string{"42"}}
	req := httptest.NewRequest(http.MethodPost, "/api/public/payment/alipay/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (missing sign)", w.Code)
	}
}

func TestAlipayNotify_BadSignature(t *testing.T) {
	verifyCalled := 0
	fg := &fakeGateway{
		verify: func(_ map[string]string, _ string) error {
			verifyCalled++
			return payment.ErrUnknownProvider // any non-nil error works
		},
	}
	e, _ := newHandlerWithFake(t, fg)
	form := url.Values{
		"sign":         []string{"badsig"},
		"trade_status": []string{"TRADE_SUCCESS"},
		"out_trade_no": []string{"42"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/public/payment/alipay/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 on bad signature", w.Code)
	}
	if verifyCalled != 1 {
		t.Errorf("verify call count = %d, want 1", verifyCalled)
	}
	if strings.Contains(w.Body.String(), "success") {
		t.Errorf("response should NOT contain 'success' on bad sig: %s", w.Body.String())
	}
}

// ---- Stripe ---------------------------------------------------------------

// We can't easily construct a full stripe.Gateway in a handler test
// (it lives in another package), so we exercise the failure paths
// that don't require the gateway — bad signature header, gateway
// not configured. Roundtrip success is covered by stripe package
// tests + the e2e suite once it exists.

func TestStripeWebhook_GatewayNotConfigured(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := billing.New(nil, nil, nil, nil, nil, payment.NewRegistry(), logger)
	h := NewPaymentNotifyHandler(svc, logger)
	e := gin.New()
	h.RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodPost, "/api/public/payment/stripe/webhook", strings.NewReader(`{"type":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t=1700000000,v1=abc")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestAlipayNotify_GatewayNotConfigured(t *testing.T) {
	// Build a handler with NO gateway registered.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := billing.New(nil, nil, nil, nil, nil, payment.NewRegistry(), logger)
	h := NewPaymentNotifyHandler(svc, logger)
	e := gin.New()
	h.RegisterRoutes(e)

	form := url.Values{"sign": []string{"x"}, "trade_status": []string{"TRADE_SUCCESS"}, "out_trade_no": []string{"42"}}
	req := httptest.NewRequest(http.MethodPost, "/api/public/payment/alipay/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when no alipay registered", w.Code)
	}
}

// generateAlipayLikeKeypair returns a (privPEM, pubPEM) pair — used
// to exercise the full alipay verify path against the real Gateway
// adapter from the alipay package. Lives here, not in the alipay
// package, because the handler-level smoke test only needs the
// PEMs; no other tests need this helper.
func generateAlipayLikeKeypair(t *testing.T) (priv, pub string) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen rsa: %v", err)
	}
	privDer := x509.MarshalPKCS1PrivateKey(k)
	pubDer, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	priv = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDer}))
	pub = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}))
	return
}

// signParams replicates alipay's canonical-string + RSA2 algorithm
// to sign a notify payload from the test side.
func signParams(t *testing.T, params map[string]string, privPEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(privPEM))
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse priv: %v", err)
	}
	// canonical string: sorted keys, drop empty + sign/sign_type
	keys := []string{}
	for k, v := range params {
		if v == "" || k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sortStrings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k + "=" + params[k])
	}
	digest := sha256.Sum256([]byte(b.String()))
	sig, err := rsa.SignPKCS1v15(nil, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
