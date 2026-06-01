// Package template renders Clash YAML and sing-box JSON subscription
// payloads by substituting placeholder markers in a template document
// with the user's resolved proxy node list and (for Clash) the routing
// policy blocks produced from a subscription profile.
//
// Substitution is intentionally text-level (not parse-and-recompose):
// templates are operator-controlled — they're free to include comments
// and fenced markers we shouldn't reformat. Defaults are pinned so the
// indentation is known. Operator-supplied overrides are validated by
// attempting a YAML/JSON parse on the rendered output; failure surfaces
// as a wrapped error so callers can fall back to the default.
package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Placeholders recognized in templates.
const (
	placeholderProxies       = "${proxies}"        // serialized node list
	placeholderProxyNames    = "${proxy_names}"    // comma-joined node names
	placeholderProxyGroups   = "${proxy_groups}"   // proxy-groups: block (Clash)
	placeholderRuleProviders = "${rule_providers}" // rule-providers: block (Clash)
	placeholderRules         = "${rules}"          // rules: block (Clash)
)

// ClashPolicy holds the pre-rendered Clash policy blocks substituted
// into the template. The caller builds these from a resolved
// subscription profile (see internal/sub/clash_policy.go); empty fields
// substitute to nothing.
type ClashPolicy struct {
	Groups        string // proxy-groups: block
	RuleProviders string // rule-providers: block (may be empty)
	Rules         string // rules: block
}

// RenderClash substitutes the placeholders in base (or the default
// skeleton when base is empty) with the supplied nodes + policy blocks
// and returns the rendered YAML. A post-render yaml.Unmarshal is
// performed; parse failures are wrapped so callers can fall back to the
// default.
//
// Empty nodes short-circuit to a minimal valid Clash config so users
// with no provisioned clients still get parseable YAML.
func RenderClash(nodes []map[string]any, pol ClashPolicy, base string) ([]byte, error) {
	if len(nodes) == 0 && base == "" {
		return []byte(emptyClashYAML), nil
	}
	tmpl := base
	usingDefault := tmpl == ""
	if usingDefault {
		tmpl = defaultClashSkeleton
	}

	nodesYAML, err := marshalProxiesYAML(nodes)
	if err != nil {
		return nil, fmt.Errorf("template: marshal clash nodes: %w", err)
	}

	out := strings.ReplaceAll(tmpl, placeholderProxies, nodesYAML)
	out = strings.ReplaceAll(out, placeholderProxyNames, joinNames(proxyNames(nodes), ", "))
	out = strings.ReplaceAll(out, placeholderProxyGroups, pol.Groups)
	out = strings.ReplaceAll(out, placeholderRuleProviders, pol.RuleProviders)
	out = strings.ReplaceAll(out, placeholderRules, pol.Rules)

	var probe any
	if err := yaml.Unmarshal([]byte(out), &probe); err != nil {
		if usingDefault {
			return nil, fmt.Errorf("template: default clash yaml failed to parse (bug): %w", err)
		}
		return nil, fmt.Errorf("template: operator clash yaml failed to parse: %w", err)
	}
	return []byte(out), nil
}

// RenderSingBox substitutes placeholders in base (or default when empty)
// with the sing-box outbound list. Same parse-check semantics as
// RenderClash but with json.Unmarshal.
func RenderSingBox(outbounds []map[string]any, base string) ([]byte, error) {
	if len(outbounds) == 0 && base == "" {
		return []byte(emptySingBoxJSON), nil
	}
	tmpl := base
	usingDefault := tmpl == ""
	if usingDefault {
		tmpl = defaultSingBoxJSON
	}

	parts := make([]string, len(outbounds))
	for i, o := range outbounds {
		b, err := json.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("template: marshal singbox outbound %d: %w", i, err)
		}
		parts[i] = string(b)
	}
	nodesJSON := strings.Join(parts, ",\n    ")
	namesJSON := joinNamesAsJSONList(singboxNames(outbounds))

	out := strings.ReplaceAll(tmpl, placeholderProxies, nodesJSON)
	out = strings.ReplaceAll(out, placeholderProxyNames, namesJSON)

	var probe any
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		if usingDefault {
			return nil, fmt.Errorf("template: default singbox json failed to parse (bug): %w", err)
		}
		return nil, fmt.Errorf("template: operator singbox json failed to parse: %w", err)
	}
	return []byte(out), nil
}

// marshalProxiesYAML serializes a list of Clash proxy objects as a
// YAML sequence, indented to fit under `proxies:`. Returns the
// sequence without a leading `proxies:` key — that lives in the
// template.
func marshalProxiesYAML(nodes []map[string]any) (string, error) {
	if len(nodes) == 0 {
		// Empty sequence still needs the 2-space indent so it slots
		// under `proxies:` as a value.
		return "  []", nil
	}
	b, err := yaml.Marshal(nodes)
	if err != nil {
		return "", err
	}
	raw := strings.TrimRight(string(b), "\n")
	lines := strings.Split(raw, "\n")
	for i, ln := range lines {
		lines[i] = "  " + ln
	}
	return strings.Join(lines, "\n"), nil
}

// proxyNames extracts the .name field from each Clash proxy entry.
// Missing / non-string names fall back to "node-<i>".
func proxyNames(nodes []map[string]any) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		if s, ok := n["name"].(string); ok && s != "" {
			out[i] = s
			continue
		}
		out[i] = fmt.Sprintf("node-%d", i+1)
	}
	return out
}

// singboxNames extracts .tag from each sing-box outbound entry.
func singboxNames(outbounds []map[string]any) []string {
	out := make([]string, len(outbounds))
	for i, o := range outbounds {
		if s, ok := o["tag"].(string); ok && s != "" {
			out[i] = s
			continue
		}
		out[i] = fmt.Sprintf("node-%d", i+1)
	}
	return out
}

// joinNames concatenates names with the given separator, quoting any
// name that contains a comma or special YAML character.
func joinNames(names []string, sep string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		if strings.ContainsAny(n, ",[]{}\"") {
			quoted[i] = fmt.Sprintf("%q", n)
		} else {
			quoted[i] = n
		}
	}
	return strings.Join(quoted, sep)
}

// joinNamesAsJSONList returns a JSON array fragment without the
// surrounding brackets (the template provides the brackets).
func joinNamesAsJSONList(names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		b, _ := json.Marshal(n)
		parts[i] = string(b)
	}
	return strings.Join(parts, ", ")
}
