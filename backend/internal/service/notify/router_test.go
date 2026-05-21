package notify

import (
	"strings"
	"testing"
)

func TestParseRoutes_Empty(t *testing.T) {
	r, err := ParseRoutes("")
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	// After the messages/notifications split there are no default
	// routes — ops fanout is opt-in via NOTIFY_ROUTES, user-facing
	// client lifecycle goes through service/messages.
	if got := r.Channels("client.expired"); got != nil {
		t.Errorf("expected no default route for client.expired, got %v", got)
	}
	if got := r.Channels("node.offline"); got != nil {
		t.Errorf("expected no default route for node.offline, got %v", got)
	}
}

func TestParseRoutes_Simple(t *testing.T) {
	r, err := ParseRoutes("node.offline:telegram,discord;client.expired:email")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := r.Channels("node.offline"); len(got) != 2 || got[0] != "telegram" || got[1] != "discord" {
		t.Errorf("node.offline = %v, want [telegram discord]", got)
	}
	if got := r.Channels("client.expired"); len(got) != 1 || got[0] != "email" {
		t.Errorf("client.expired = %v", got)
	}
}

func TestParseRoutes_WhitespaceTolerated(t *testing.T) {
	r, err := ParseRoutes("  node.offline : telegram , discord ; client.expired : email ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := r.Channels("node.offline"); len(got) != 2 {
		t.Errorf("whitespace eats channels: %v", got)
	}
}

func TestParseRoutes_MissingColon(t *testing.T) {
	_, err := ParseRoutes("node.offline-telegram")
	if err == nil {
		t.Fatal("expected error on missing :")
	}
	if !strings.Contains(err.Error(), "missing ':'") {
		t.Errorf("error doesn't identify the issue: %v", err)
	}
}

func TestParseRoutes_EmptyEventName(t *testing.T) {
	_, err := ParseRoutes(":telegram")
	if err == nil || !strings.Contains(err.Error(), "empty event type") {
		t.Errorf("want empty-event error, got %v", err)
	}
}

func TestParseRoutes_NoChannels(t *testing.T) {
	_, err := ParseRoutes("node.offline:")
	if err == nil || !strings.Contains(err.Error(), "no channels") {
		t.Errorf("want no-channels error, got %v", err)
	}
}

func TestParseRoutes_EmptyChannelToken(t *testing.T) {
	_, err := ParseRoutes("node.offline:telegram,,discord")
	if err == nil || !strings.Contains(err.Error(), "empty channel") {
		t.Errorf("want empty-channel-token error, got %v", err)
	}
}

func TestParseRoutes_TrailingSemicolonOK(t *testing.T) {
	// Trailing semicolons are common in env var pastes; we tolerate
	// empty rules and skip them rather than erroring.
	r, err := ParseRoutes("node.offline:telegram;")
	if err != nil {
		t.Fatalf("trailing ; should be ok: %v", err)
	}
	if got := r.Channels("node.offline"); len(got) != 1 {
		t.Errorf("rule lost: %v", got)
	}
}

func TestConfiguredChannels(t *testing.T) {
	r, _ := ParseRoutes("a:x,y;b:y,z")
	got := r.ConfiguredChannels()
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	// Order isn't guaranteed (map iteration); just check membership.
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	for _, want := range []string{"x", "y", "z"} {
		if !seen[want] {
			t.Errorf("missing %q from ConfiguredChannels", want)
		}
	}
}

func TestChannels_NilRouter(t *testing.T) {
	var r *Router
	if got := r.Channels("anything"); got != nil {
		t.Errorf("nil router should return nil, got %v", got)
	}
}
