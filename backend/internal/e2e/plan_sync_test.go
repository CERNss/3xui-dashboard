package e2e

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// TestPlanSyncMirror drives the "保存即全量镜像" contract end-to-end:
// editing a plan's limits refreshes existing subscribers' panel
// clients (without touching expiry), adding a pool target provisions
// it for existing subscribers with their inherited expiry, and
// deleting a pool target tears the corresponding clients down.
func TestPlanSyncMirror(t *testing.T) {
	const gib = int64(1) << 30

	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	userID, userTok := h.registerUser(t, "mirror@example.com", "hunter2hunter2")

	h.panel.SeedInbound(runtime.Inbound{
		Tag: "sync-a", Port: 443, Protocol: "vless", Enable: true,
		Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"none"}`,
	})
	h.panel.SeedInbound(runtime.Inbound{
		Tag: "sync-b", Port: 444, Protocol: "vless", Enable: true,
		Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"none"}`,
	})

	u, _ := url.Parse(h.panel.URL())
	port, _ := strconv.Atoi(u.Port())
	var node struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/nodes", token: adminTok,
		body: map[string]any{
			"name": "mirror-node", "scheme": "http", "host": u.Hostname(),
			"port": port, "api_token": "test-token", "enabled": true,
		},
	}, &node); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}

	poolID := seedProvisioningPool(t, h, adminTok, node.ID, "sync-a")

	var plan struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/plans", token: adminTok,
		body: map[string]any{
			"name": "mirror-30d", "duration_days": 30, "traffic_limit_bytes": gib,
			"price_cents": 500, "enabled": true, "provisioning_pool_id": poolID,
		},
	}, &plan); got != http.StatusCreated {
		t.Fatalf("create plan: status=%d", got)
	}

	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/users/" + itoa(userID) + "/balance", token: adminTok,
		body:   map[string]any{"delta_cents": 1000, "note": "mirror topup"},
	}, nil); got != http.StatusOK {
		t.Fatalf("topup: status=%d", got)
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/user/purchase", token: userTok,
		body:   map[string]any{"plan_id": plan.ID, "idempotency_key": "mirror-key-1"},
	}, nil); got != http.StatusOK {
		t.Fatalf("purchase: status=%d", got)
	}

	readOwnership := func(tag string) (limit *int64, expires *time.Time, found bool) {
		var row struct {
			TrafficLimitBytes *int64     `gorm:"column:traffic_limit_bytes"`
			ExpiresAt         *time.Time `gorm:"column:expires_at"`
			ID                int64      `gorm:"column:id"`
		}
		res := h.db.Raw(`SELECT id, traffic_limit_bytes, expires_at FROM client_ownerships
			WHERE user_id = ? AND inbound_tag = ?`, userID, tag).Scan(&row)
		if res.Error != nil {
			t.Fatalf("read ownership %s: %v", tag, res.Error)
		}
		return row.TrafficLimitBytes, row.ExpiresAt, row.ID != 0
	}

	limit0, expiry0, ok := readOwnership("sync-a")
	if !ok || limit0 == nil || *limit0 != gib || expiry0 == nil {
		t.Fatalf("post-purchase ownership: limit=%v expiry=%v found=%v", limit0, expiry0, ok)
	}

	// --- 1) Edit traffic limit → existing subscriber refreshed -------------
	var updated struct {
		Sync struct {
			Users     int      `json:"users"`
			Refreshed int      `json:"refreshed"`
			Errors    []string `json:"errors"`
		} `json:"sync"`
		SyncError string `json:"sync_error"`
	}
	if got := h.do(t, req{
		method: http.MethodPut, path: "/api/admin/plans/" + itoa(plan.ID), token: adminTok,
		body:   map[string]any{"traffic_limit_bytes": 2 * gib},
	}, &updated); got != http.StatusOK {
		t.Fatalf("update plan: status=%d", got)
	}
	if updated.SyncError != "" || updated.Sync.Users != 1 || updated.Sync.Refreshed != 1 {
		t.Fatalf("update sync = %+v (err=%q), want users=1 refreshed=1", updated.Sync, updated.SyncError)
	}
	clients := h.panel.ClientsOn("sync-a")
	if len(clients) != 1 || clients[0].TotalGB != 2*gib {
		t.Fatalf("panel client after limit edit = %+v, want TotalGB=%d", clients, 2*gib)
	}
	limit1, expiry1, _ := readOwnership("sync-a")
	if limit1 == nil || *limit1 != 2*gib {
		t.Errorf("ownership limit after edit = %v, want %d", limit1, 2*gib)
	}
	if expiry1 == nil || !expiry1.Equal(*expiry0) {
		t.Errorf("expiry changed by limit edit: %v -> %v", expiry0, expiry1)
	}

	// --- 2) Add a pool target → provisioned with inherited expiry ----------
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/provisioning-pools/" + itoa(poolID) + "/targets", token: adminTok,
		body: map[string]any{
			"node_id": node.ID, "inbound_tag": "sync-b",
			"max_clients": 0, "priority": 1, "enabled": true,
		},
	}, nil); got != http.StatusCreated {
		t.Fatalf("add target: status=%d", got)
	}
	var sync struct {
		Users   int      `json:"users"`
		Added   int      `json:"added"`
		Removed int      `json:"removed"`
		Errors  []string `json:"errors"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/plans/" + itoa(plan.ID) + "/sync", token: adminTok,
	}, &sync); got != http.StatusOK {
		t.Fatalf("manual sync: status=%d", got)
	}
	if sync.Added != 1 || len(sync.Errors) != 0 {
		t.Fatalf("sync after target add = %+v, want added=1", sync)
	}
	if got := len(h.panel.ClientsOn("sync-b")); got != 1 {
		t.Fatalf("panel clients on sync-b = %d, want 1", got)
	}
	_, expiryB, okB := readOwnership("sync-b")
	if !okB || expiryB == nil || !expiryB.Equal(*expiry0) {
		t.Errorf("added target expiry = %v, want inherited %v", expiryB, expiry0)
	}

	// --- 3) Delete the pool target → torn down ------------------------------
	var targetBID int64
	if err := h.db.Raw(`SELECT id FROM provisioning_pool_targets WHERE inbound_tag = 'sync-b'`).
		Scan(&targetBID).Error; err != nil || targetBID == 0 {
		t.Fatalf("find target B id: %v (id=%d)", err, targetBID)
	}
	if got := h.do(t, req{
		method: http.MethodDelete,
		path:   "/api/admin/provisioning-pools/targets/" + itoa(targetBID), token: adminTok,
	}, nil); got != http.StatusNoContent && got != http.StatusOK {
		t.Fatalf("delete target: status=%d", got)
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/plans/" + itoa(plan.ID) + "/sync", token: adminTok,
	}, &sync); got != http.StatusOK {
		t.Fatalf("sync after target delete: status=%d", got)
	}
	if sync.Removed != 1 || len(sync.Errors) != 0 {
		t.Fatalf("sync after target delete = %+v, want removed=1", sync)
	}
	if got := len(h.panel.ClientsOn("sync-b")); got != 0 {
		t.Errorf("panel clients on sync-b after removal = %d, want 0", got)
	}
	if _, _, still := readOwnership("sync-b"); still {
		t.Error("ownership row for sync-b survived target removal")
	}
	var ledger int64
	if err := h.db.Raw(`SELECT COUNT(*) FROM managed_clients WHERE inbound_tag = 'sync-b'`).
		Scan(&ledger).Error; err != nil {
		t.Fatalf("count managed_clients: %v", err)
	}
	if ledger != 0 {
		t.Errorf("managed_clients for sync-b = %d, want 0", ledger)
	}
}
