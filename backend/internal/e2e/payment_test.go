package e2e

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// seedNodeInboundPlan sets up the prerequisites for a purchase
// test: one mock-panel node + one inbound + one provisioning pool
// (with the node+inbound registered as a target) + one plan bound
// to that pool. Returns the plan ID so the test can post against it.
//
// The provisioning pool wiring is mandatory: as of the pool-aware
// billing refactor, plan.ProvisioningPoolID must be set for the
// service to resolve a target — purchases against pool-less plans
// short-circuit with ErrNoProvisioningTarget (409) before the
// balance check ever runs.
func seedNodeInboundPlan(t *testing.T, h *harness, adminTok string) (planID, nodeID int64, inboundTag string) {
	t.Helper()
	// Node — the e2e mockPanel serves canned responses, but the
	// dashboard's app wiring expects a real Node row in the DB so
	// inbound listing has something to find.
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/nodes",
		token:  adminTok,
		body: map[string]any{
			"name": "mock-node", "scheme": "http",
			"host":      h.panel.server.Listener.Addr().(*net.TCPAddr).IP.String(),
			"port":      h.panel.server.Listener.Addr().(*net.TCPAddr).Port,
			"api_token": "x", "enabled": true,
		},
	}, nil); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}
	nodeID = 1
	inboundTag = "vless-1"
	h.panel.SeedInbound(runtime.Inbound{
		ID: 1, Tag: inboundTag, Port: 443, Protocol: "vless", Enable: true,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	})

	poolID := seedProvisioningPool(t, h, adminTok, nodeID, inboundTag)

	var plan struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/plans", token: adminTok,
		body: map[string]any{
			"name": "30d", "duration_days": 30, "traffic_limit_bytes": 0,
			"price_cents": 500, "enabled": true,
			"provisioning_pool_id": poolID,
		},
	}, &plan); got != http.StatusCreated {
		t.Fatalf("create plan: status=%d", got)
	}
	return plan.ID, nodeID, inboundTag
}

// seedProvisioningPool creates a pool, adds (nodeID, inboundTag)
// as a target on it, and returns the pool ID. The pool has no
// protocol / port-range constraints so any inbound is accepted.
func seedProvisioningPool(t *testing.T, h *harness, adminTok string, nodeID int64, inboundTag string) int64 {
	t.Helper()
	var pool struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/provisioning-pools", token: adminTok,
		body: map[string]any{
			"name": "e2e-pool", "description": "e2e test pool",
			"enabled": true, "auto_create": false,
			"allowed_protocols": []string{},
		},
	}, &pool); got != http.StatusCreated {
		t.Fatalf("create provisioning pool: status=%d", got)
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/provisioning-pools/" + itoa(pool.ID) + "/targets",
		token:  adminTok,
		body: map[string]any{
			"node_id": nodeID, "inbound_tag": inboundTag,
			"max_clients": 0, "priority": 0, "enabled": true,
		},
	}, nil); got != http.StatusCreated {
		t.Fatalf("create provisioning target: status=%d", got)
	}
	return pool.ID
}

func seedAutoCreatePlan(t *testing.T, h *harness, adminTok string) int64 {
	t.Helper()
	u, _ := url.Parse(h.panel.URL())
	port, _ := strconv.Atoi(u.Port())
	var node struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/nodes",
		token:  adminTok,
		body: map[string]any{
			"name": "auto-node", "scheme": "http", "host": u.Hostname(),
			"port": port, "api_token": "x", "enabled": true,
		},
	}, &node); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}

	var tmpl struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbound-templates",
		token:  adminTok,
		body: map[string]any{
			"name": "auto-vless", "enabled": true, "protocol": "vless",
			"remark": "auto-vless", "listen": "",
			"settings":       `{"clients":[]}`,
			"streamSettings": `{"network":"tcp","security":"none"}`,
			"sniffing":       `{}`,
			"trafficReset":   "never",
			"total":          0,
			"expiryTime":     0,
		},
	}, &tmpl); got != http.StatusCreated {
		t.Fatalf("create inbound template: status=%d", got)
	}

	var pool struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/provisioning-pools",
		token:  adminTok,
		body: map[string]any{
			"name": "auto-pool", "enabled": true, "auto_create": true,
			"template_id": tmpl.ID, "node_ids": []int64{node.ID},
			"port_min": 18081, "port_max": 18082, "max_clients": 1,
			"allowed_protocols": []string{"vless"},
		},
	}, &pool); got != http.StatusCreated {
		t.Fatalf("create auto provisioning pool: status=%d", got)
	}

	var plan struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/plans",
		token:  adminTok,
		body: map[string]any{
			"name": "auto-30d", "duration_days": 30, "traffic_limit_bytes": 0,
			"price_cents": 500, "enabled": true,
			"provisioning_pool_id": pool.ID,
		},
	}, &plan); got != http.StatusCreated {
		t.Fatalf("create plan: status=%d", got)
	}
	return plan.ID
}

