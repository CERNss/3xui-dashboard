package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

func hysteriaTestInbound(sni string, version int) *runtime.Inbound {
	streamSettings := `{
		"network": "hysteria",
		"security": "tls",
		"tlsSettings": {"serverName": "` + sni + `", "allowInsecure": false, "alpn": ["h3"]},
		"hysteriaSettings": {"version": ` + intToStr(version) + `, "udpIdleTimeout": 60}
	}`
	return &runtime.Inbound{
		ID:             5,
		Tag:            "hys-1",
		Protocol:       "hysteria",
		Port:           34587,
		Remark:         "tokyo-hys",
		StreamSettings: streamSettings,
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	buf := []byte{}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func TestHysteriaLink_HappyPath(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 2)
	c := &runtime.Client{Auth: "V5iNls6qQa", Email: "q1h291un"}
	got := BuildLink("vpn.example.com", in.Port, in, c, "TKY · q1h291un")

	if !strings.HasPrefix(got, "hysteria2://") {
		t.Fatalf("URI must start with hysteria2:// — got %q", got)
	}
	if !strings.Contains(got, "@vpn.example.com:34587/") {
		t.Errorf("host/port wrong: %q", got)
	}
	if !strings.Contains(got, "sni=vpn.example.com") {
		t.Errorf("missing sni query: %q", got)
	}
	if !strings.Contains(got, "alpn=h3") {
		t.Errorf("missing alpn=h3: %q", got)
	}
	if !strings.Contains(got, "insecure=0") {
		t.Errorf("missing insecure=0: %q", got)
	}
	if !strings.HasSuffix(got, "#TKY+%C2%B7+q1h291un") {
		// QueryEscape uses '+' for space and percent-encodes ·.
		t.Errorf("remark fragment wrong: %q", got)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("URI doesn't parse: %v", err)
	}
	if u.User.Username() != "V5iNls6qQa" {
		t.Errorf("auth not in userinfo: %q", u.User.Username())
	}
}

func TestHysteriaLink_EmptyAuthReturnsEmpty(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 2)
	c := &runtime.Client{Auth: "", Email: "q"}
	if got := BuildLink("vpn.example.com", in.Port, in, c, "x"); got != "" {
		t.Errorf("expected empty link for unkeyed client, got %q", got)
	}
}

func TestHysteriaLink_V1Skipped(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 1)
	c := &runtime.Client{Auth: "k", Email: "e"}
	if got := BuildLink("vpn.example.com", in.Port, in, c, "x"); got != "" {
		t.Errorf("v1 should be skipped (URI builder emits v2 only), got %q", got)
	}
}

func TestClashHysteria2_FieldShape(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 2)
	c := &runtime.Client{Auth: "secret", Email: "e"}
	n, ok := clashNode("vpn.example.com", in.Port, in, c, "tag")
	if !ok || n == nil {
		t.Fatalf("clashNode returned nil/false for hysteria")
	}
	if n["type"] != "hysteria2" {
		t.Errorf("type = %v, want hysteria2", n["type"])
	}
	if n["password"] != "secret" {
		t.Errorf("password = %v, want secret", n["password"])
	}
	if n["sni"] != "vpn.example.com" {
		t.Errorf("sni = %v, want vpn.example.com", n["sni"])
	}
	alpn, _ := n["alpn"].([]string)
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn = %v, want [h3]", n["alpn"])
	}
}

func TestClashHysteria2_AllowInsecureFlows(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 2)
	// Insert allowInsecure=true into the streamSettings.
	in.StreamSettings = strings.Replace(in.StreamSettings, `"allowInsecure": false`, `"allowInsecure": true`, 1)
	c := &runtime.Client{Auth: "secret", Email: "e"}
	n, _ := clashNode("vpn.example.com", in.Port, in, c, "tag")
	if got, ok := n["skip-cert-verify"].(bool); !ok || !got {
		t.Errorf("skip-cert-verify = %v, want true", n["skip-cert-verify"])
	}
}

func TestSingboxHysteria2_FieldShape(t *testing.T) {
	in := hysteriaTestInbound("vpn.example.com", 2)
	c := &runtime.Client{Auth: "secret", Email: "e"}
	o, ok := singboxOutbound("vpn.example.com", in.Port, in, c, "tagname")
	if !ok || o == nil {
		t.Fatalf("singboxOutbound returned nil/false for hysteria")
	}
	if o["type"] != "hysteria2" || o["tag"] != "tagname" {
		t.Errorf("type/tag wrong: %v", o)
	}
	if o["password"] != "secret" {
		t.Errorf("password = %v, want secret", o["password"])
	}
	tls, _ := o["tls"].(map[string]any)
	if tls == nil {
		t.Fatal("tls block missing")
	}
	if tls["server_name"] != "vpn.example.com" {
		t.Errorf("tls.server_name = %v, want vpn.example.com", tls["server_name"])
	}
	alpn, _ := tls["alpn"].([]string)
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("tls.alpn = %v, want [h3]", tls["alpn"])
	}
}

func TestSingboxHysteria2_EmptySNIFallsBackToHost(t *testing.T) {
	in := hysteriaTestInbound("", 2)
	c := &runtime.Client{Auth: "k", Email: "e"}
	o, _ := singboxOutbound("realhost.example.com", in.Port, in, c, "t")
	tls, _ := o["tls"].(map[string]any)
	if tls["server_name"] != "realhost.example.com" {
		t.Errorf("empty SNI should fall back to host, got %v", tls["server_name"])
	}
}
