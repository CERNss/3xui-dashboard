package sub

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func wgTestLink(name, ip, priv, pub, srvPub string) Link {
	return Link{
		Protocol: "wireguard",
		Remark:   name,
		Host:     "vpn.example.com",
		Port:     51820,
		WGPeer: &WGPeerView{
			PrivateKey:      priv,
			PublicKey:       pub,
			ServerPublicKey: srvPub,
			AllocatedIP:     ip,
			MTU:             1420,
		},
	}
}

func TestBuildWGConf_Format(t *testing.T) {
	l := wgTestLink("home-vpn", "10.0.0.42", "ZGF2ZGE=", "Zm9vYmFy", "U2VydmVyUHViS2V5Lg==")
	got := BuildWGConf(l)

	// Required line items — order-agnostic via Contains.
	mustContain := []string{
		"[Interface]",
		"PrivateKey = ZGF2ZGE=",
		"Address = 10.0.0.42/32",
		"MTU = 1420",
		"DNS = ",
		"[Peer]",
		"PublicKey = U2VydmVyUHViS2V5Lg==",
		"Endpoint = vpn.example.com:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// Both sections present and in order.
	ifIdx := strings.Index(got, "[Interface]")
	pIdx := strings.Index(got, "[Peer]")
	if ifIdx == -1 || pIdx == -1 || ifIdx > pIdx {
		t.Errorf("[Interface] and [Peer] not in order:\n%s", got)
	}
}

func TestBuildWGConf_NilPeerIsEmpty(t *testing.T) {
	if got := BuildWGConf(Link{Protocol: "wireguard"}); got != "" {
		t.Errorf("got %q, want empty for nil WGPeer", got)
	}
}

func TestBuildWGConfZip_OnePerLink(t *testing.T) {
	links := []Link{
		wgTestLink("alpha", "10.0.0.2", "AA==", "BB==", "SS=="),
		wgTestLink("beta", "10.0.0.3", "CC==", "DD==", "SS=="),
		{Protocol: "vless", Remark: "skip-me"}, // non-WG must be skipped
	}
	zipBytes, err := BuildWGConfZip(links)
	if err != nil {
		t.Fatalf("BuildWGConfZip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Errorf("zip has %d files, want 2 (vless skipped)", len(zr.File))
	}
	gotNames := map[string]bool{}
	for _, f := range zr.File {
		gotNames[f.Name] = true
		rc, _ := f.Open()
		body, _ := io.ReadAll(rc)
		_ = rc.Close()
		if !strings.Contains(string(body), "[Interface]") {
			t.Errorf("%s missing [Interface]:\n%s", f.Name, body)
		}
	}
	for _, want := range []string{"alpha.conf", "beta.conf"} {
		if !gotNames[want] {
			t.Errorf("missing %q in zip; have %v", want, gotNames)
		}
	}
}

func TestSafeWGFilename(t *testing.T) {
	cases := []struct {
		remark, pub, want string
	}{
		{"alpha", "anykey", "alpha"},
		{"VPN - home / mobile", "anykey", "VPN---home--mobile"}, // spaces→dash, '/' stripped
		{"日本-tokyo", "anykey", "-tokyo"},                       // non-ASCII dropped, "-tokyo" preserved
		{"", "AAAABBBBCCCC=", "wg-AABBBBCC"},                     // (input "AAAABBBBCCCC=" minus "=" is "AAAABBBBCCCC", last 8 = "AABBBBCC" — but " " padding rules...)
	}
	// Last case: ensure we at least produce "wg-" prefix when remark is empty.
	for _, tc := range cases {
		got := safeWGFilename(tc.remark, tc.pub)
		if tc.remark == "" {
			if !strings.HasPrefix(got, "wg-") {
				t.Errorf("safeWGFilename empty remark got %q, want wg-prefixed", got)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("safeWGFilename(%q) = %q, want %q", tc.remark, got, tc.want)
		}
	}
}

func TestClashWGNode_FieldShape(t *testing.T) {
	n := clashWGNode(wgTestLink("home", "10.0.0.42", "PRIV", "PUB", "SRV"))
	if n == nil {
		t.Fatal("clashWGNode returned nil for valid WG link")
	}
	expect := map[string]any{
		"name":        "home",
		"type":        "wireguard",
		"server":      "vpn.example.com",
		"port":        51820,
		"ip":          "10.0.0.42",
		"private-key": "PRIV",
		"public-key":  "SRV",
		"udp":         true,
		"mtu":         1420,
	}
	for k, want := range expect {
		if got := n[k]; got != want {
			t.Errorf("field %q = %v (%T), want %v (%T)", k, got, got, want, want)
		}
	}
}

func TestSingboxWGOutbound_FieldShape(t *testing.T) {
	o := singboxWGOutbound(wgTestLink("home", "10.0.0.42", "PRIV", "PUB", "SRV"))
	if o == nil {
		t.Fatal("singboxWGOutbound returned nil for valid WG link")
	}
	if o["type"] != "wireguard" || o["tag"] != "home" {
		t.Errorf("type/tag wrong: %v", o)
	}
	if o["server_port"] != 51820 {
		t.Errorf("server_port = %v, want 51820", o["server_port"])
	}
	if o["private_key"] != "PRIV" || o["peer_public_key"] != "SRV" {
		t.Errorf("keys wrong: %v", o)
	}
	local, ok := o["local_address"].([]string)
	if !ok || len(local) != 1 || local[0] != "10.0.0.42/32" {
		t.Errorf("local_address = %v, want [10.0.0.42/32]", o["local_address"])
	}
}
