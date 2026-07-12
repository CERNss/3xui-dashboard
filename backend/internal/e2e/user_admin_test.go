package e2e

import (
	"net/http"
	"testing"
)

// TestAdminCreateUserWithInitialBalance pins the "create user with an
// initial balance" chain end-to-end: POST /api/admin/users with
// initial_balance_cents must credit the new account immediately AND
// leave an admin_adjust audit row in balance_logs. Regression test for
// the operator report "填了初始余额但没入账".
func TestAdminCreateUserWithInitialBalance(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)

	var created struct {
		ID           int64 `json:"id"`
		BalanceCents int64 `json:"balance_cents"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/users", token: adminTok,
		body: map[string]any{
			"email":                 "funded@example.com",
			"password":              "password-123",
			"initial_balance_cents": 1234,
		},
	}, &created); got != http.StatusCreated {
		t.Fatalf("create user: status=%d, want 201", got)
	}
	if created.ID == 0 {
		t.Fatal("create user: response carries no id")
	}
	if created.BalanceCents != 1234 {
		t.Errorf("create response balance_cents = %d, want 1234", created.BalanceCents)
	}

	// The credit must be persisted, not just echoed in the response.
	var fetched struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	if got := h.do(t, req{
		method: http.MethodGet,
		path:   "/api/admin/users/" + itoa(created.ID), token: adminTok,
	}, &fetched); got != http.StatusOK {
		t.Fatalf("get user: status=%d, want 200", got)
	}
	if fetched.BalanceCents != 1234 {
		t.Errorf("persisted balance_cents = %d, want 1234", fetched.BalanceCents)
	}

	// Audit trail: exactly one admin_adjust credit row.
	var log struct {
		Count             int64
		DeltaCents        int64
		BalanceAfterCents int64
	}
	if err := h.db.Raw(`
		SELECT COUNT(*) AS count,
		       COALESCE(MAX(delta_cents), 0) AS delta_cents,
		       COALESCE(MAX(balance_after_cents), 0) AS balance_after_cents
		FROM balance_logs WHERE user_id = ? AND reason = 'admin_adjust'`, created.ID,
	).Scan(&log).Error; err != nil {
		t.Fatalf("query balance_logs: %v", err)
	}
	if log.Count != 1 {
		t.Fatalf("balance_logs admin_adjust rows = %d, want 1", log.Count)
	}
	if log.DeltaCents != 1234 || log.BalanceAfterCents != 1234 {
		t.Errorf("balance_log delta=%d after=%d, want 1234/1234", log.DeltaCents, log.BalanceAfterCents)
	}

	// Zero-balance create must not write a phantom log row.
	var plain struct {
		ID           int64 `json:"id"`
		BalanceCents int64 `json:"balance_cents"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/users", token: adminTok,
		body: map[string]any{"email": "plain@example.com", "password": "password-123"},
	}, &plain); got != http.StatusCreated {
		t.Fatalf("create plain user: status=%d, want 201", got)
	}
	if plain.BalanceCents != 0 {
		t.Errorf("plain user balance_cents = %d, want 0", plain.BalanceCents)
	}
	var plainLogs int64
	if err := h.db.Raw(`SELECT COUNT(*) FROM balance_logs WHERE user_id = ?`, plain.ID).
		Scan(&plainLogs).Error; err != nil {
		t.Fatalf("query balance_logs: %v", err)
	}
	if plainLogs != 0 {
		t.Errorf("plain user balance_logs = %d, want 0", plainLogs)
	}
}
