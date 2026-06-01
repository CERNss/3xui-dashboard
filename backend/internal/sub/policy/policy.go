// Package policy resolves a target-agnostic subscription Profile (proxy
// groups + ordered rules + referenced rulesets) against a concrete list
// of node names into a flat intermediate representation (Resolved) that
// each target renderer (Clash, Surge, Quantumult X, Loon, sing-box) can
// serialize without re-deriving group membership or rule ordering.
//
// It is deliberately target-agnostic: no Clash/Surge syntax lives here.
// DefaultProfile/DefaultRulesets reproduce the converter's built-in
// ACL4SSR-style policy so a deployment with zero admin configuration
// renders exactly as it did before profiles existed.
package policy

import (
	"regexp"
	"strings"

	"github.com/cern/3xui-dashboard/internal/model"
)

// Resolved is the flattened, ready-to-serialize policy.
type Resolved struct {
	Groups   []ResolvedGroup
	Rules    []ResolvedRule
	Rulesets []ResolvedRuleset // referenced rulesets, first-seen order, deduped
}

// ResolvedGroup is a proxy group with its membership already computed.
// Members lists Include entries (other groups / DIRECT / REJECT) first,
// then the node names that passed the group's filter, in node order.
type ResolvedGroup struct {
	Name     string
	Type     string
	Members  []string
	TestURL  string
	Interval int
}

// ResolvedRule is one ordered routing entry. Type is the upper-cased
// matcher ("RULE-SET", "DOMAIN-SUFFIX", "GEOIP", "MATCH", ...); Payload
// is the ruleset key, the matcher value, or "" for MATCH.
type ResolvedRule struct {
	Type      string
	Payload   string
	Group     string
	NoResolve bool
}

// ResolvedRuleset is a referenced ruleset's render-relevant fields.
type ResolvedRuleset struct {
	Key        string
	SourceType string // model.RulesetSource*
	URL        string
	Content    string
	Behavior   string
	Format     string
	TTLSeconds int
}

// Resolve flattens profile against nodeNames. rulesets maps ruleset key
// to its definition (from DefaultRulesets or the repo); a rule that
// references an unknown key still emits its RULE-SET line but contributes
// no provider entry (the renderer can warn / drop).
func Resolve(profile model.SubscriptionProfile, nodeNames []string, rulesets map[string]model.SubscriptionRuleset) Resolved {
	var res Resolved

	for _, g := range profile.ProxyGroups {
		members := make([]string, 0, len(g.Include)+len(nodeNames))
		members = append(members, g.Include...)
		include := compileFilter(g.Filter)
		exclude := compileFilter(g.Exclude)
		for _, name := range nodeNames {
			if include != nil && !include.MatchString(name) {
				continue
			}
			if exclude != nil && exclude.MatchString(name) {
				continue
			}
			members = append(members, name)
		}
		res.Groups = append(res.Groups, ResolvedGroup{
			Name:     g.Name,
			Type:     g.Type,
			Members:  members,
			TestURL:  g.TestURL,
			Interval: g.Interval,
		})
	}

	seen := make(map[string]bool)
	hasMatch := false
	for _, r := range profile.Rules {
		switch r.Kind {
		case model.RuleKindRuleset:
			res.Rules = append(res.Rules, ResolvedRule{Type: "RULE-SET", Payload: r.RulesetKey, Group: r.Group})
			if rs, ok := rulesets[r.RulesetKey]; ok && !seen[r.RulesetKey] {
				seen[r.RulesetKey] = true
				res.Rulesets = append(res.Rulesets, resolvedRuleset(rs))
			}
		case model.RuleKindInline:
			t := strings.ToUpper(strings.TrimSpace(r.Matcher))
			if t == "" {
				continue
			}
			if t == "MATCH" {
				hasMatch = true
			}
			res.Rules = append(res.Rules, ResolvedRule{Type: t, Payload: r.Value, Group: r.Group, NoResolve: r.NoResolve})
		}
	}

	// Safety net: a config without a terminal MATCH would drop traffic
	// that matched nothing. Fall through to the first group.
	if !hasMatch && len(res.Groups) > 0 {
		res.Rules = append(res.Rules, ResolvedRule{Type: "MATCH", Group: res.Groups[0].Name})
	}

	return res
}

// compileFilter returns a compiled regex, or nil for an empty pattern
// (match everything) or an invalid one (treated as no filter rather than
// silently dropping every node — admin-facing validation lives in the
// handler).
func compileFilter(pattern string) *regexp.Regexp {
	if strings.TrimSpace(pattern) == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

func resolvedRuleset(rs model.SubscriptionRuleset) ResolvedRuleset {
	return ResolvedRuleset{
		Key:        rs.Key,
		SourceType: rs.SourceType,
		URL:        rs.URL,
		Content:    rs.Content,
		Behavior:   rs.Behavior,
		Format:     rs.Format,
		TTLSeconds: rs.TTLSeconds,
	}
}

// RulesetMap indexes rulesets by key for Resolve.
func RulesetMap(rulesets []model.SubscriptionRuleset) map[string]model.SubscriptionRuleset {
	m := make(map[string]model.SubscriptionRuleset, len(rulesets))
	for _, rs := range rulesets {
		m[rs.Key] = rs
	}
	return m
}
