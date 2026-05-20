package sub

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

func TestSingboxOutbound_VLESSReality(t *testing.T) {
	in := fixture("vless", `{
		"network": "tcp",
		"security": "reality",
		"realitySettings": {
			"publicKey": "PUB",
			"shortIds": ["sid"],
			"serverNames": ["s.example.com"]
		}
	}`, "")
	c := &runtime.Client{ID: "v-uuid", Flow: "xtls-rprx-vision"}

	o, ok := singboxOutbound("h", 443, in, c, "vless-r")
	if !ok {
		t.Fatalf("ok=false")
	}
	if o["type"] != "vless" {
		t.Errorf("type = %v", o["type"])
	}
	if o["uuid"] != c.ID {
		t.Errorf("uuid wrong")
	}
	if o["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow missing")
	}
	if o["server_port"] != 443 {
		t.Errorf("server_port wrong")
	}
	tls, ok := o["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls block missing")
	}
	if tls["enabled"] != true {
		t.Errorf("tls.enabled not set")
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok {
		t.Fatalf("reality block missing")
	}
	if reality["public_key"] != "PUB" {
		t.Errorf("public_key wrong")
	}
	if tls["server_name"] != "s.example.com" {
		t.Errorf("server_name wrong: %v", tls["server_name"])
	}
}

func TestSingboxOutbound_VMessws(t *testing.T) {
	in := fixture("vmess", `{
		"network": "ws",
		"wsSettings": {"path": "/p", "headers": {"Host": "h.example.com"}}
	}`, "")
	c := &runtime.Client{ID: "u"}

	o, _ := singboxOutbound("h", 443, in, c, "v")
	if o["type"] != "vmess" {
		t.Errorf("type wrong")
	}
	if o["alter_id"] != 0 {
		t.Errorf("alter_id must be 0, got %v", o["alter_id"])
	}
	transport, ok := o["transport"].(map[string]any)
	if !ok {
		t.Fatalf("transport missing")
	}
	if transport["type"] != "ws" {
		t.Errorf("transport.type wrong")
	}
	if transport["path"] != "/p" {
		t.Errorf("transport.path wrong")
	}
}

func TestSingboxOutbound_Trojan(t *testing.T) {
	in := fixture("trojan", `{
		"network": "tcp",
		"security": "tls",
		"tlsSettings": {"serverName": "t.example.com"}
	}`, "")
	c := &runtime.Client{Password: "pw"}

	o, _ := singboxOutbound("h", 443, in, c, "t")
	if o["type"] != "trojan" {
		t.Errorf("type wrong")
	}
	if o["password"] != "pw" {
		t.Errorf("password wrong")
	}
	tls, _ := o["tls"].(map[string]any)
	if tls["server_name"] != "t.example.com" {
		t.Errorf("server_name wrong")
	}
}

func TestSingboxOutbound_Shadowsocks(t *testing.T) {
	in := fixture("shadowsocks", `{}`, `{"method": "chacha20-ietf-poly1305"}`)
	c := &runtime.Client{Password: "p"}

	o, _ := singboxOutbound("h", 8388, in, c, "ss")
	if o["type"] != "shadowsocks" {
		t.Errorf("type wrong")
	}
	if o["method"] != "chacha20-ietf-poly1305" {
		t.Errorf("method wrong")
	}
	if o["password"] != "p" {
		t.Errorf("password wrong")
	}
	if _, ok := o["transport"]; ok {
		t.Errorf("SS should not emit transport block")
	}
}

func TestSingboxOutbound_NoTransportForTCP(t *testing.T) {
	// VLESS / VMess / Trojan with network=tcp should not have a transport key.
	in := fixture("vless", `{"network": "tcp"}`, "")
	c := &runtime.Client{ID: "u"}
	o, _ := singboxOutbound("h", 443, in, c, "n")
	if _, ok := o["transport"]; ok {
		t.Errorf("tcp network should not emit transport block")
	}
}

func TestSingboxOutbound_UnsupportedProtocol(t *testing.T) {
	in := fixture("dokodemo-door", `{}`, "")
	c := &runtime.Client{}
	_, ok := singboxOutbound("h", 1, in, c, "x")
	if ok {
		t.Errorf("expected ok=false for unsupported")
	}
}
