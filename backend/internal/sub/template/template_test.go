package template

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderClash_DefaultParses(t *testing.T) {
	nodes := []map[string]any{
		{"name": "node-1", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"},
		{"name": "node-2", "type": "trojan", "server": "2.2.2.2", "port": 443, "password": "pw"},
	}
	out, err := RenderClash(nodes, Options{RuleProvidersEnabled: true})
	if err != nil {
		t.Fatalf("RenderClash: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid YAML: %v\n---\n%s", err, out)
	}

	// Must contain proxies + proxy-groups + rules
	if _, ok := doc["proxies"]; !ok {
		t.Errorf("missing proxies key")
	}
	if _, ok := doc["proxy-groups"]; !ok {
		t.Errorf("missing proxy-groups key")
	}
	if _, ok := doc["rules"]; !ok {
		t.Errorf("missing rules (rule_providers_enabled=true)")
	}
	if !strings.Contains(string(out), "node-1") || !strings.Contains(string(out), "node-2") {
		t.Errorf("output does not contain expected node names")
	}
}

func TestRenderClash_RuleProvidersDisabled(t *testing.T) {
	nodes := []map[string]any{
		{"name": "n", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"},
	}
	out, err := RenderClash(nodes, Options{RuleProvidersEnabled: false})
	if err != nil {
		t.Fatalf("RenderClash: %v", err)
	}
	if strings.Contains(string(out), "rule-providers:") {
		t.Errorf("rule_providers_enabled=false should strip rule-providers")
	}
	if strings.Contains(string(out), "loyalsoldier") {
		t.Errorf("ruleset URLs leaked despite disabled flag")
	}
	// Should still be valid YAML
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid YAML: %v\n---\n%s", err, out)
	}
}

func TestRenderClash_StrategyAutoOnly(t *testing.T) {
	nodes := []map[string]any{
		{"name": "n", "type": "vless", "server": "1.1.1.1", "port": 443, "uuid": "x"},
	}
	out, err := RenderClash(nodes, Options{ProxyGroupStrategy: "auto-only", RuleProvidersEnabled: true})
	if err != nil {
		t.Fatalf("RenderClash: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: 自动选择") {
		t.Errorf("auto-only strategy missing 自动选择 group")
	}
	if strings.Contains(s, "name: 节点选择") {
		t.Errorf("auto-only should not have 节点选择 group")
	}
}

func TestRenderClash_BadOperatorTemplateFails(t *testing.T) {
	nodes := []map[string]any{{"name": "n", "type": "vless"}}
	_, err := RenderClash(nodes, Options{ClashTemplate: "this: is: not: valid: yaml::"})
	if err == nil {
		t.Fatalf("expected parse error on broken operator template, got nil")
	}
}

func TestRenderSingBox_DefaultParses(t *testing.T) {
	outs := []map[string]any{
		{"tag": "node-1", "type": "vless", "server": "1.1.1.1", "server_port": 443, "uuid": "x"},
		{"tag": "node-2", "type": "shadowsocks", "server": "2.2.2.2", "server_port": 8388, "method": "aes-256-gcm", "password": "pw"},
	}
	out, err := RenderSingBox(outs, Options{})
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
		t.Errorf("expected ≥4 outbounds (selector + urltest + nodes + direct + block + dns), got %d", len(obs))
	}
}

func TestRenderClash_EmptyNodes(t *testing.T) {
	out, err := RenderClash(nil, Options{RuleProvidersEnabled: true})
	if err != nil {
		t.Fatalf("RenderClash empty: %v", err)
	}
	// Should still be valid YAML even with no nodes
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("empty render not valid YAML: %v\n---\n%s", err, out)
	}
}
