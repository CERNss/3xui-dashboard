package sub

import (
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/sub/policy"
)

func TestFormatSurge_DefaultProfileSkipsVLESS(t *testing.T) {
	a := &Assembler{}
	vless := fixture("vless", `{"network":"tcp","security":"reality"}`, "")
	trojan := fixture("trojan", `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"t.example.com"}}`, "")
	d := &SubscriptionData{Links: []Link{
		{Protocol: "vless", Host: "1.1.1.1", Port: 443, Inbound: vless, Client: &runtime.Client{ID: "u1"}, Remark: "VL-1"},
		{Protocol: "trojan", Host: "2.2.2.2", Port: 443, Inbound: trojan, Client: &runtime.Client{Password: "pw"}, Remark: "TJ-1"},
	}}

	out, err := a.FormatSurge(d, policy.DefaultProfile(), policy.DefaultRulesets(), "", "")
	if err != nil {
		t.Fatalf("FormatSurge: %v", err)
	}
	s := string(out)
	for _, want := range []string{"[Proxy]", "[Proxy Group]", "[Rule]", "TJ-1 = trojan", "RULE-SET,", "FINAL,节点选择"} {
		if !strings.Contains(s, want) {
			t.Errorf("surge config missing %q\n%s", want, s)
		}
	}
	if strings.Contains(s, "VL-1") {
		t.Errorf("VLESS node must be skipped in Surge output:\n%s", s)
	}
}

func TestFormatSurge_SelfHostedRuleSetURL(t *testing.T) {
	a := &Assembler{}
	trojan := fixture("trojan", `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"t"}}`, "")
	d := &SubscriptionData{Links: []Link{
		{Protocol: "trojan", Host: "h", Port: 443, Inbound: trojan, Client: &runtime.Client{Password: "pw"}, Remark: "TJ-1"},
	}}
	profile := policy.DefaultProfile()
	profile.RulesetMode = model.RulesetModeSelfHosted

	out, err := a.FormatSurge(d, profile, policy.DefaultRulesets(), "", "https://panel.example.com")
	if err != nil {
		t.Fatalf("FormatSurge: %v", err)
	}
	if !strings.Contains(string(out), "RULE-SET,https://panel.example.com/sub/ruleset/proxy,节点选择") {
		t.Errorf("self-hosted RULE-SET URL missing:\n%s", out)
	}
}

func TestFormatSurge_VLESSOnlyFleetIsMinimal(t *testing.T) {
	a := &Assembler{}
	vless := fixture("vless", `{"network":"tcp"}`, "")
	d := &SubscriptionData{Links: []Link{
		{Protocol: "vless", Host: "h", Port: 443, Inbound: vless, Client: &runtime.Client{ID: "u"}, Remark: "VL-1"},
	}}
	out, err := a.FormatSurge(d, policy.DefaultProfile(), policy.DefaultRulesets(), "", "")
	if err != nil {
		t.Fatalf("FormatSurge: %v", err)
	}
	if !strings.Contains(string(out), "FINAL,") {
		t.Errorf("VLESS-only fleet should still yield a minimal valid Surge config:\n%s", out)
	}
}
