package sub

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

func TestBuildSIP008_FiltersToShadowsocks(t *testing.T) {
	subID := "abc123"
	user := &model.User{SubID: subID}

	// Mixed-protocol link list: 2 SS clients + 1 VLESS + 1 Trojan.
	d := &SubscriptionData{
		User: user,
		Links: []Link{
			{
				Protocol: "shadowsocks", Host: "n1.example", Port: 8388, Remark: "tokyo-ss",
				Inbound:  fixture("shadowsocks", `{}`, `{"method": "aes-256-gcm"}`),
				Client:   &runtime.Client{ID: "ss-id-1", Password: "pw1"},
			},
			{
				Protocol: "vless", Host: "n2.example", Port: 443, Remark: "vless-skip",
				Inbound:  fixture("vless", `{"network":"tcp"}`, ""),
				Client:   &runtime.Client{ID: "v-id"},
			},
			{
				Protocol: "shadowsocks", Host: "n3.example", Port: 8389, Remark: "sg-ss",
				Inbound:  fixture("shadowsocks", `{}`, `{"method": "chacha20-ietf-poly1305"}`),
				Client:   &runtime.Client{ID: "ss-id-2", Password: "pw2"},
			},
			{
				Protocol: "trojan", Host: "n4.example", Port: 443, Remark: "trojan-skip",
				Inbound:  fixture("trojan", `{}`, ""),
				Client:   &runtime.Client{Password: "trojan-pw"},
			},
		},
	}

	doc := buildSIP008(d)
	if doc.Version != 1 {
		t.Errorf("version = %d, want 1", doc.Version)
	}
	if doc.Username != subID {
		t.Errorf("username = %q, want %q", doc.Username, subID)
	}
	if len(doc.Servers) != 2 {
		t.Fatalf("servers count = %d, want 2 (SS-only)", len(doc.Servers))
	}
	if doc.Servers[0].Method != "aes-256-gcm" || doc.Servers[1].Method != "chacha20-ietf-poly1305" {
		t.Errorf("methods wrong: %v / %v", doc.Servers[0].Method, doc.Servers[1].Method)
	}
	if doc.Servers[0].Password != "pw1" || doc.Servers[1].Password != "pw2" {
		t.Errorf("passwords wrong")
	}
	if doc.Servers[0].Server != "n1.example" || doc.Servers[1].Server != "n3.example" {
		t.Errorf("servers wrong: %v / %v", doc.Servers[0].Server, doc.Servers[1].Server)
	}
	if doc.Servers[0].ServerPort != 8388 || doc.Servers[1].ServerPort != 8389 {
		t.Errorf("ports wrong")
	}
}

func TestBuildSIP008_EmptyDataReturnsEmptyServers(t *testing.T) {
	doc := buildSIP008(&SubscriptionData{User: &model.User{SubID: "x"}})
	if doc.Version != 1 {
		t.Errorf("version wrong")
	}
	if len(doc.Servers) != 0 {
		t.Errorf("expected empty servers slice")
	}
}

func TestBuildSIP008_NilSubscriptionData(t *testing.T) {
	doc := buildSIP008(nil)
	if doc.Version != 1 {
		t.Errorf("version wrong")
	}
	if len(doc.Servers) != 0 {
		t.Errorf("nil should produce empty servers")
	}
}

func TestBuildSIP008_NoSSClients(t *testing.T) {
	// User has only VLESS — SIP008 should still return a valid empty list,
	// not omit anything or crash.
	d := &SubscriptionData{
		User: &model.User{SubID: "y"},
		Links: []Link{
			{
				Protocol: "vless", Host: "n", Port: 443, Remark: "x",
				Inbound:  fixture("vless", `{}`, ""),
				Client:   &runtime.Client{ID: "u"},
			},
		},
	}
	doc := buildSIP008(d)
	if len(doc.Servers) != 0 {
		t.Errorf("expected empty servers list (no SS clients), got %d", len(doc.Servers))
	}
}
