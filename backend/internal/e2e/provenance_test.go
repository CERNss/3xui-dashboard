package e2e

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// TestProvenanceSplit drives the managed/external split end-to-end:
// pre-existing upstream inbounds stay external, dashboard-created
// inbounds/clients land in the ledger and come back annotated on the
// fleet endpoint, and deleting a managed inbound clears every
// dashboard-side record.
func TestProvenanceSplit(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)

	// Upstream panel already has an inbound with a client — created
	// outside the dashboard.
	h.panel.SeedInbound(runtime.Inbound{
		Tag: "external-1", Protocol: "vless", Port: 8443, Enable: true,
		Settings:       `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"ext-mail","enable":true}]}`,
		StreamSettings: `{}`, Sniffing: `{}`,
	})

	// Register the node.
	u, _ := url.Parse(h.panel.URL())
	port, _ := strconv.Atoi(u.Port())
	var node struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/nodes", token: adminTok,
		body: map[string]any{
			"name": "mock-node", "area": "sg", "scheme": "http", "host": u.Hostname(),
			"port": port, "base_path": "", "api_token": "test-token", "enabled": true,
		},
	}, &node); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}

	// Dashboard creates its own inbound → must be recorded managed.
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/inbounds/nodes/" + itoa(node.ID), token: adminTok,
		body: map[string]any{
			"remark": "ours", "enable": true, "port": 9555, "protocol": "vless",
			"settings": `{"clients":[]}`, "streamSettings": `{}`, "sniffing": `{}`, "tag": "ours-1",
		},
	}, nil); got != http.StatusCreated {
		t.Fatalf("create inbound: status=%d", got)
	}

	// Direct-add a client WITHOUT a bound user — the historic leak:
	// it must still be recorded as managed.
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/clients/nodes/" + itoa(node.ID) + "/inbounds/ours-1/add", token: adminTok,
		body: map[string]any{
			"client":  map[string]any{"id": "22222222-2222-2222-2222-222222222222", "email": "unbound-mail", "enable": true},
			"user_id": 0,
		},
	}, nil); got != http.StatusCreated {
		t.Fatalf("direct add client: status=%d", got)
	}

	// Fleet endpoint: managed flags + client states.
	type fleetInbound struct {
		NodeID  int64 `json:"node_id"`
		Managed bool  `json:"managed"`
		Inbound struct {
			Tag string `json:"tag"`
		} `json:"inbound"`
	}
	type clientState struct {
		InboundTag  string `json:"inbound_tag"`
		ClientEmail string `json:"client_email"`
		Managed     bool   `json:"managed"`
		UserID      *int64 `json:"user_id"`
	}
	var fleet struct {
		Inbounds     []fleetInbound `json:"inbounds"`
		ClientStates []clientState  `json:"client_states"`
	}
	if got := h.do(t, req{method: http.MethodGet, path: "/api/admin/inbounds", token: adminTok}, &fleet); got != http.StatusOK {
		t.Fatalf("fleet list: status=%d", got)
	}
	managedByTag := map[string]bool{}
	for _, row := range fleet.Inbounds {
		managedByTag[row.Inbound.Tag] = row.Managed
	}
	if !managedByTag["ours-1"] {
		t.Errorf("dashboard-created inbound not managed: %+v", managedByTag)
	}
	if managedByTag["external-1"] {
		t.Errorf("pre-existing upstream inbound marked managed: %+v", managedByTag)
	}
	var sawUnbound bool
	for _, st := range fleet.ClientStates {
		if st.ClientEmail == "ext-mail" {
			t.Errorf("external client leaked into client_states: %+v", st)
		}
		if st.ClientEmail == "unbound-mail" {
			sawUnbound = true
			if !st.Managed || st.UserID != nil {
				t.Errorf("unbound direct-add state = %+v, want managed with nil user", st)
			}
		}
	}
	if !sawUnbound {
		t.Error("unbound direct-add missing from client_states")
	}

	// Delete the managed inbound → ledger + ownership rows must go.
	if got := h.do(t, req{
		method: http.MethodDelete,
		path:   "/api/admin/inbounds/nodes/" + itoa(node.ID) + "/ours-1", token: adminTok,
	}, nil); got != http.StatusNoContent {
		t.Fatalf("delete inbound: status=%d", got)
	}
	for table, want := range map[string]int64{"managed_inbounds": 0, "managed_clients": 0} {
		var c int64
		if err := h.db.Raw(`SELECT COUNT(*) FROM ` + table).Scan(&c).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if c != want {
			t.Errorf("%s rows after delete = %d, want %d", table, c, want)
		}
	}
}
