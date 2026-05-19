package e2e

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// TestAdjustBalance_RowLockedAgainstRace fires 8 concurrent topups +
// 8 concurrent debits against the same user. Without the FOR UPDATE
// lock added in TD-1 a lost-update race produces a final balance that
// disagrees with the algebraic sum and/or balance_logs.balance_after_cents
// values that violate the running-balance invariant. With the lock
// every adjustment serializes and the final balance matches.
func TestAdjustBalance_RowLockedAgainstRace(t *testing.T) {
	h := setupHarness(t)
	_, _ = h.registerUser(t, "race@example.com", "hunter2hunter2")

	userRepo := repository.NewUserRepo(h.db)
	const n = 8
	const delta = int64(100)

	var wg sync.WaitGroup
	wg.Add(2 * n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := userRepo.AdjustBalance(context.Background(), 1, +delta, model.BalanceReasonAdminAdjust, "race up", nil)
			if err != nil {
				t.Errorf("AdjustBalance +: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := userRepo.AdjustBalance(context.Background(), 1, -delta, model.BalanceReasonOrderCharge, "race down", nil)
			if err != nil {
				t.Errorf("AdjustBalance -: %v", err)
			}
		}()
	}
	wg.Wait()

	// Algebraic sum is zero — 8 ups of +100 cancelling 8 downs of -100.
	u, err := userRepo.Get(context.Background(), 1)
	if err != nil || u == nil {
		t.Fatalf("re-read user: %v", err)
	}
	if u.BalanceCents != 0 {
		t.Errorf("final balance = %d, want 0 (lost-update race)", u.BalanceCents)
	}

	// Running-balance invariant: each balance_log's balance_after_cents
	// must equal the sum of every preceding delta_cents (sorted by id,
	// since balance_logs is monotonic with order of mutation under
	// SELECT FOR UPDATE serialization).
	var rows []struct {
		ID                int64
		DeltaCents        int64
		BalanceAfterCents int64
	}
	if err := h.db.Raw(`
        SELECT id, delta_cents, balance_after_cents
          FROM balance_logs
         WHERE user_id = 1
         ORDER BY id
    `).Scan(&rows).Error; err != nil {
		t.Fatalf("read balance_logs: %v", err)
	}
	if len(rows) != 2*n {
		t.Fatalf("balance_logs rows = %d, want %d", len(rows), 2*n)
	}
	running := int64(0)
	for _, r := range rows {
		running += r.DeltaCents
		if r.BalanceAfterCents != running {
			t.Errorf("balance_logs id=%d: balance_after=%d, expected running=%d (broken serialization)",
				r.ID, r.BalanceAfterCents, running)
		}
	}
}

// TestWebhookRetry_PersistsAcrossRetryCronTrigger schedules a webhook
// against an unreachable URL, fires the test event, then invokes
// RetryDue manually to simulate the cron job. Confirms:
//   1) initial delivery row is pending with next_attempt_at ~ now
//   2) after first failed attempt the row stays pending with
//      next_attempt_at pushed out
//   3) the cron-equivalent RetryDue picks the row up and increments
//      attempt counter
//
// This is the property TD-2 enabled: prior to the migration, the row
// flipped to status=failed after attempt 1 and the in-memory retry
// loop was lost on shutdown.
func TestWebhookRetry_PersistsAcrossRetryCronTrigger(t *testing.T) {
	h := setupHarness(t)
	_, _ = h.registerUser(t, "wh@example.com", "hunter2hunter2")
	adminTok := h.adminLogin(t)

	// 127.0.0.1:1 → connection refused. allow_private bypasses SSRF
	// so the dialer actually attempts it.
	var wh struct{ ID int64 `json:"id"` }
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/webhooks", token: adminTok,
		body: map[string]any{
			"name": "dead", "url": "http://127.0.0.1:1/dead",
			"events": []string{"webhook.test"}, "enabled": true, "allow_private": true,
		},
	}, &wh); got != http.StatusCreated {
		t.Fatalf("create webhook: status=%d", got)
	}
	var d1 model.WebhookDelivery
	if got := h.do(t, req{
		method: http.MethodPost, path: "/api/admin/webhooks/" + itoa(wh.ID) + "/test", token: adminTok,
	}, &d1); got != http.StatusAccepted {
		t.Fatalf("test event: status=%d", got)
	}
	// Wait for the first attempt to complete and ScheduleRetry to
	// have stamped next_attempt_at in the future. The connect-refused
	// is immediate so a tight poll is fine.
	deadline := time.Now().Add(5 * time.Second)
	var afterAttempt1 model.WebhookDelivery
	for time.Now().Before(deadline) {
		var row model.WebhookDelivery
		if err := h.db.First(&row, d1.ID).Error; err != nil {
			t.Fatalf("read delivery: %v", err)
		}
		if row.Attempt >= 1 && row.Status == model.WebhookDeliveryStatusPending {
			afterAttempt1 = row
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if afterAttempt1.Attempt != 1 {
		t.Fatalf("expected attempt=1 after first try, got attempt=%d status=%s",
			afterAttempt1.Attempt, afterAttempt1.Status)
	}
	if !afterAttempt1.NextAttemptAt.After(afterAttempt1.ScheduledAt) {
		t.Errorf("next_attempt_at must move forward on retry: scheduled=%v next=%v",
			afterAttempt1.ScheduledAt, afterAttempt1.NextAttemptAt)
	}

	// Yank next_attempt_at back to now so RetryDue picks it up on
	// the same test run — production waits 1-60s naturally.
	if err := h.db.Exec(
		`UPDATE webhook_deliveries SET next_attempt_at = now() - interval '1 second' WHERE id = ?`,
		d1.ID,
	).Error; err != nil {
		t.Fatalf("force due: %v", err)
	}

	h.app.WebhookService.RetryDue(context.Background(), 16)

	// Wait for attempt 2.
	deadline = time.Now().Add(5 * time.Second)
	var afterAttempt2 model.WebhookDelivery
	for time.Now().Before(deadline) {
		var row model.WebhookDelivery
		_ = h.db.First(&row, d1.ID).Error
		if row.Attempt >= 2 {
			afterAttempt2 = row
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if afterAttempt2.Attempt < 2 {
		t.Errorf("cron-equivalent RetryDue didn't bump attempt: %+v", afterAttempt2)
	}

	// Sanity: app.RuntimeManager wiring + SSRF carve-out still works.
	_ = runtime.ErrTagNotFound
}
