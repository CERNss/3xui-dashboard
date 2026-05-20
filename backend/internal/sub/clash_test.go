package sub

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// fixture builds an Inbound with the given protocol + streamSettings JSON.
// Settings string left empty; pass a value only when the per-protocol path
// reads from in.Settings (Shadowsocks).
func fixture(protocol, streamSettings, settings string) *runtime.Inbound {
	return &runtime.Inbound{
		Protocol:       protocol,
		StreamSettings: streamSettings,
		Settings:       settings,
		Tag:            "test",
		Port:           443,
	}
}

func TestClashNode_VLESSReality(t *testing.T) {
	in := fixture("vless", `{
		"network": "tcp",
		"security": "reality",
		"realitySettings": {
			"publicKey": "PUB_KEY_HERE",
			"shortIds": ["abcd1234"],
			"serverNames": ["example.com"],
			"fingerprint": "chrome"
		}
	}`, "")
	c := &runtime.Client{ID: "11111111-1111-1111-1111-111111111111", Flow: "xtls-rprx-vision"}

	node, ok := clashNode("1.2.3.4", 443, in, c, "vless-reality")
	if !ok {
		t.Fatalf("clashNode returned ok=false")
	}
	if node["type"] != "vless" {
		t.Errorf("type = %v, want vless", node["type"])
	}
	if node["uuid"] != c.ID {
		t.Errorf("uuid = %v, want %v", node["uuid"], c.ID)
	}
	if node["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow missing or wrong: %v", node["flow"])
	}
	if node["tls"] != true {
		t.Errorf("tls flag not set for reality")
	}
	if node["servername"] != "example.com" {
		t.Errorf("servername = %v, want example.com", node["servername"])
	}
	ro, ok := node["reality-opts"].(map[string]any)
	if !ok {
		t.Fatalf("reality-opts missing")
	}
	if ro["public-key"] != "PUB_KEY_HERE" {
		t.Errorf("public-key wrong: %v", ro["public-key"])
	}
	if ro["short-id"] != "abcd1234" {
		t.Errorf("short-id wrong: %v", ro["short-id"])
	}
}

func TestClashNode_VLESSwsTLS(t *testing.T) {
	in := fixture("vless", `{
		"network": "ws",
		"security": "tls",
		"tlsSettings": {"serverName": "ws.example.com", "alpn": ["h2", "http/1.1"]},
		"wsSettings": {"path": "/ray", "headers": {"Host": "ws.example.com"}}
	}`, "")
	c := &runtime.Client{ID: "uuid-here"}

	node, _ := clashNode("host", 8443, in, c, "ws+tls")
	if node["network"] != "ws" {
		t.Errorf("network = %v, want ws", node["network"])
	}
	if node["tls"] != true {
		t.Errorf("tls not set")
	}
	ws, ok := node["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("ws-opts missing")
	}
	if ws["path"] != "/ray" {
		t.Errorf("ws path = %v", ws["path"])
	}
	hdr, _ := ws["headers"].(map[string]any)
	if hdr["Host"] != "ws.example.com" {
		t.Errorf("ws Host header wrong: %v", hdr["Host"])
	}
}

func TestClashNode_VLESSgRPC(t *testing.T) {
	in := fixture("vless", `{
		"network": "grpc",
		"security": "tls",
		"tlsSettings": {"serverName": "g.example.com"},
		"grpcSettings": {"serviceName": "my-svc"}
	}`, "")
	c := &runtime.Client{ID: "u"}

	node, _ := clashNode("h", 443, in, c, "g")
	g, ok := node["grpc-opts"].(map[string]any)
	if !ok {
		t.Fatalf("grpc-opts missing")
	}
	if g["grpc-service-name"] != "my-svc" {
		t.Errorf("grpc-service-name wrong: %v", g["grpc-service-name"])
	}
}

func TestClashNode_VMess(t *testing.T) {
	in := fixture("vmess", `{"network": "tcp"}`, "")
	c := &runtime.Client{ID: "v-uuid"}

	node, _ := clashNode("h", 443, in, c, "vmess")
	if node["type"] != "vmess" {
		t.Errorf("type wrong")
	}
	if node["alterId"] != 0 {
		t.Errorf("alterId must be 0 for modern AEAD VMess, got %v", node["alterId"])
	}
	if node["cipher"] != "auto" {
		t.Errorf("cipher should be auto, got %v", node["cipher"])
	}
}

func TestClashNode_Trojan(t *testing.T) {
	in := fixture("trojan", `{
		"network": "tcp",
		"security": "tls",
		"tlsSettings": {"serverName": "t.example.com"}
	}`, "")
	c := &runtime.Client{Password: "trojan-pw"}

	node, _ := clashNode("h", 443, in, c, "trojan")
	if node["type"] != "trojan" {
		t.Errorf("type wrong")
	}
	if node["password"] != "trojan-pw" {
		t.Errorf("password missing")
	}
	if node["sni"] != "t.example.com" {
		t.Errorf("sni wrong: %v", node["sni"])
	}
	if node["skip-cert-verify"] != false {
		t.Errorf("skip-cert-verify should be false")
	}
}

func TestClashNode_Shadowsocks(t *testing.T) {
	// SS stores method on the inbound settings; password on the client.
	in := fixture("shadowsocks", `{}`, `{"method": "aes-256-gcm"}`)
	c := &runtime.Client{Password: "ss-pw"}

	node, _ := clashNode("h", 8388, in, c, "ss")
	if node["type"] != "ss" {
		t.Errorf("type wrong")
	}
	if node["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher = %v, want aes-256-gcm", node["cipher"])
	}
	if node["password"] != "ss-pw" {
		t.Errorf("password = %v", node["password"])
	}
}

func TestClashNode_UnsupportedProtocol(t *testing.T) {
	in := fixture("dokodemo-door", `{}`, "")
	c := &runtime.Client{}
	_, ok := clashNode("h", 1234, in, c, "x")
	if ok {
		t.Errorf("expected ok=false for unsupported protocol")
	}
}

func TestClashNode_UDPDefaultsTrue(t *testing.T) {
	// Every protocol's clash node should default udp:true (per Mihomo convention).
	for _, p := range []string{"vless", "vmess", "trojan", "shadowsocks"} {
		var in *runtime.Inbound
		if p == "shadowsocks" {
			in = fixture(p, `{}`, `{"method": "aes-256-gcm"}`)
		} else {
			in = fixture(p, `{"network": "tcp"}`, "")
		}
		c := &runtime.Client{ID: "x", Password: "x"}
		node, _ := clashNode("h", 1, in, c, "n")
		if node["udp"] != true {
			t.Errorf("%s: udp != true (got %v)", p, node["udp"])
		}
	}
}
