package policy

import "github.com/cern/3xui-dashboard/internal/model"

// Built-in group names + the url-test probe. Kept as constants so the
// default profile and any code that references the groups stay in sync.
const (
	GroupSelect  = "节点选择" // manual selector (default MATCH target)
	GroupAuto    = "自动选择" // url-test
	defaultTestURL = "http://www.gstatic.com/generate_204"
)

// loyalsoldierBase is the raw-rule source the built-in rulesets pull
// from — the same Loyalsoldier/clash-rules release lists the previous
// hardcoded template referenced.
const loyalsoldierBase = "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/"

// DefaultRulesets is the built-in ACL4SSR-style ruleset list. It mirrors
// exactly the rule-providers the previous embedded Clash template
// shipped (same keys, behaviors, URLs), so the default render is
// unchanged. Format is left empty to reproduce the old template, which
// omitted an explicit `format:` and let the client default it.
func DefaultRulesets() []model.SubscriptionRuleset {
	type spec struct {
		key      string
		behavior string
	}
	specs := []spec{
		{"reject", model.RulesetBehaviorDomain},
		{"icloud", model.RulesetBehaviorDomain},
		{"apple", model.RulesetBehaviorDomain},
		{"google", model.RulesetBehaviorDomain},
		{"proxy", model.RulesetBehaviorDomain},
		{"direct", model.RulesetBehaviorDomain},
		{"private", model.RulesetBehaviorDomain},
		{"gfw", model.RulesetBehaviorDomain},
		{"tld-not-cn", model.RulesetBehaviorDomain},
		{"telegramcidr", model.RulesetBehaviorIPCIDR},
		{"cncidr", model.RulesetBehaviorIPCIDR},
		{"lancidr", model.RulesetBehaviorIPCIDR},
		{"applications", model.RulesetBehaviorClassical},
	}
	out := make([]model.SubscriptionRuleset, 0, len(specs))
	for _, s := range specs {
		out = append(out, model.SubscriptionRuleset{
			Key:        s.key,
			Name:       s.key,
			SourceType: model.RulesetSourceRemote,
			URL:        loyalsoldierBase + s.key + ".txt",
			Behavior:   s.behavior,
			TTLSeconds: 86400,
			Enabled:    true,
		})
	}
	return out
}

// DefaultProfile is the built-in routing policy: a manual selector that
// folds in an auto url-test group, and the ACL4SSR-style rule order the
// previous embedded template hardcoded. Used when a deployment has no
// admin-configured profiles, so behaviour is identical to before.
func DefaultProfile() model.SubscriptionProfile {
	return model.SubscriptionProfile{
		Key:       "default",
		Name:      "Default",
		IsDefault: true,
		Enabled:   true,
		ProxyGroups: model.ProxyGroups{
			{Name: GroupSelect, Type: model.GroupTypeSelect, Include: []string{GroupAuto, "DIRECT"}},
			{Name: GroupAuto, Type: model.GroupTypeURLTest, TestURL: defaultTestURL, Interval: 300},
		},
		Rules: model.Rules{
			{Kind: model.RuleKindInline, Matcher: "DOMAIN-SUFFIX", Value: "local", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "private", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "reject", Group: "REJECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "icloud", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "apple", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "google", Group: GroupSelect},
			{Kind: model.RuleKindRuleset, RulesetKey: "proxy", Group: GroupSelect},
			{Kind: model.RuleKindRuleset, RulesetKey: "direct", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "tld-not-cn", Group: GroupSelect},
			{Kind: model.RuleKindRuleset, RulesetKey: "gfw", Group: GroupSelect},
			{Kind: model.RuleKindRuleset, RulesetKey: "telegramcidr", Group: GroupSelect},
			{Kind: model.RuleKindRuleset, RulesetKey: "lancidr", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "cncidr", Group: "DIRECT"},
			{Kind: model.RuleKindRuleset, RulesetKey: "applications", Group: "DIRECT"},
			{Kind: model.RuleKindInline, Matcher: "GEOIP", Value: "CN", Group: "DIRECT"},
			{Kind: model.RuleKindInline, Matcher: "MATCH", Group: GroupSelect},
		},
	}
}
