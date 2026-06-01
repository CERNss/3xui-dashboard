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

// surgeRulesBlock renders the [Rule] lines from the policy's INLINE
// rules only (DOMAIN-SUFFIX/GEOIP/... + FINAL). MATCH becomes Surge's
// terminal FINAL.
//
// TODO(sub-rulesets): ruleset (RULE-SET) rules are intentionally skipped
// for non-Clash targets. Rule *lists* are client-format-specific, but
// our default rulesets are Clash-format (Loyalsoldier), so emitting a
// RULE-SET pointing at them would feed Surge a list it can't parse. The
// full fix (deferred): server-side fetch + convert the list to the
// target's format and serve it from /sub/ruleset/<key>?target=surge, or
// let a ruleset carry per-target URLs. Until then Surge gets
// proxies + groups + inline rules (correct, just coarser than Clash).
func surgeRulesBlock(r policy.Resolved) string {
	var lines []string
	for _, rule := range r.Rules {
		switch {
		case strings.EqualFold(rule.Type, "MATCH"):
			lines = append(lines, "FINAL,"+rule.Group)
		case rule.Type == "RULE-SET":
			continue // see TODO(sub-rulesets) above
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
