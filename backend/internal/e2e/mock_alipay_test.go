package e2e

import (
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
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
)

// mockAlipay stands in for the alipay open-platform gateway. It
// signs every response with `serverPriv` so the dashboard's
// envelope-verify path validates; tests have access to the
// matching public PEM via the .ServerPublicKeyPEM() method.
//
// The handler routes by `method=alipay.trade.precreate` /
// `alipay.trade.query` and emits canned business responses.
type mockAlipay struct {
	server     *httptest.Server
	serverPriv *rsa.PrivateKey
	serverPub  string // PEM

	mu       sync.Mutex
	tradeNo  int           // monotonic — incremented per precreate
	statuses map[string]string // out_trade_no → trade_status override (defaults WAIT_BUYER_PAY)
}

func newMockAlipay(t *testing.T) *mockAlipay {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen rsa: %v", err)
	}
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}))

	m := &mockAlipay{
		serverPriv: priv,
		serverPub:  pubPEM,
		statuses:   map[string]string{},
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.server.Close)
	return m
}

// URL is the gateway endpoint the dashboard should call.
func (m *mockAlipay) URL() string { return m.server.URL }

// AlipayPublicKeyPEM is what gets configured as
// ALIPAY_PUBLIC_KEY — the dashboard verifies outbound-API responses
// + inbound notifies with this key.
func (m *mockAlipay) AlipayPublicKeyPEM() string { return m.serverPub }

// DashboardKeypairPEM generates a fresh keypair the dashboard uses
// to sign its requests. Mock doesn't actually verify requests, but
// the dashboard's SignRSA2 fails without a valid private key.
func (m *mockAlipay) DashboardKeypairPEM(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen dashboard key: %v", err)
	}
	privDer := x509.MarshalPKCS1PrivateKey(priv)
	pubDer, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDer}))
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer}))
	return
}

// SetTradeStatus overrides what trade.query returns for a given
// out_trade_no. Default is WAIT_BUYER_PAY (still-pending).
func (m *mockAlipay) SetTradeStatus(outTradeNo, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[outTradeNo] = status
}

func (m *mockAlipay) serve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	method := r.PostForm.Get("method")
	var innerJSON []byte
	switch method {
	case "alipay.trade.precreate":
		biz := struct {
			OutTradeNo  string `json:"out_trade_no"`
			TotalAmount string `json:"total_amount"`
			Subject     string `json:"subject"`
		}{}
		_ = json.Unmarshal([]byte(r.PostForm.Get("biz_content")), &biz)
		m.mu.Lock()
		m.tradeNo++
		m.mu.Unlock()
		innerJSON = []byte(fmt.Sprintf(
			`{"code":"10000","msg":"Success","out_trade_no":"%s","qr_code":"https://qr.alipay.com/bax%d"}`,
			biz.OutTradeNo, m.tradeNo,
		))
	case "alipay.trade.query":
		biz := struct {
			OutTradeNo string `json:"out_trade_no"`
		}{}
		_ = json.Unmarshal([]byte(r.PostForm.Get("biz_content")), &biz)
		m.mu.Lock()
		status := m.statuses[biz.OutTradeNo]
		m.mu.Unlock()
		if status == "" {
			status = "WAIT_BUYER_PAY"
		}
		innerJSON = []byte(fmt.Sprintf(
			`{"code":"10000","msg":"Success","out_trade_no":"%s","trade_no":"alipay-%s","trade_status":"%s","total_amount":"5.00"}`,
			biz.OutTradeNo, biz.OutTradeNo, status,
		))
	default:
		http.Error(w, "unknown method: "+method, 400)
		return
	}

	// Sign the inner JSON raw (envelope-sign behavior alipay uses).
	digest := sha256.Sum256(innerJSON)
	sig, err := rsa.SignPKCS1v15(nil, m.serverPriv, crypto.SHA256, digest[:])
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	envKey := strings.ReplaceAll(method, ".", "_") + "_response"
	envelope := map[string]json.RawMessage{
		envKey: innerJSON,
		"sign": json.RawMessage(fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(sig))),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(envelope)
}

// AlipayNotifyForm builds a notify POST body the dashboard's
// /api/public/payment/alipay/notify endpoint accepts. Signs with
// the mock's private key (which the dashboard knows as the
// "alipay public key" → verify-side).
func (m *mockAlipay) AlipayNotifyForm(t *testing.T, outTradeNo, tradeStatus string) url.Values {
	t.Helper()
	params := map[string]string{
		"out_trade_no": outTradeNo,
		"trade_no":     "alipay-" + outTradeNo,
		"trade_status": tradeStatus,
		"total_amount": "5.00",
		"sign_type":    "RSA2",
	}
	// Canonical string: sorted keys, drop empty + sign/sign_type
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" || k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k + "=" + params[k])
	}
	digest := sha256.Sum256([]byte(b.String()))
	sig, err := rsa.SignPKCS1v15(nil, m.serverPriv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign notify: %v", err)
	}
	params["sign"] = base64.StdEncoding.EncodeToString(sig)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	return form
}
