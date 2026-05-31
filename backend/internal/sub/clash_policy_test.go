package sub

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cern/3xui-dashboard/internal/sub/policy"
)

func TestClashPolicyBlocks_DefaultComposesValidYAML(t *testing.T) {
	r := policy.Resolve(policy.DefaultProfile(), []string{"HK-1", "US-1"}, policy.RulesetMap(policy.DefaultRulesets()))

	groups := clashProxyGroupsYAML(r)
	providers := clashRuleProvidersYAML(r)
	rules := clashRulesYAML(r)

	for _, want := range []string{"name: " + policy.GroupSelect, "name: " + policy.GroupAuto, "type: url-test", "tolerance: 50"} {
		if !strings.Contains(groups, want) {
			t.Errorf("groups block missing %q\n%s", want, groups)
		}
	}
	for _, want := range []string{"rule-providers:", "reject:", "behavior: domain", "cncidr:", "behavior: ipcidr", "Loyalsoldier"} {
		if !strings.Contains(providers, want) {
			t.Errorf("providers block missing %q\n%s", want, providers)
		}
	}
	for _, want := range []string{"DOMAIN-SUFFIX,local,DIRECT", "RULE-SET,proxy,节点选择", "GEOIP,CN,DIRECT", "MATCH,节点选择"} {
		if !strings.Contains(rules, want) {
			t.Errorf("rules block missing %q\n%s", want, rules)
		}
	}

	// The three policy blocks must compose into valid Clash YAML.
	doc := "proxies: []\n" + groups + "\n" + providers + "\n" + rules + "\n"
	var probe map[string]any
	if err := yaml.Unmarshal([]byte(doc), &probe); err != nil {
		t.Fatalf("composed policy blocks are not valid YAML: %v\n---\n%s", err, doc)
	}
	for _, key := range []string{"proxy-groups", "rule-providers", "rules"} {
		if _, ok := probe[key]; !ok {
			t.Errorf("composed doc missing %q key", key)
		}
	}
}

func TestClashFlowList_QuotesAndEmpty(t *testing.T) {
	if got := clashFlowList(nil); got != "DIRECT" {
		t.Errorf("empty membership = %q, want DIRECT (never proxy-less)", got)
	}
	got := clashFlowList([]string{"plain", "comma,name"})
	if !strings.Contains(got, "plain") {
		t.Errorf("plain name should pass through: %q", got)
	}
	if !strings.Contains(got, `"comma,name"`) {
		t.Errorf("name with comma must be quoted: %q", got)
	}
}

func TestClashRuleProviders_EmptyWhenNoRemote(t *testing.T) {
	if got := clashRuleProvidersYAML(policy.Resolved{}); got != "" {
		t.Errorf("no rulesets should yield empty providers block, got %q", got)
	}
}
