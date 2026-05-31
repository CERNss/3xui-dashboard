package policy

import (
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestResolve_DefaultProfileGroupsAndRules(t *testing.T) {
	names := []string{"HK-1", "US-1"}
	r := Resolve(DefaultProfile(), names, RulesetMap(DefaultRulesets()))

	if len(r.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(r.Groups))
	}
	sel := r.Groups[0]
	if sel.Name != GroupSelect {
		t.Fatalf("group[0] = %q, want %q", sel.Name, GroupSelect)
	}
	// Include entries come first, then the nodes in order.
	if got, want := strings.Join(sel.Members, ","), GroupAuto+",DIRECT,HK-1,US-1"; got != want {
		t.Errorf("select members = %q, want %q", got, want)
	}
	auto := r.Groups[1]
	if auto.Type != model.GroupTypeURLTest {
		t.Errorf("auto type = %q, want url-test", auto.Type)
	}
	if got := strings.Join(auto.Members, ","); got != "HK-1,US-1" {
		t.Errorf("auto members = %q", got)
	}

	if len(r.Rulesets) != 13 {
		t.Errorf("referenced rulesets = %d, want 13", len(r.Rulesets))
	}
	last := r.Rules[len(r.Rules)-1]
	if last.Type != "MATCH" || last.Group != GroupSelect {
		t.Errorf("last rule = %+v, want MATCH -> %s", last, GroupSelect)
	}
}

func TestResolve_FilterIncludeExclude(t *testing.T) {
	profile := model.SubscriptionProfile{
		ProxyGroups: model.ProxyGroups{
			{Name: "US", Type: model.GroupTypeSelect, Filter: "^US", Include: []string{"DIRECT"}},
			{Name: "NotHK", Type: model.GroupTypeSelect, Exclude: "HK"},
		},
		Rules: model.Rules{{Kind: model.RuleKindInline, Matcher: "MATCH", Group: "US"}},
	}
	r := Resolve(profile, []string{"US-1", "US-2", "HK-1"}, nil)

	if got := strings.Join(r.Groups[0].Members, ","); got != "DIRECT,US-1,US-2" {
		t.Errorf("US members = %q (Include first, then ^US matches)", got)
	}
	if got := strings.Join(r.Groups[1].Members, ","); got != "US-1,US-2" {
		t.Errorf("NotHK members = %q (HK excluded)", got)
	}
}

func TestResolve_MatchSafetyNet(t *testing.T) {
	profile := model.SubscriptionProfile{
		ProxyGroups: model.ProxyGroups{{Name: "G", Type: model.GroupTypeSelect}},
		Rules:       model.Rules{{Kind: model.RuleKindInline, Matcher: "DOMAIN-SUFFIX", Value: "x.com", Group: "G"}},
	}
	r := Resolve(profile, []string{"n1"}, nil)
	last := r.Rules[len(r.Rules)-1]
	if last.Type != "MATCH" || last.Group != "G" {
		t.Errorf("expected appended MATCH -> G, got %+v", last)
	}
}

func TestResolve_RulesetDedup(t *testing.T) {
	rsmap := map[string]model.SubscriptionRuleset{
		"a": {Key: "a", SourceType: model.RulesetSourceRemote, URL: "http://x/a"},
	}
	profile := model.SubscriptionProfile{
		Rules: model.Rules{
			{Kind: model.RuleKindRuleset, RulesetKey: "a", Group: "G"},
			{Kind: model.RuleKindRuleset, RulesetKey: "a", Group: "H"},
		},
	}
	r := Resolve(profile, nil, rsmap)
	if len(r.Rulesets) != 1 {
		t.Errorf("ruleset refs = %d, want 1 (deduped)", len(r.Rulesets))
	}
	if len(r.Rules) != 2 {
		t.Errorf("rules = %d, want 2 (both RULE-SET lines kept)", len(r.Rules))
	}
}
