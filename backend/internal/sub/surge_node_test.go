package sub

import (
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

func TestSurgeNode_SkipsVLESS(t *testing.T) {
	in := fixture("vless", `{"network":"tcp","security":"reality"}`, "")
	if _, ok := surgeNode("h", 443, in, &runtime.Client{ID: "u"}); ok {
		t.Fatal("Surge cannot speak VLESS — surgeNode should return ok=false so it's skipped")
	}
}

func TestSurgeNode_Trojan(t *testing.T) {
	in := fixture("trojan", `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"t.example.com"}}`, "")
	line, ok := surgeNode("1.2.3.4", 443, in, &runtime.Client{Password: "pw"})
	if !ok {
		t.Fatal("trojan should render")
	}
	for _, want := range []string{"trojan, 1.2.3.4, 443", "password=pw", "sni=t.example.com"} {
		if !strings.Contains(line, want) {
			t.Errorf("trojan line missing %q: %s", want, line)
		}
	}
}

func TestSurgeNode_VMessWS(t *testing.T) {
	in := fixture("vmess", `{"network":"ws","security":"tls","tlsSettings":{"serverName":"v.example.com"},"wsSettings":{"path":"/ray","headers":{"Host":"v.example.com"}}}`, "")
	line, ok := surgeNode("h", 8443, in, &runtime.Client{ID: "uuid-1"})
	if !ok {
		t.Fatal("vmess should render")
	}
	for _, want := range []string{"vmess, h, 8443", "username=uuid-1", "vmess-aead=true", "tls=true", "ws=true", "ws-path=/ray", "ws-headers=Host:v.example.com"} {
		if !strings.Contains(line, want) {
			t.Errorf("vmess line missing %q: %s", want, line)
		}
	}
}

func TestSurgeNode_Shadowsocks(t *testing.T) {
	in := fixture("shadowsocks", `{}`, `{"method":"aes-256-gcm"}`)
	line, ok := surgeNode("h", 8388, in, &runtime.Client{Password: "ss-pw"})
	if !ok {
		t.Fatal("ss should render")
	}
	for _, want := range []string{"ss, h, 8388", "encrypt-method=aes-256-gcm", "password=ss-pw"} {
		if !strings.Contains(line, want) {
			t.Errorf("ss line missing %q: %s", want, line)
		}
	}
}

func TestSurgeNode_Hysteria2NeedsAuth(t *testing.T) {
	in := fixture("hysteria2", `{"tlsSettings":{"serverName":"hy.example.com"}}`, "")
	if _, ok := surgeNode("h", 443, in, &runtime.Client{}); ok {
		t.Fatal("hysteria2 with no auth should not render")
	}
	line, ok := surgeNode("h", 443, in, &runtime.Client{Auth: "secret"})
	if !ok {
		t.Fatal("hysteria2 with auth should render")
	}
	for _, want := range []string{"hysteria2, h, 443", "password=secret", "sni=hy.example.com"} {
		if !strings.Contains(line, want) {
			t.Errorf("hysteria2 line missing %q: %s", want, line)
		}
	}
}
