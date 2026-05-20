// Package template renders Clash YAML and sing-box JSON subscription
// payloads by substituting placeholder markers in a template document
// with the user's resolved proxy node list.
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

// Placeholders recognized in both Clash and sing-box templates.
const (
	placeholderProxies     = "${proxies}"      // serialized node list
	placeholderProxyNames  = "${proxy_names}"  // comma-joined node names (for group .proxies arrays)
)

// Options controls how the default templates render. Operator-supplied
// overrides (non-empty ClashTemplate / SingBoxTemplate) bypass these
// knobs entirely.
type Options struct {
	// ProxyGroupStrategy: "auto-only" / "select-only" / "auto+select".
	// Empty defaults to "auto+select".
	ProxyGroupStrategy string

	// RuleProvidersEnabled: when false, the default Clash template
	// strips rule-providers + rules sections, leaving a minimal
	// proxies-and-groups config.
	RuleProvidersEnabled bool

	// ClashTemplate overrides defaultClashYAML when non-empty.
	ClashTemplate string

	// SingBoxTemplate overrides defaultSingBoxJSON when non-empty.
	SingBoxTemplate string
}

// RenderClash substitutes the placeholders in tmpl (or the default
// when tmpl is empty) with the supplied nodes and returns the rendered
// YAML. A post-render yaml.Unmarshal is performed; parse failures are
// wrapped and returned so callers can fall back to the default.
//
// `nodes` is a slice of `map[string]any` — one Clash proxy entry per
// node. The map is YAML-marshaled and indented to match the template's
// `proxies:` block (2-space prefix on each line).
//
// Empty `nodes` short-circuits to a minimal valid Clash config (no
// rules, single DIRECT group) so users with no provisioned clients
// still get parseable YAML instead of a broken substitution.
func RenderClash(nodes []map[string]any, opts Options) ([]byte, error) {
	if len(nodes) == 0 && opts.ClashTemplate == "" {
		return []byte(emptyClashYAML), nil
	}
	tmpl := opts.ClashTemplate
	usingDefault := tmpl == ""
	if usingDefault {
		tmpl = applyClashKnobs(defaultClashYAML, opts)
	}

	// Serialize nodes as a YAML list. We rely on yaml.v3's default
	// indent (2 spaces) and then prefix each line with 2 more spaces
	// so the result slots cleanly under the template's `proxies:`
	// declaration.
	nodesYAML, err := marshalProxiesYAML(nodes)
	if err != nil {
		return nil, fmt.Errorf("template: marshal clash nodes: %w", err)
	}
	names := proxyNames(nodes)

	out := strings.ReplaceAll(tmpl, placeholderProxies, nodesYAML)
	out = strings.ReplaceAll(out, placeholderProxyNames, joinNames(names, ", "))

	// Post-render validation — operator templates can be malformed.
	// For defaults this is a sanity check.
	var probe any
	if err := yaml.Unmarshal([]byte(out), &probe); err != nil {
		if usingDefault {
			// Should never happen — the default is unit-tested. Surface
			// as internal error so we don't ship broken YAML.
			return nil, fmt.Errorf("template: default clash yaml failed to parse (bug): %w", err)
		}
		return nil, fmt.Errorf("template: operator clash yaml failed to parse: %w", err)
	}
	return []byte(out), nil
}

// RenderSingBox substitutes placeholders in tmpl (or default when
// empty) with the sing-box outbound list. Same parse-check semantics
// as RenderClash but with json.Unmarshal.
//
// Empty `outbounds` short-circuits to a minimal valid sing-box config
// (no proxy outbounds, just direct/block/dns + a final-direct route)
// — the placeholder substitution otherwise leaves dangling commas
// inside the JSON `outbounds[]` array.
func RenderSingBox(outbounds []map[string]any, opts Options) ([]byte, error) {
	if len(outbounds) == 0 && opts.SingBoxTemplate == "" {
		return []byte(emptySingBoxJSON), nil
	}
	tmpl := opts.SingBoxTemplate
	usingDefault := tmpl == ""
	if usingDefault {
		tmpl = defaultSingBoxJSON
	}

	// sing-box's outbounds[] is JSON — marshal each one compactly and
	// join with commas. We're substituting into a JSON array slot, so
	// we just emit the entries themselves.
	parts := make([]string, len(outbounds))
	for i, o := range outbounds {
		b, err := json.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("template: marshal singbox outbound %d: %w", i, err)
		}
		parts[i] = string(b)
	}
	nodesJSON := strings.Join(parts, ",\n    ")
	names := singboxNames(outbounds)
	namesJSON := joinNamesAsJSONList(names)

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

// applyClashKnobs swaps the proxy-groups block and (optionally) strips
// rule-providers + rules from the default template based on Options.
// Only invoked for the default template — operator overrides bypass.
func applyClashKnobs(tmpl string, opts Options) string {
	out := tmpl

	strategy := opts.ProxyGroupStrategy
	if strategy == "" {
		strategy = "auto+select"
	}
	out = strings.ReplaceAll(out, "${proxy_groups}", clashProxyGroupsBlock(strategy))

	if !opts.RuleProvidersEnabled {
		// Strip everything between the rule-section markers (inclusive).
		out = stripBetween(out, "# >>> rule-section >>>", "# <<< rule-section <<<")
	} else {
		// Just remove the marker lines so the section renders cleanly.
		out = strings.ReplaceAll(out, "# >>> rule-section >>>\n", "")
		out = strings.ReplaceAll(out, "# <<< rule-section <<<\n", "")
	}
	return out
}

// clashProxyGroupsBlock returns the proxy-groups YAML for the chosen
// strategy. Names array uses ${proxy_names} which is substituted later
// in the same render call.
func clashProxyGroupsBlock(strategy string) string {
	switch strategy {
	case "auto-only":
		return `proxy-groups:
  - name: 自动选择
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies: [${proxy_names}]`
	case "select-only":
		return `proxy-groups:
  - name: 节点选择
    type: select
    proxies: [DIRECT, ${proxy_names}]`
	case "auto+select":
		fallthrough
	default:
		return `proxy-groups:
  - name: 节点选择
    type: select
    proxies: [自动选择, DIRECT, ${proxy_names}]
  - name: 自动选择
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies: [${proxy_names}]`
	}
}

// marshalProxiesYAML serializes a list of Clash proxy objects as a
// YAML sequence, indented to fit under `proxies:`. Returns the
// sequence without a leading `proxies:` key — that lives in the
// template.
func marshalProxiesYAML(nodes []map[string]any) (string, error) {
	if len(nodes) == 0 {
		// Empty sequence still needs the 2-space indent so it slots
		// under `proxies:` as a value (otherwise YAML parsers treat
		// the `[]` as a new root document).
		return "  []", nil
	}
	// Marshal as a sequence at root, then drop the leading newline
	// yaml.v3 emits and indent every line by 2 spaces.
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
// name that contains a comma or special YAML character. For our
// templates the values land in a flow-style sequence (e.g.
// `proxies: [a, b, c]`) so quoting is conservative.
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

// stripBetween removes the block bounded by start..end markers
// inclusive, with surrounding whitespace.
func stripBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return s
	}
	j := strings.Index(s[i:], end)
	if j < 0 {
		return s
	}
	j += i + len(end)
	// Eat the trailing newline after the end marker if present.
	if j < len(s) && s[j] == '\n' {
		j++
	}
	return s[:i] + s[j:]
}
