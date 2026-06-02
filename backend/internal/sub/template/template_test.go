package template

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sampleClashPolicy() ClashPolicy {
	return ClashPolicy{
		Groups: "proxy-groups:\n" +
			"  - name: 节点选择\n    type: select\n    proxies: [DIRECT, node-1, node-2]",
		RuleProviders: "rule-providers:\n" +
			"  proxy:\n    type: http\n    behavior: domain\n" +
			"    url: https://example.com/proxy.txt\n    path: ./ruleset/proxy.yaml\n    interval: 86400",
		Rules: "rules:\n  - RULE-SET,proxy,节点选择\n  - MATCH,节点选择",
	}
}

func TestRenderClash_SubstitutesPolicyBlocks(t *testing.T) {
	nodes := []map[string]any{
		{"name": "node-1", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"},
		{"name": "node-2", "type": "trojan", "server": "2.2.2.2", "port": 443, "password": "pw"},
	}
	out, err := RenderClash(nodes, sampleClashPolicy(), "")
	if err != nil {
		t.Fatalf("RenderClash: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid YAML: %v\n---\n%s", err, out)
	}
	for _, key := range []string{"proxies", "proxy-groups", "rule-providers", "rules"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("missing %q key", key)
		}
	}
	dns, ok := doc["dns"].(map[string]any)
	if !ok {
		t.Fatalf("dns is not a YAML map")
	}
	proxyServerNameservers, ok := dns["proxy-server-nameserver"].([]any)
	if !ok {
		t.Fatalf("dns.proxy-server-nameserver is not a YAML list")
	}
	gotNameservers := map[string]bool{}
	for _, ns := range proxyServerNameservers {
		if s, ok := ns.(string); ok {
			gotNameservers[s] = true
		}
	}
	for _, want := range []string{"223.5.5.5", "119.29.29.29"} {
		if !gotNameservers[want] {
			t.Errorf("dns.proxy-server-nameserver missing %q", want)
		}
	}
	if !strings.Contains(string(out), "node-1") || !strings.Contains(string(out), "node-2") {
		t.Errorf("output does not contain expected node names")
	}
}

func TestRenderClash_EmptyRuleProvidersOmitsSection(t *testing.T) {
	nodes := []map[string]any{{"name": "n", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"}}
	pol := ClashPolicy{
		Groups: "proxy-groups:\n  - name: G\n    type: select\n    proxies: [DIRECT, n]",
		Rules:  "rules:\n  - MATCH,G",
	}
	out, err := RenderClash(nodes, pol, "")
	if err != nil {
		t.Fatalf("RenderClash: %v", err)
	}
	if strings.Contains(string(out), "rule-providers:") {
		t.Errorf("empty RuleProviders should not emit a rule-providers section")
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid YAML: %v\n---\n%s", err, out)
	}
}

func TestRenderClash_OperatorBaseUsesPlaceholders(t *testing.T) {
	nodes := []map[string]any{
		{"name": "node-1", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"},
	}
	base := "mode: rule\n" +
		"proxies:\n${proxies}\n" +
		"proxy-groups:\n  - name: PROXY\n    type: select\n    proxies: [${proxy_names}]\n" +
		"rules:\n  - MATCH,PROXY\n"
	out, err := RenderClash(nodes, ClashPolicy{}, base)
	if err != nil {
		t.Fatalf("RenderClash operator base: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: PROXY") || !strings.Contains(s, "node-1") {
		t.Errorf("operator base did not substitute placeholders:\n%s", s)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("operator output not valid YAML: %v\n---\n%s", err, s)
	}
}

func TestRenderClash_BadOperatorBaseFails(t *testing.T) {
	nodes := []map[string]any{{"name": "n", "type": "vless"}}
	if _, err := RenderClash(nodes, ClashPolicy{}, "this: is: not: valid: yaml::"); err == nil {
		t.Fatalf("expected parse error on broken operator base, got nil")
	}
}

func TestRenderClash_EmptyNodes(t *testing.T) {
	out, err := RenderClash(nil, ClashPolicy{}, "")
	if err != nil {
		t.Fatalf("RenderClash empty: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("empty render not valid YAML: %v\n---\n%s", err, out)
	}
}

func TestRenderSingBox_DefaultParses(t *testing.T) {
	outs := []map[string]any{
		{"tag": "node-1", "type": "vless", "server": "1.1.1.1", "server_port": 443, "uuid": "x"},
		{"tag": "node-2", "type": "shadowsocks", "server": "2.2.2.2", "server_port": 8388, "method": "aes-256-gcm", "password": "pw"},
	}
	out, err := RenderSingBox(outs, "")
	if err != nil {
		t.Fatalf("RenderSingBox: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v\n---\n%s", err, out)
	}
	obs, ok := doc["outbounds"].([]any)
	if !ok {
		t.Fatalf("outbounds is not a JSON array")
	}
	if len(obs) < 4 {
		t.Errorf("expected ≥4 outbounds, got %d", len(obs))
	}
}
