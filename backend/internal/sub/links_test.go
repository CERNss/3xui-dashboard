package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

func TestVLESS_TLS_WS_Link(t *testing.T) {
	stream, _ := json.Marshal(map[string]any{
		"network":  "ws",
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName": "sni.example.com",
			"fingerprint": "chrome",
		},
		"wsSettings": map[string]any{
			"path":    "/cool",
			"headers": map[string]any{"Host": "host.example.com"},
		},
	})
	in := &runtime.Inbound{Protocol: "vless", Port: 443, StreamSettings: string(stream)}
	c := &runtime.Client{ID: "uuid-1", Flow: "xtls-rprx-vision", Email: "alice"}

	got := BuildLink("server.example.com", 443, in, c, "alice@server")
	if !strings.HasPrefix(got, "vless://uuid-1@server.example.com:443?") {
		t.Errorf("link prefix wrong: %s", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	for _, kv := range []struct{ k, want string }{
		{"type", "ws"},
		{"security", "tls"},
		{"sni", "sni.example.com"},
		{"fp", "chrome"},
		{"flow", "xtls-rprx-vision"},
		{"path", "/cool"},
		{"host", "host.example.com"},
	} {
		if got := q.Get(kv.k); got != kv.want {
			t.Errorf("q[%s] = %q, want %q", kv.k, got, kv.want)
		}
	}
	if u.Fragment != "alice@server" {
		t.Errorf("fragment = %q", u.Fragment)
	}
}

func TestVLESS_Reality(t *testing.T) {
	stream, _ := json.Marshal(map[string]any{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]any{
			"publicKey":   "PUB",
			"shortIds":    []any{"abcd"},
			"serverNames": []any{"www.cloudflare.com"},
			"fingerprint": "chrome",
		},
	})
	in := &runtime.Inbound{Protocol: "vless", Port: 443, StreamSettings: string(stream)}
	c := &runtime.Client{ID: "uuid-x", Email: "bob"}
	got := BuildLink("1.2.3.4", 443, in, c, "bob@reality")
	u, _ := url.Parse(got)
	q := u.Query()
	for _, kv := range []struct{ k, want string }{
		{"security", "reality"},
		{"pbk", "PUB"},
		{"sid", "abcd"},
		{"sni", "www.cloudflare.com"},
		{"fp", "chrome"},
	} {
		if got := q.Get(kv.k); got != kv.want {
			t.Errorf("reality q[%s] = %q, want %q", kv.k, got, kv.want)
		}
	}
}

func TestVMess_LinkIsBase64JSON(t *testing.T) {
	in := &runtime.Inbound{Protocol: "vmess", Port: 80}
	c := &runtime.Client{ID: "uuid-v", Email: "carol"}
	got := BuildLink("vm.example.com", 80, in, c, "carol@vmess")
	if !strings.HasPrefix(got, "vmess://") {
		t.Fatalf("vmess link missing prefix: %s", got)
	}
	payload := strings.TrimPrefix(got, "vmess://")
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode vmess base64: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(decoded, &obj); err != nil {
		t.Fatalf("vmess inner json: %v", err)
	}
	if obj["add"] != "vm.example.com" || obj["id"] != "uuid-v" || obj["ps"] != "carol@vmess" {
		t.Errorf("vmess payload mismatch: %+v", obj)
	}
}

func TestTrojanAndShadowsocks(t *testing.T) {
	tro := BuildLink("tr.example.com", 443,
		&runtime.Inbound{Protocol: "trojan"},
		&runtime.Client{Password: "pwd"}, "t1")
	if !strings.HasPrefix(tro, "trojan://pwd@tr.example.com:443") {
		t.Errorf("trojan link: %s", tro)
	}
	ss := BuildLink("ss.example.com", 8388,
		&runtime.Inbound{Protocol: "shadowsocks", Settings: `{"method":"aes-128-gcm"}`},
		&runtime.Client{Password: "pw"}, "s1")
	if !strings.HasPrefix(ss, "ss://") {
		t.Errorf("ss link: %s", ss)
	}
}

func TestBuildLink_UnknownProtocolReturnsEmpty(t *testing.T) {
	got := BuildLink("h", 1, &runtime.Inbound{Protocol: "wireguard"}, &runtime.Client{}, "x")
	if got != "" {
		t.Errorf("unknown protocol should return empty, got %q", got)
	}
}

func TestFormatRemark(t *testing.T) {
	got := formatRemark("-ieo", "node1", "vless-tls", "vless-1", "alice@x.com")
	if got != "vless-tls - alice@x.com - node1" {
		t.Errorf("got %q", got)
	}
	got = formatRemark("-oti", "node2", "trojan", "trojan-1", "")
	if got != "node2 - trojan-1 - trojan" {
		t.Errorf("got %q", got)
	}
}