// orderRow reads a single order row directly from the DB so tests
// can assert payment columns without going through the JSON API.
func orderRow(t *testing.T, h *harness, id int64) *model.Order {
	t.Helper()
	var o model.Order
	if err := h.db.First(&o, id).Error; err != nil {
		t.Fatalf("read order %d: %v", id, err)
	}
	return &o
}

// ---- Alipay --------------------------------------------------------------

func TestAlipay_PurchaseFlow_HappyPath(t *testing.T) {
	mock := newMockAlipayHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	// Purchase via alipay — should land in payment_pending with
	// a QR URL populated.
	var order struct {
		ID                     int64  `json:"id"`
		Status                 string `json:"status"`
		PaymentMethod          string `json:"payment_method"`
		PaymentTargetURL       string `json:"payment_target_url"`
		PaymentProviderOrderID string `json:"payment_provider_order_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/user/purchase/alipay",
		token:  userTok,
		body: map[string]any{
			"plan_id":         planID,
			"idempotency_key": "alipay-test-key-1",
			"node_id":         1,
			"inbound_tag":     "vless-1",
		},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: status=%d", got)
	}
	if order.Status != "payment_pending" {
		t.Errorf("status = %q, want payment_pending", order.Status)
	}
	if order.PaymentMethod != "alipay" {
		t.Errorf("method = %q, want alipay", order.PaymentMethod)
	}
	if !strings.HasPrefix(order.PaymentTargetURL, "https://qr.alipay.com/bax") {
		t.Errorf("target_url = %q, want alipay QR URL", order.PaymentTargetURL)
	}
	if order.PaymentProviderOrderID == "" {
		t.Errorf("provider_order_id should be populated")
	}

	// Now simulate alipay's async notify → should advance the order.
	form := mock.alipay.AlipayNotifyForm(t, strconv.FormatInt(order.ID, 10), "TRADE_SUCCESS")
	resp := postForm(t, h, "/api/public/payment/alipay/notify", form)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notify: status=%d, body=%s", resp.StatusCode, resp.body)
	}
	if resp.body != "success" {
		t.Errorf("notify body = %q, want literal \"success\" per alipay contract", resp.body)
	}

	// Order should now be completed + have a client_ownership_id.
	final := orderRow(t, h, order.ID)
	if final.Status != "completed" {
		t.Errorf("final status = %q, want completed", final.Status)
	}
	if final.ClientOwnershipID == nil {
		t.Errorf("client_ownership_id should be set after provisioning")
	}
}

func TestAlipay_Notify_BadSignature(t *testing.T) {
	mock := newMockAlipayHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	var order struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase/alipay", token: userTok,
		body: map[string]any{"plan_id": planID, "idempotency_key": "k", "node_id": 1, "inbound_tag": "vless-1"},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: %d", got)
	}

	// Build a valid form then tamper with the signature.
	form := mock.alipay.AlipayNotifyForm(t, strconv.FormatInt(order.ID, 10), "TRADE_SUCCESS")
	form.Set("sign", "bm90LWEtdmFsaWQtc2lnbmF0dXJl") // base64("not-a-valid-signature")
	resp := postForm(t, h, "/api/public/payment/alipay/notify", form)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad-sig status = %d, want 400", resp.StatusCode)
	}

	// Order MUST still be payment_pending — no advance on bad sig.
	final := orderRow(t, h, order.ID)
	if final.Status != "payment_pending" {
		t.Errorf("bad-sig should not advance order; got status %q", final.Status)
	}
}

func TestBalancePurchase_AutoCreatesProvisioningTargetFromTemplate(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	userID, userTok := h.registerUser(t, "auto@example.com", "hunter2hunter2")
	planID := seedAutoCreatePlan(t, h, adminTok)
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/users/" + itoa(userID) + "/balance",
		token:  adminTok,
		body:   map[string]any{"delta_cents": 1000, "note": "auto-create e2e topup"},
	}, nil); got != http.StatusOK {
		t.Fatalf("balance adjust: status=%d", got)
	}

	var order struct {
		ID                     int64  `json:"id"`
		Status                 string `json:"status"`
		ProvisioningInboundTag string `json:"provisioning_inbound_tag"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/user/purchase",
		token:  userTok,
		body: map[string]any{
			"plan_id": planID, "idempotency_key": "auto-create-balance-1",
		},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: status=%d", got)
	}
	if order.Status != model.OrderStatusCompleted {
		t.Fatalf("order status = %q, want completed", order.Status)
	}
	if order.ProvisioningInboundTag != "pool-1-18081" {
		t.Fatalf("provisioning tag = %q, want pool-1-18081", order.ProvisioningInboundTag)
	}
	clients := h.panel.ClientsOn(order.ProvisioningInboundTag)
	if len(clients) != 1 {
		t.Fatalf("clients on generated inbound = %d, want 1", len(clients))
	}

	var targets []model.ProvisioningPoolTarget
	if err := h.db.Where("generated = TRUE").Find(&targets).Error; err != nil {
		t.Fatalf("read generated targets: %v", err)
	}
	if len(targets) != 1 || targets[0].InboundTag != order.ProvisioningInboundTag || targets[0].MaxClients != 1 {
		t.Fatalf("generated targets = %+v", targets)
	}
}

