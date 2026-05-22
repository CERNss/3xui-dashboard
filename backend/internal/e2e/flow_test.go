package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// TestFullFlow walks the whole API surface against a real DB +
// mocked 3x-ui panel. One scenario, sequenced, so failures point at
// the exact step that broke.
func TestFullFlow(t *testing.T) {
	h := setupHarness(t)

	// --- Admin login -------------------------------------------------------
	adminTok := h.adminLogin(t)
	if adminTok == "" {
		t.Fatal("admin login: empty token")
	}

	// Bad credentials must 401.
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/auth/login",
		body:   map[string]string{"username": adminUser, "password": "WRONG"},
	}, nil); got != http.StatusUnauthorized {
		t.Fatalf("bad creds: status=%d, want 401", got)
	}

	// --- User registration / login ----------------------------------------
	userID, userTok := h.registerUser(t, "alice@example.com", "hunter2hunter2")
	if userID == 0 || userTok == "" {
		t.Fatalf("register: userID=%d token-empty=%v", userID, userTok == "")
	}
	// Cross-audience: user token on admin route must 403.
	if got := h.do(t, req{
		method: http.MethodGet, path: "/api/admin/nodes", token: userTok,
	}, nil); got != http.StatusForbidden {
		t.Fatalf("cross-audience: status=%d, want 403", got)
	}
	// Duplicate register must 409.
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/auth/register",
		body: map[string]string{"email": "alice@example.com", "password": "hunter2hunter2"},
	}, nil); got != http.StatusConflict {
		t.Fatalf("duplicate register: status=%d, want 409", got)
	}

	// --- Node create + probe ----------------------------------------------
	u, _ := url.Parse(h.panel.URL())
	port, _ := strconv.Atoi(u.Port())
	var node struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/nodes", token: adminTok,
		body: map[string]any{
			"name": "mock-node", "scheme": "http", "host": u.Hostname(),
			"port": port, "base_path": "", "api_token": "test-token", "enabled": true,
		},
	}, &node); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}
	if node.Status != "unknown" {
		t.Errorf("new node status = %q, want unknown", node.Status)
	}

	// Probe the node — must reach the mock panel and flip to online.
	var probe struct {
		PriorStatus string         `json:"prior_status"`
		Status      runtime.Status `json:"status"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/nodes/" + itoa(node.ID) + "/probe", token: adminTok,
	}, &probe); got != http.StatusOK {
		t.Fatalf("probe node: status=%d", got)
	}
	if probe.Status.Xray.Version != "25.1.0" {
		t.Errorf("probe xray version = %q, want 25.1.0 (mock panel response)", probe.Status.Xray.Version)
	}
	if h.panel.Calls("/panel/api/server/status") == 0 {
		t.Error("probe didn't reach mock panel")
	}

	// Reload node — status should now be online.
	var nodeAfter struct {
		Status      string  `json:"status"`
		XrayVersion string  `json:"xray_version"`
		CPUPercent  float64 `json:"cpu_pct"`
		MemPercent  float64 `json:"mem_pct"`
	}
	_ = h.do(t, req{method: http.MethodGet, path: "/api/admin/nodes/" + itoa(node.ID), token: adminTok}, &nodeAfter)
	if nodeAfter.Status != "online" {
		t.Errorf("status after probe = %q, want online", nodeAfter.Status)
	}
	if nodeAfter.XrayVersion != "25.1.0" {
		t.Errorf("xray_version persisted = %q, want 25.1.0", nodeAfter.XrayVersion)
	}

	// Duplicate node name → 409.
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/nodes", token: adminTok,
		body: map[string]any{
			"name": "mock-node", "scheme": "http", "host": "h", "port": 1, "api_token": "x", "enabled": true,
		},
	}, nil); got != http.StatusConflict {
		t.Fatalf("dup node: status=%d, want 409", got)
	}

	// --- Plan + balance + purchase ----------------------------------------
	// Seed an inbound on the mock panel so ProvisionClient succeeds.
	h.panel.SeedInbound(runtime.Inbound{
		ID: 1, Tag: "vless-1", Port: 443, Protocol: "vless", Enable: true,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	})

	var plan struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/plans", token: adminTok,
		body: map[string]any{
			"name": "30d", "duration_days": 30, "traffic_limit_bytes": 0,
			"price_cents": 500, "enabled": true,
		},
	}, &plan); got != http.StatusCreated {
		t.Fatalf("create plan: status=%d", got)
	}

	// Purchase without balance → 402.
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: userTok,
		body: map[string]any{
			"plan_id": plan.ID, "node_id": node.ID, "inbound_tag": "vless-1",
			"idempotency_key": "key-1",
		},
	}, nil); got != http.StatusPaymentRequired {
		t.Fatalf("insufficient balance: status=%d, want 402", got)
	}

	// Top up.
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/users/" + itoa(userID) + "/balance", token: adminTok,
		body: map[string]any{"delta_cents": 1000, "note": "e2e topup"},
	}, nil); got != http.StatusOK {
		t.Fatalf("balance adjust: status=%d", got)
	}

	// Purchase happy path.
	var order struct {
		ID                int64  `json:"id"`
		Status            string `json:"status"`
		ClientOwnershipID *int64 `json:"client_ownership_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: userTok,
		body: map[string]any{
			"plan_id": plan.ID, "node_id": node.ID, "inbound_tag": "vless-1",
			"idempotency_key": "key-2",
		},
	}, &order); got != http.StatusOK {
		t.Fatalf("purchase: status=%d order=%+v", got, order)
	}
	if order.Status != "completed" {
		t.Errorf("order status = %q, want completed", order.Status)
	}
	if order.ClientOwnershipID == nil {
		t.Error("completed order missing ownership_id")
	}

	// Idempotency: same key returns the same order.
	var orderReplay struct {
		ID int64 `json:"id"`
	}
	_ = h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: userTok,
		body: map[string]any{
			"plan_id": plan.ID, "node_id": node.ID, "inbound_tag": "vless-1",
			"idempotency_key": "key-2",
		},
	}, &orderReplay)
	if orderReplay.ID != order.ID {
		t.Errorf("idempotency replay: got order %d, want %d", orderReplay.ID, order.ID)
	}

	// Same idempotency key cannot be replayed for a different purchase.
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: userTok,
		body: map[string]any{
			"plan_id": plan.ID, "node_id": node.ID, "inbound_tag": "missing-inbound",
			"idempotency_key": "key-2",
		},
	}, nil); got != http.StatusConflict {
		t.Fatalf("idempotency mismatch: status=%d, want 409", got)
	}

	// Another user must not be able to retrieve Alice's order by
	// reusing the same idempotency key.
	_, otherTok := h.registerUser(t, "bob@example.com", "hunter2hunter2")
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: otherTok,
		body: map[string]any{
			"plan_id": plan.ID, "node_id": node.ID, "inbound_tag": "vless-1",
			"idempotency_key": "key-2",
		},
	}, nil); got != http.StatusConflict {
		t.Fatalf("cross-user idempotency replay: status=%d, want 409", got)
	}

	// The provisioned client must show up on the mock panel (proves
	// addClient body reached the panel and was applied).
	clientsAfter := h.panel.ClientsOn("vless-1")
	if len(clientsAfter) != 1 {
		t.Fatalf("clients on vless-1 = %d, want 1", len(clientsAfter))
	}
	if clientsAfter[0].ID == "" {
		t.Errorf("provisioned VLESS client missing UUID id: %+v", clientsAfter[0])
	}

	// Balance log audit trail: admin_adjust + order_charge for the
	// successful order = 2 entries (no refund because purchase succeeded).
	var balanceCount int64
	if err := h.db.Raw(`SELECT COUNT(*) FROM balance_logs WHERE user_id = ?`, userID).Scan(&balanceCount).Error; err != nil {
		t.Fatalf("count balance_logs: %v", err)
	}
	if balanceCount != 2 {
		t.Errorf("balance_logs count = %d, want 2 (topup + charge)", balanceCount)
	}

	// --- Subscription ------------------------------------------------------
	var profile struct {
		SubID string `json:"sub_id"`
	}
	_ = h.do(t, req{method: http.MethodGet, path: "/api/user/profile", token: userTok}, &profile)
	if profile.SubID == "" {
		t.Fatal("profile.sub_id empty")
	}

	// Real sub_id → 200 + Subscription-Userinfo header.
	status, hdrs, body := h.raw(t, req{method: http.MethodGet, path: "/sub/" + profile.SubID})
	if status != http.StatusOK {
		t.Fatalf("/sub/<real>: status=%d", status)
	}
	if !strings.HasPrefix(hdrs.Get("Subscription-Userinfo"), "upload=") {
		t.Errorf("Subscription-Userinfo missing: %q", hdrs.Get("Subscription-Userinfo"))
	}
	if len(body) == 0 {
		t.Error("/sub/<real>: empty body (expected base64 link payload)")
	}

	if got := h.do(t, req{
		method: http.MethodPut, path: fmt.Sprintf("/api/admin/users/%d", userID), token: adminTok,
		body: map[string]any{"status": "suspended"},
	}, nil); got != http.StatusOK {
		t.Fatalf("suspend user: status=%d", got)
	}
	if got := h.do(t, req{method: http.MethodGet, path: "/api/user/profile", token: userTok}, nil); got != http.StatusForbidden {
		t.Errorf("suspended user old JWT: status=%d, want 403", got)
	}
	if got, _, _ := h.raw(t, req{method: http.MethodGet, path: "/sub/" + profile.SubID}); got != http.StatusNotFound {
		t.Errorf("suspended user sub URL: status=%d, want 404", got)
	}
	if got := h.do(t, req{
		method: http.MethodPut, path: fmt.Sprintf("/api/admin/users/%d", userID), token: adminTok,
		body: map[string]any{"status": "active"},
	}, nil); got != http.StatusOK {
		t.Fatalf("reactivate user: status=%d", got)
	}

	// Unknown sub_id → 404.
	if got, _, _ := h.raw(t, req{method: http.MethodGet, path: "/sub/no-such-id"}); got != http.StatusNotFound {
		t.Errorf("/sub/<unknown>: status=%d, want 404", got)
	}

	// --- Webhook create + test --------------------------------------------
	var webhook struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/webhooks", token: adminTok,
		body: map[string]any{
			"name": "ops", "url": "http://" + u.Host + "/never-resolves",
			"events": []string{"node.*", "order.*"}, "enabled": true, "allow_private": true,
		},
	}, &webhook); got != http.StatusCreated {
		t.Fatalf("create webhook: status=%d", got)
	}
	// Test delivery — async, just check the row was queued.
	var delivery struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/webhooks/" + itoa(webhook.ID) + "/test", token: adminTok,
	}, &delivery); got != http.StatusAccepted {
		t.Fatalf("webhook test: status=%d", got)
	}
	if delivery.Status != "pending" {
		t.Errorf("delivery starts as %q, want pending", delivery.Status)
	}
}
