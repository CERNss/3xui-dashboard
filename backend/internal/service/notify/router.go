package notify

import (
	"fmt"
	"strings"
)

// Router maps event types to ordered channel-name lists.
//
// Format (env var): `event.type:chan1,chan2;event.type2:chan3`.
// Whitespace around the parts is tolerated; empty tokens are
// rejected so an operator who pastes a trailing semicolon sees the
// error at boot.
type Router struct {
	rules map[string][]string
}

// defaultRoutes encodes the legacy behavior: client lifecycle
// events go to email, nothing else. Used when the env var is
// empty so a fresh deployment behaves like v1.
func defaultRoutes() map[string][]string {
	return map[string][]string{
		"client.expired":       {"email"},
		"client.expiring_soon": {"email"},
		"client.over_limit":    {"email"},
	}
}

// ParseRoutes parses the env-var format. Empty input returns the
// default rules.
func ParseRoutes(raw string) (*Router, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &Router{rules: defaultRoutes()}, nil
	}
	rules := map[string][]string{}
	for _, rule := range strings.Split(raw, ";") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		colon := strings.IndexByte(rule, ':')
		if colon < 0 {
			return nil, fmt.Errorf("notify route %q missing ':' (expected `event.type:channel`)", rule)
		}
		event := strings.TrimSpace(rule[:colon])
		channelsRaw := strings.TrimSpace(rule[colon+1:])
		if event == "" {
			return nil, fmt.Errorf("notify route %q has empty event type", rule)
		}
		if channelsRaw == "" {
			return nil, fmt.Errorf("notify route %q has no channels", rule)
		}
		var channels []string
		for _, c := range strings.Split(channelsRaw, ",") {
			c = strings.TrimSpace(c)
			if c == "" {
				return nil, fmt.Errorf("notify route %q has an empty channel token", rule)
			}
			channels = append(channels, c)
		}
		rules[event] = channels
	}
	return &Router{rules: rules}, nil
}

// Channels returns the channel names routed for an event type.
// Returns nil (not an empty slice) if no rule matches — callers
// can range over the result safely either way.
func (r *Router) Channels(eventType string) []string {
	if r == nil {
		return nil
	}
	return r.rules[eventType]
}

// OpsEventTypes returns the bus event types the service treats as
// "ops alerts" — events without an inherent per-user recipient.
// Exported so the app wiring can boot-check that the email channel
// (if routed for any of these) has a configured fallback recipient.
//
// Kept aligned with the set of events Service.Start subscribes to
// via opsOrderEvent / dispatchOpsEvent; the lifecycle events
// (client.*) are NOT in this list because they always have a
// per-user recipient.
func OpsEventTypes() []string {
	return []string{
		"node.offline",
		"node.recovered",
		"order.payment_confirmed",
		"order.payment_failed",
		"order.payment_expired",
		"order.failed",
	}
}

// ConfiguredChannels returns the set of channel names referenced
// anywhere in the rules. Used at boot to warn about channels that
// are routed-to but unconfigured.
func (r *Router) ConfiguredChannels() []string {
	if r == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, chans := range r.rules {
		for _, c := range chans {
			if _, ok := seen[c]; !ok {
				seen[c] = struct{}{}
				out = append(out, c)
			}
		}
	}
	return out
}
