package sub

import (
	"fmt"
	"strings"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/sub/policy"
)

// clash_policy.go serializes a resolved policy (groups / rule-providers /
// rules) into the three Clash YAML blocks the template substitutes. Node
// proxy entries themselves are still rendered by clash.go (clashNode);
// this file only handles the policy sections.

// clashProxyGroupsYAML renders the `proxy-groups:` block, or "" when the
// policy has no groups.
func clashProxyGroupsYAML(r policy.Resolved) string {
	if len(r.Groups) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("proxy-groups:")
	for _, g := range r.Groups {
		b.WriteString("\n  - name: ")
		b.WriteString(clashScalar(g.Name))
		b.WriteString("\n    type: ")
		b.WriteString(g.Type)
		if g.Type == model.GroupTypeURLTest || g.Type == model.GroupTypeFallback {
			url := g.TestURL
			if url == "" {
				url = "http://www.gstatic.com/generate_204"
			}
			interval := g.Interval
			if interval <= 0 {
				interval = 300
			}
			b.WriteString("\n    url: ")
			b.WriteString(url)
			fmt.Fprintf(&b, "\n    interval: %d", interval)
			b.WriteString("\n    tolerance: 50")
		}
		b.WriteString("\n    proxies: [")
		b.WriteString(clashFlowList(g.Members))
		b.WriteString("]")
	}
	return b.String()
}

// clashRuleProvidersYAML renders the `rule-providers:` block from the
// policy's referenced remote rulesets, or "" when none are remote.
// Inline rulesets are not providers (their matchers are expanded into
// rules elsewhere).
func clashRuleProvidersYAML(r policy.Resolved) string {
	var remote []policy.ResolvedRuleset
	for _, rs := range r.Rulesets {
		if rs.SourceType == model.RulesetSourceRemote && rs.URL != "" {
			remote = append(remote, rs)
		}
	}
	if len(remote) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("rule-providers:")
	for _, rs := range remote {
		interval := rs.TTLSeconds
		if interval <= 0 {
			interval = 86400
		}
		behavior := rs.Behavior
		if behavior == "" {
			behavior = model.RulesetBehaviorClassical
		}
		b.WriteString("\n  ")
		b.WriteString(rs.Key)
		b.WriteString(":\n    type: http\n    behavior: ")
		b.WriteString(behavior)
		if rs.Format != "" {
			b.WriteString("\n    format: ")
			b.WriteString(rs.Format)
		}
		b.WriteString("\n    url: ")
		b.WriteString(rs.URL)
		b.WriteString("\n    path: ./ruleset/")
		b.WriteString(rs.Key)
		b.WriteString(".yaml")
		fmt.Fprintf(&b, "\n    interval: %d", interval)
	}
	return b.String()
}

// clashRulesYAML renders the ordered `rules:` block. Always emits at
// least a terminal MATCH so the config never silently drops traffic.
func clashRulesYAML(r policy.Resolved) string {
	var b strings.Builder
	b.WriteString("rules:")
	if len(r.Rules) == 0 {
		b.WriteString("\n  - MATCH,DIRECT")
		return b.String()
	}
	for _, rule := range r.Rules {
		b.WriteString("\n  - ")
		if strings.EqualFold(rule.Type, "MATCH") {
			b.WriteString("MATCH,")
			b.WriteString(rule.Group)
			continue
		}
		b.WriteString(rule.Type)
		b.WriteString(",")
		b.WriteString(rule.Payload)
		b.WriteString(",")
		b.WriteString(rule.Group)
		if rule.NoResolve {
			b.WriteString(",no-resolve")
		}
	}
	return b.String()
}

// clashFlowList renders node/group names as the inner contents of a YAML
// flow sequence (`[a, b, c]`). Empty membership falls back to DIRECT so
// a group is never proxy-less (invalid in Clash).
func clashFlowList(names []string) string {
	if len(names) == 0 {
		return "DIRECT"
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = clashScalar(n)
	}
	return strings.Join(parts, ", ")
}

// clashScalar quotes a YAML scalar when it contains indicator characters
// that would break a flow sequence or mapping. Plain names (incl. CJK)
// pass through unquoted. The renderer's post-parse validation is the
// backstop if this is ever too lax.
func clashScalar(s string) string {
	if s == "" || strings.ContainsAny(s, ",[]{}\"':#&*!|>%@`") ||
		strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