// ---- Stripe --------------------------------------------------------------

func TestStripe_PurchaseFlow_HappyPath(t *testing.T) {
	mock := newMockStripeHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	var order struct {
		ID                     int64  `json:"id"`
		Status                 string `json:"status"`
		PaymentMethod          string `json:"payment_method"`
		PaymentTargetURL       string `json:"payment_target_url"`
		PaymentProviderOrderID string `json:"payment_provider_order_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase/stripe", token: userTok,
		body: map[string]any{
			"plan_id": planID, "idempotency_key": "stripe-test-k1",
			"node_id": 1, "inbound_tag": "vless-1",
		},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: status=%d", got)
	}
	if order.Status != "payment_pending" {
		t.Errorf("status = %q, want payment_pending", order.Status)
	}
	if !strings.HasPrefix(order.PaymentTargetURL, "https://checkout.stripe.com/c/pay/") {
		t.Errorf("target_url = %q, want stripe checkout URL", order.PaymentTargetURL)
	}
	if !strings.HasPrefix(order.PaymentProviderOrderID, "cs_test_e2e_") {
		t.Errorf("provider_order_id = %q", order.PaymentProviderOrderID)
	}

	// Simulate Stripe webhook for checkout.session.completed.
	body := []byte(`{"type":"checkout.session.completed","data":{"object":{"id":"` + order.PaymentProviderOrderID + `","client_reference_id":"` + strconv.FormatInt(order.ID, 10) + `"}}}`)
	sig := mock.stripe.SignWebhook(body, time.Now())
	resp := postRaw(t, h, "/api/public/payment/stripe/webhook", body, map[string]string{
		"Content-Type":     "application/json",
		"Stripe-Signature": sig,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook: status=%d, body=%s", resp.StatusCode, resp.body)
	}

	final := orderRow(t, h, order.ID)
	if final.Status != "completed" {
		t.Errorf("final status = %q, want completed", final.Status)
	}
	if final.ClientOwnershipID == nil {
		t.Errorf("client_ownership_id should be set")
	}
}

func TestStripe_Webhook_ReplayProtection(t *testing.T) {
	mock := newMockStripeHarness(t)
	h := setupHarness(t, mock.opt)

	adminTok := h.adminLogin(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	planID, _, _ := seedNodeInboundPlan(t, h, adminTok)

	var order struct {
		ID                     int64  `json:"id"`
		PaymentProviderOrderID string `json:"payment_provider_order_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase/stripe", token: userTok,
		body: map[string]any{"plan_id": planID, "idempotency_key": "k", "node_id": 1, "inbound_tag": "vless-1"},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: %d", got)
	}

	// Sign with a timestamp 10 minutes old → should be rejected.
	body := []byte(`{"type":"checkout.session.completed","data":{"object":{"id":"` + order.PaymentProviderOrderID + `"}}}`)
	sig := mock.stripe.SignWebhook(body, time.Now().Add(-10*time.Minute))
	resp := postRaw(t, h, "/api/public/payment/stripe/webhook", body, map[string]string{
		"Content-Type":     "application/json",
		"Stripe-Signature": sig,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("replay status = %d, want 400", resp.StatusCode)
	}

	final := orderRow(t, h, order.ID)
	if final.Status != "payment_pending" {
		t.Errorf("replay should not advance order; got status %q", final.Status)
	}
}

// ---- PaymentMethods discovery -------------------------------------------

func TestPaymentMethods_ReflectsConfiguredGateways(t *testing.T) {
	// No payment gateways configured — only balance.
	h := setupHarness(t)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	var out struct {
		Methods []string `json:"methods"`
	}
	if got := h.do(t, req{
		method: http.MethodGet, path: "/api/user/payment-methods", token: userTok,
	}, &out); got != http.StatusOK {
		t.Fatalf("methods: %d", got)
	}
	if len(out.Methods) != 1 || out.Methods[0] != "balance" {
		t.Errorf("no-gateway methods = %v, want [balance]", out.Methods)
	}
}

func TestPaymentMethods_IncludesAlipayWhenConfigured(t *testing.T) {
	mock := newMockAlipayHarness(t)
	h := setupHarness(t, mock.opt)
	_, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	var out struct {
		Methods []string `json:"methods"`
	}
	if got := h.do(t, req{
		method: http.MethodGet, path: "/api/user/payment-methods", token: userTok,
	}, &out); got != http.StatusOK {
		t.Fatalf("methods: %d", got)
	}
	hasBalance, hasAlipay := false, false
	for _, m := range out.Methods {
		if m == "balance" {
			hasBalance = true
		}
		if m == "alipay" {
			hasAlipay = true
		}
	}
	if !hasBalance || !hasAlipay {
		t.Errorf("methods = %v, want both balance + alipay", out.Methods)
	}
}

// ---- harness wiring ------------------------------------------------------

// alipayHarness bundles a mockAlipay + the harnessOption that
// configures the dashboard to point at it. `bind` finishes the
// keypair handoff after the harness is built (needs t for keygen).
type alipayHarness struct {
	alipay  *mockAlipay
	privPEM string
	opt     harnessOption
}

func newMockAlipayHarness(t *testing.T) *alipayHarness {
	t.Helper()
	mock := newMockAlipay(t)
	priv, _ := mock.DashboardKeypairPEM(t)
	return &alipayHarness{
		alipay:  mock,
		privPEM: priv,
		opt: func(cfg *config.Config) {
			cfg.Alipay = config.Alipay{
				AppID:           "e2e_app_id",
				PrivateKey:      priv,
				AlipayPublicKey: mock.AlipayPublicKeyPEM(),
				Gateway:         mock.URL(),
			}
		},
	}
}

type stripeHarness struct {
	stripe *mockStripe
	opt    harnessOption
}

func newMockStripeHarness(t *testing.T) *stripeHarness {
	t.Helper()
	mock := newMockStripe(t)
	return &stripeHarness{
		stripe: mock,
		opt: func(cfg *config.Config) {
			cfg.Stripe = config.Stripe{
				SecretKey:            "sk_test_e2e_dummy",
				WebhookSecret:        mock.WebhookSecret(),
				Currency:             "usd",
				Endpoint:             mock.URL(),
				SessionExpiryMinutes: 30,
			}
		},
	}
}

// ---- HTTP form/raw helpers ----------------------------------------------

type rawResponse struct {
	StatusCode int
	body       string
}

func postForm(t *testing.T, h *harness, path string, form url.Values) rawResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.URL(path), strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return rawResponse{StatusCode: resp.StatusCode, body: string(body)}
}

func postRaw(t *testing.T, h *harness, path string, body []byte, headers map[string]string) rawResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.URL(path), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return rawResponse{StatusCode: resp.StatusCode, body: string(raw)}
}
