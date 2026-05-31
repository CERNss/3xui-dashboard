package sub

import (
	"fmt"
	"strings"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/sub/policy"
)

// surge_policy.go serializes a resolved policy into Surge's
// [Proxy Group] and [Rule] section bodies.

// surgeProxyGroupsBlock renders the [Proxy Group] lines.
func surgeProxyGroupsBlock(r policy.Resolved) string {
	var lines []string
	for _, g := range r.Groups {
		members := g.Members
		if len(members) == 0 {
			members = []string{"DIRECT"} // a Surge group is never proxy-less
		}
		parts := append([]string{surgeGroupType(g.Type)}, members...)
		if g.Type == model.GroupTypeURLTest || g.Type == model.GroupTypeFallback {
			url := g.TestURL
			if url == "" {
				url = "http://www.gstatic.com/generate_204"
			}
			interval := g.Interval
			if interval <= 0 {
				interval = 300
			}
			parts = append(parts, "url = "+url, fmt.Sprintf("interval = %d", interval))
		}
		lines = append(lines, g.Name+" = "+strings.Join(parts, ", "))
	}
	return strings.Join(lines, "\n")
}

// surgeGroupType maps a model group type to a Surge one. Surge has no
// relay group, so unknown types degrade to select.
func surgeGroupType(t string) string {
	switch t {
	case model.GroupTypeSelect, model.GroupTypeURLTest, model.GroupTypeFallback, model.GroupTypeLoadBalance:
		return t
	default:
		return model.GroupTypeSelect
	}
}

// surgeRulesBlock renders the [Rule] lines. Surge has no rule-provider
// section, so RULE-SET carries the list URL inline (upstream in
// passthrough mode, our /sub/ruleset endpoint in self_hosted). MATCH
// becomes Surge's terminal FINAL.
func surgeRulesBlock(r policy.Resolved, mode, serveBase string) string {
	byKey := make(map[string]policy.ResolvedRuleset, len(r.Rulesets))
	for _, rs := range r.Rulesets {
		byKey[rs.Key] = rs
	}

	var lines []string
	for _, rule := range r.Rules {
		switch {
		case strings.EqualFold(rule.Type, "MATCH"):
			lines = append(lines, "FINAL,"+rule.Group)
		case rule.Type == "RULE-SET":
			rs, ok := byKey[rule.Payload]
			if !ok {
				continue
			}
			url := rs.URL
			if mode == model.RulesetModeSelfHosted {
				url = serveBase + "/sub/ruleset/" + rs.Key
			}
			if url == "" {
				continue // inline ruleset in passthrough — nothing to point at
			}
			lines = append(lines, "RULE-SET,"+url+","+rule.Group)
		default:
			line := rule.Type + "," + rule.Payload + "," + rule.Group
			if rule.NoResolve {
				line += ",no-resolve"
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "FINAL,DIRECT")
	}
	return strings.Join(lines, "\n")
}

// surgeName sanitizes a node remark for use as a Surge proxy name. Surge
// has no quoting for names, and "," / "=" are structural in its line
// grammar, so they're replaced. Applied consistently to both the [Proxy]
// line and group membership so they always match.
func surgeName(remark string) string {
	return strings.TrimSpace(strings.NewReplacer(",", " ", "=", "-").Replace(remark))
}
