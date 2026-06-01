package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestAdminSubscriptionProfileCRUD exercises the admin profile CRUD API
// against a real DB, including the boot-seeded default, validation,
// duplicate-key conflict, and the single-default / no-delete-default
// invariants.
func TestAdminSubscriptionProfileCRUD(t *testing.T) {
	h := setupHarness(t)
	tok := h.adminLogin(t)

	type profile struct {
		ID        int64  `json:"id"`
		Key       string `json:"key"`
		IsDefault bool   `json:"is_default"`
	}

	// Boot seeds a default profile.
	var list struct {
		Profiles []profile `json:"profiles"`
	}
	if got := h.do(t, req{method: http.MethodGet, path: "/api/admin/subscription/profiles", token: tok}, &list); got != http.StatusOK {
		t.Fatalf("list profiles: %d", got)
	}
	var seededDefault int64
	for _, p := range list.Profiles {
		if p.IsDefault {
			seededDefault = p.ID
		}
	}
	if seededDefault == 0 {
		t.Fatal("expected a boot-seeded default profile")
	}

	// Create a new (non-default) profile.
	body := map[string]any{
		"key":          "minimal",
		"name":         "Minimal",
		"proxy_groups": []map[string]any{{"name": "PROXY", "type": "select", "include": []string{"DIRECT"}}},
		"rules":        []map[string]any{{"kind": "inline", "matcher": "MATCH", "group": "PROXY"}},
	}
	var created profile
	if got := h.do(t, req{method: http.MethodPost, path: "/api/admin/subscription/profiles", token: tok, body: body}, &created); got != http.StatusCreated {
		t.Fatalf("create profile: %d", got)
	}
	if created.ID == 0 || created.Key != "minimal" || created.IsDefault {
		t.Fatalf("unexpected created profile: %+v", created)
	}

	// Duplicate key → 409; invalid mode → 400.
	if got := h.do(t, req{method: http.MethodPost, path: "/api/admin/subscription/profiles", token: tok, body: body}, nil); got != http.StatusConflict {
		t.Fatalf("duplicate key: got %d, want 409", got)
	}
	if got := h.do(t, req{method: http.MethodPost, path: "/api/admin/subscription/profiles", token: tok, body: map[string]any{"key": "x", "ruleset_mode": "bogus"}}, nil); got != http.StatusBadRequest {
		t.Fatalf("bad ruleset_mode: got %d, want 400", got)
	}

	// Promote to default, then verify deleting the default is refused.
	if got := h.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/admin/subscription/profiles/%d/default", created.ID), token: tok}, nil); got != http.StatusOK {
		t.Fatalf("set default: %d", got)
	}
	if got := h.do(t, req{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/subscription/profiles/%d", created.ID), token: tok}, nil); got != http.StatusBadRequest {
		t.Fatalf("delete-default should be refused: got %d, want 400", got)
	}

	// Restore the seeded default, then minimal can be deleted.
	if got := h.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/admin/subscription/profiles/%d/default", seededDefault), token: tok}, nil); got != http.StatusOK {
		t.Fatalf("restore default: %d", got)
	}
	if got := h.do(t, req{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/subscription/profiles/%d", created.ID), token: tok}, nil); got != http.StatusOK {
		t.Fatalf("delete minimal: %d", got)
	}
}

// TestAdminSubscriptionRulesetCRUD exercises the ruleset CRUD API.
func TestAdminSubscriptionRulesetCRUD(t *testing.T) {
	h := setupHarness(t)
	tok := h.adminLogin(t)

	var created struct {
		ID  int64  `json:"id"`
		Key string `json:"key"`
	}
	create := map[string]any{"key": "custom", "name": "Custom", "source_type": "remote", "url": "https://example.com/custom.txt", "behavior": "domain"}
	if got := h.do(t, req{method: http.MethodPost, path: "/api/admin/subscription/rulesets", token: tok, body: create}, &created); got != http.StatusCreated {
		t.Fatalf("create ruleset: %d", got)
	}
	if created.ID == 0 || created.Key != "custom" {
		t.Fatalf("unexpected created ruleset: %+v", created)
	}

	// Remote with no URL → 400.
	if got := h.do(t, req{method: http.MethodPost, path: "/api/admin/subscription/rulesets", token: tok, body: map[string]any{"key": "bad", "source_type": "remote", "behavior": "domain"}}, nil); got != http.StatusBadRequest {
		t.Fatalf("remote-without-url: got %d, want 400", got)
	}

	upd := map[string]any{"key": "custom", "name": "Renamed", "source_type": "remote", "url": "https://example.com/custom.txt", "behavior": "domain"}
	if got := h.do(t, req{method: http.MethodPut, path: fmt.Sprintf("/api/admin/subscription/rulesets/%d", created.ID), token: tok, body: upd}, nil); got != http.StatusOK {
		t.Fatalf("update ruleset: %d", got)
	}
	if got := h.do(t, req{method: http.MethodDelete, path: fmt.Sprintf("/api/admin/subscription/rulesets/%d", created.ID), token: tok}, nil); got != http.StatusOK {
		t.Fatalf("delete ruleset: %d", got)
	}
}
