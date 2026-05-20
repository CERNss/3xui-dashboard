package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeAlipayServer returns a httptest.Server that pretends to be
// the alipay gateway. The handler signs every response with
// `serverPriv` so the client's envelope-verify step can validate.
type fakeAlipayServer struct {
	t          *testing.T
	server     *httptest.Server
	serverPriv *rsa.PrivateKey
	serverPub  string // PEM
	// handler is replaced per-test to control the response
	handler func(method string, params map[string]string) (innerJSON []byte, alipayCode string)
}

func newFakeAlipay(t *testing.T) *fakeAlipayServer {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen alipay key: %v", err)
	}
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal alipay pub: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}))

	f := &fakeAlipayServer{
		t:          t,
		serverPriv: priv,
		serverPub:  pubPEM,
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAlipayServer) serve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	params := map[string]string{}
	for k, v := range r.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	method := params["method"]
	if f.handler == nil {
		http.Error(w, "no handler set", 500)
		return
	}
	innerJSON, _ := f.handler(method, params)
	// Sign inner JSON RAW (alipay envelope-sign behavior)
	digest := sha256.Sum256(innerJSON)
	sig, err := rsa.SignPKCS1v15(nil, f.serverPriv, crypto.SHA256, digest[:])
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Wrap into envelope keyed by method
	envKey := strings.ReplaceAll(method, ".", "_") + "_response"
	envelope := map[string]json.RawMessage{
		envKey: innerJSON,
		"sign": json.RawMessage(fmt.Sprintf("%q", sigB64)),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(envelope)
}

// clientKeypair returns a (priv, pub) PEM pair representing our app's
// keypair. The alipay fake server doesn't actually verify our request
// signature (real alipay does), but we need a parseable priv to sign.
func clientKeypair(t *testing.T) (priv, pub string) {
	t.Helper()
	return genTestKeypair(t)
}

func TestPrecreate_Success(t *testing.T) {
	f := newFakeAlipay(t)
	priv, _ := clientKeypair(t)

	f.handler = func(method string, _ map[string]string) ([]byte, string) {
		if method != "alipay.trade.precreate" {
			t.Errorf("expected method alipay.trade.precreate, got %s", method)
		}
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","qr_code":"https://qr.alipay.com/bax12345"}`), "10000"
	}

	c := NewClient("2021000000", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	resp, err := c.Precreate(context.Background(), PrecreateRequest{
		OutTradeNo: "42", TotalAmount: "5.00", Subject: "test plan",
	})
	if err != nil {
		t.Fatalf("Precreate: %v", err)
	}
	if resp.QRCode != "https://qr.alipay.com/bax12345" {
		t.Errorf("QRCode = %q", resp.QRCode)
	}
	if resp.OutTradeNo != "42" {
		t.Errorf("OutTradeNo = %q", resp.OutTradeNo)
	}
}

func TestPrecreate_AlipayErrorCode(t *testing.T) {
	f := newFakeAlipay(t)
	priv, _ := clientKeypair(t)
	f.handler = func(_ string, _ map[string]string) ([]byte, string) {
		return []byte(`{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_HAS_SUCCESS","sub_msg":"交易已支付"}`), "40004"
	}
	c := NewClient("2021000000", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	_, err := c.Precreate(context.Background(), PrecreateRequest{OutTradeNo: "42", TotalAmount: "5.00", Subject: "x"})
	if err == nil {
		t.Fatal("expected error on alipay business failure")
	}
	if !strings.Contains(err.Error(), "40004") {
		t.Errorf("error missing alipay code: %v", err)
	}
}

func TestPrecreate_RejectsTamperedEnvelope(t *testing.T) {
	f := newFakeAlipay(t)
	priv, _ := clientKeypair(t)
	// Use a DIFFERENT keypair as the "alipay public key" so the
	// signature won't verify even though the response shape is fine.
	_, otherPub := genTestKeypair(t)

	f.handler = func(_ string, _ map[string]string) ([]byte, string) {
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","qr_code":"https://x"}`), "10000"
	}
	c := NewClient("2021000000", priv, otherPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	_, err := c.Precreate(context.Background(), PrecreateRequest{OutTradeNo: "42", TotalAmount: "5.00", Subject: "x"})
	if err == nil {
		t.Fatal("expected signature failure with mismatched alipay pubkey")
	}
}

func TestTradeQuery_Paid(t *testing.T) {
	f := newFakeAlipay(t)
	priv, _ := clientKeypair(t)
	f.handler = func(method string, _ map[string]string) ([]byte, string) {
		if method != "alipay.trade.query" {
			t.Errorf("method = %s", method)
		}
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","trade_no":"2026052022001000001234567","trade_status":"TRADE_SUCCESS","total_amount":"5.00"}`), "10000"
	}
	c := NewClient("2021000000", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	resp, err := c.TradeQuery(context.Background(), "42")
	if err != nil {
		t.Fatalf("TradeQuery: %v", err)
	}
	if resp.TradeStatus != "TRADE_SUCCESS" {
		t.Errorf("TradeStatus = %q", resp.TradeStatus)
	}
	if resp.TradeNo != "2026052022001000001234567" {
		t.Errorf("TradeNo = %q", resp.TradeNo)
	}
}

func TestTradeQuery_Pending(t *testing.T) {
	f := newFakeAlipay(t)
	priv, _ := clientKeypair(t)
	f.handler = func(_ string, _ map[string]string) ([]byte, string) {
		return []byte(`{"code":"10000","msg":"Success","out_trade_no":"42","trade_status":"WAIT_BUYER_PAY"}`), "10000"
	}
	c := NewClient("2021000000", priv, f.serverPub, f.server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	resp, err := c.TradeQuery(context.Background(), "42")
	if err != nil {
		t.Fatalf("TradeQuery: %v", err)
	}
	if resp.TradeStatus != "WAIT_BUYER_PAY" {
		t.Errorf("TradeStatus = %q", resp.TradeStatus)
	}
}

func TestDo_HTTPErrorBubbles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte("gateway down"))
	}))
	t.Cleanup(server.Close)

	priv, _ := clientKeypair(t)
	_, pub := genTestKeypair(t)
	c := NewClient("2021000000", priv, pub, server.URL, "")
	c.SetNow(func() time.Time { return time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC) })

	_, err := c.TradeQuery(context.Background(), "42")
	if err == nil {
		t.Fatal("expected error from 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error missing status: %v", err)
	}
}
