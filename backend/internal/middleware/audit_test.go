package middleware

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		path           string
		wantResource   string
		wantID         string
	}{
		{"/api/admin/orders/42/refund", "orders", "42"},
		{"/api/admin/users/7", "users", "7"},
		{"/api/admin/webhooks/3/test", "webhooks", "3"},
		{"/api/admin/settings/smtp-test", "settings", "smtp-test"},
		{"/api/admin/plans", "plans", ""},
		{"/api/admin/", "", ""},
		{"/api/admin", "", ""},
		{"/healthz", "", ""},                      // not admin
		{"/api/user/auth/login", "", ""},          // not admin
		{"/api/admin/nodes/1/probe", "nodes", "1"},
	}
	for _, tc := range cases {
		gotR, gotID := parseTarget(tc.path)
		if gotR != tc.wantResource || gotID != tc.wantID {
			t.Errorf("parseTarget(%q) = (%q, %q), want (%q, %q)",
				tc.path, gotR, gotID, tc.wantResource, tc.wantID)
		}
	}
}
