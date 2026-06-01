package admin

import (
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestValidateProfile(t *testing.T) {
	base := model.SubscriptionProfile{
		Key:         "default",
		RulesetMode: model.RulesetModePassthrough,
		ProxyGroups: model.ProxyGroups{{Name: "G", Type: model.GroupTypeSelect}},
		Rules:       model.Rules{{Kind: model.RuleKindInline, Matcher: "MATCH", Group: "G"}},
	}
	if err := validateProfile(&base); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*model.SubscriptionProfile)
	}{
		{"empty key", func(p *model.SubscriptionProfile) { p.Key = "" }},
		{"bad mode", func(p *model.SubscriptionProfile) { p.RulesetMode = "nope" }},
		{"group no name", func(p *model.SubscriptionProfile) { p.ProxyGroups = model.ProxyGroups{{Type: model.GroupTypeSelect}} }},
		{"group bad type", func(p *model.SubscriptionProfile) { p.ProxyGroups = model.ProxyGroups{{Name: "G", Type: "weird"}} }},
		{"ruleset rule no key", func(p *model.SubscriptionProfile) {
			p.Rules = model.Rules{{Kind: model.RuleKindRuleset, Group: "G"}}
		}},
		{"inline rule no matcher", func(p *model.SubscriptionProfile) {
			p.Rules = model.Rules{{Kind: model.RuleKindInline, Group: "G"}}
		}},
		{"rule no group", func(p *model.SubscriptionProfile) {
			p.Rules = model.Rules{{Kind: model.RuleKindInline, Matcher: "MATCH"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.ProxyGroups = append(model.ProxyGroups{}, base.ProxyGroups...)
			p.Rules = append(model.Rules{}, base.Rules...)
			tc.mut(&p)
			if err := validateProfile(&p); err == nil {
				t.Errorf("%s: expected a validation error", tc.name)
			}
		})
	}
}

func TestValidateRuleset(t *testing.T) {
	remote := model.SubscriptionRuleset{Key: "proxy", SourceType: model.RulesetSourceRemote, URL: "https://x/proxy.txt", Behavior: model.RulesetBehaviorDomain}
	if err := validateRuleset(&remote); err != nil {
		t.Fatalf("valid remote ruleset rejected: %v", err)
	}
	inline := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceInline, Content: "DOMAIN-SUFFIX,x", Behavior: model.RulesetBehaviorClassical}
	if err := validateRuleset(&inline); err != nil {
		t.Fatalf("valid inline ruleset rejected: %v", err)
	}

	bad := []model.SubscriptionRuleset{
		{Key: "", SourceType: model.RulesetSourceRemote, URL: "u", Behavior: model.RulesetBehaviorDomain},
		{Key: "k", SourceType: model.RulesetSourceRemote, URL: "", Behavior: model.RulesetBehaviorDomain},
		{Key: "k", SourceType: model.RulesetSourceInline, Content: "", Behavior: model.RulesetBehaviorDomain},
		{Key: "k", SourceType: "weird", URL: "u", Behavior: model.RulesetBehaviorDomain},
		{Key: "k", SourceType: model.RulesetSourceRemote, URL: "u", Behavior: "weird"},
	}
	for i := range bad {
		if err := validateRuleset(&bad[i]); err == nil {
			t.Errorf("bad ruleset #%d should fail validation", i)
		}
	}
}

func TestNormalizeDefaults(t *testing.T) {
	p := model.SubscriptionProfile{Key: "  k  "}
	normalizeProfile(&p)
	if p.Key != "k" {
		t.Errorf("key not trimmed: %q", p.Key)
	}
	if p.RulesetMode != model.RulesetModePassthrough {
		t.Errorf("ruleset_mode default = %q, want passthrough", p.RulesetMode)
	}

	rs := model.SubscriptionRuleset{Key: " r "}
	normalizeRuleset(&rs)
	if rs.Key != "r" || rs.SourceType != model.RulesetSourceRemote || rs.Behavior != model.RulesetBehaviorClassical || rs.TTLSeconds != 86400 {
		t.Errorf("ruleset defaults wrong: %+v", rs)
	}
}
