package job

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// ExpiryJob is the periodic worker that watches client ownerships for
// two billing events:
//
//  1. **Expired** — `expires_at <= now()` while `enabled = true`. The
//     row is flipped to `enabled = false` so the user's subscription
//     stops rendering this client, and a `client.expired` event is
//     published. Webhook / mailer subscribers can act on it.
//
//  2. **Expiring soon** — `expires_at` within the `expiry_warn_days`
//     setting window (default 3 days). A `client.expiring_soon` event
//     is published so a downstream notifier can email the user.
//
// Per-row dedup is handled with an in-memory map: we publish each
// (id, kind) pair at most once per process lifetime to avoid
// re-notifying on every cron tick. Restarts clear the dedup —
// acceptable for v1, since the underlying ownership row's
// `enabled = false` flip is the actual idempotency boundary for
// the "expired" branch.
//
// Aggressive node-side termination (calling Remote.UpdateClient
// with Enable=false on the panel) is OUT of scope for v1 — the DB
// flip + Assembler.Build filter is enough to stop new subscription
// fetches from including the link. Existing user apps that already
// cached the credentials will keep working until the node itself
// expires them. See ROADMAP §1 if this needs to be hardened.
type ExpiryJob struct {
	ownership *repository.ClientOwnershipRepo
	settings  *repository.SettingRepo
	users     *repository.UserRepo
	bus       *event.Bus
	log       *slog.Logger

	mu       sync.Mutex
	notified map[string]time.Time // dedup key → first-notified
}

// NewExpiryJob constructs an ExpiryJob.
func NewExpiryJob(
	ownership *repository.ClientOwnershipRepo,
	settings *repository.SettingRepo,
	users *repository.UserRepo,
	bus *event.Bus,
	lg *slog.Logger,
) *ExpiryJob {
	return &ExpiryJob{
		ownership: ownership,
		settings:  settings,
		users:     users,
		bus:       bus,
		log:       lg.With(slog.String("component", "job.expiry")),
		notified:  make(map[string]time.Time),
	}
}

// ExpiredPayload is what subscribers see for client.expired events
// fired by this job (distinct from the panel-reported expiry which
// uses traffic/service.go::ClientExpiredPayload — they share the
// type name across files but live in different packages).
type ExpiredPayload struct {
	OwnershipID int64      `json:"ownership_id"`
	UserID      int64      `json:"user_id"`
	UserEmail   string     `json:"user_email,omitempty"`
	NodeID      int64      `json:"node_id"`
	InboundTag  string     `json:"inbound_tag"`
	ClientEmail string     `json:"client_email"`
	ExpiredAt   time.Time  `json:"expired_at"`
}

// ExpiringSoonPayload describes one client whose plan is about to
// run out. Subscribers (mailer + telegram bot in the future) format
// it into a renewal nudge.
type ExpiringSoonPayload struct {
	OwnershipID    int64     `json:"ownership_id"`
	UserID         int64     `json:"user_id"`
	UserEmail      string    `json:"user_email,omitempty"`
	NodeID         int64     `json:"node_id"`
	InboundTag     string    `json:"inbound_tag"`
	ClientEmail    string    `json:"client_email"`
	ExpiresAt      time.Time `json:"expires_at"`
	DaysRemaining  int       `json:"days_remaining"`
}

// RunOnce performs one full pass. Cheap enough to run every 5 min.
//
//	@every 5m   → expired branch latency ≤ 5 minutes
//	(same loop) → expiring-soon branch covers warn window
func (j *ExpiryJob) RunOnce(ctx context.Context) {
	now := time.Now().UTC()

	// ---- expired branch ----------------------------------------------------
	expired, err := j.ownership.ListExpired(ctx, now)
	if err != nil {
		j.log.Error("list expired ownerships", "err", err)
	} else {
		for i := range expired {
			j.processExpired(ctx, &expired[i], now)
		}
	}

	// ---- expiring-soon branch ----------------------------------------------
	warnDays := j.warnDays(ctx)
	if warnDays <= 0 {
		return
	}
	window := time.Duration(warnDays) * 24 * time.Hour
	soon, err := j.ownership.ListExpiringWithin(ctx, now, window)
	if err != nil {
		j.log.Error("list expiring-soon ownerships", "err", err)
		return
	}
	for i := range soon {
		j.processExpiringSoon(ctx, &soon[i], now)
	}
}

func (j *ExpiryJob) processExpired(ctx context.Context, o *model.ClientOwnership, now time.Time) {
	// Flip DB enabled=false. The Assembler.Build loop already skips
	// disabled rows when serving subscriptions.
	if err := j.ownership.SetEnabled(ctx, o.ID, false); err != nil {
		j.log.Error("disable expired ownership",
			"id", o.ID, "err", err)
		return
	}
	// Each ownership emits the event at most once per process — the
	// DB flip ensures we won't see this row again on the next tick
	// anyway, but the dedup map handles the rare case of a manual
	// re-enable mid-process.
	key := "expired|" + strconv.FormatInt(o.ID, 10)
	if !j.shouldNotify(key, now) {
		return
	}
	payload := ExpiredPayload{
		OwnershipID: o.ID,
		UserID:      o.UserID,
		UserEmail:   j.userEmailFor(ctx, o.UserID),
		NodeID:      o.NodeID,
		InboundTag:  o.InboundTag,
		ClientEmail: o.ClientEmail,
	}
	if o.ExpiresAt != nil {
		payload.ExpiredAt = *o.ExpiresAt
	}
	j.bus.PublishType(event.ClientExpired, payload)
	j.log.Info("client expired",
		"ownership_id", o.ID,
		"user_id", o.UserID,
		"node_id", o.NodeID,
		"client_email", o.ClientEmail,
	)
}

func (j *ExpiryJob) processExpiringSoon(ctx context.Context, o *model.ClientOwnership, now time.Time) {
	if o.ExpiresAt == nil {
		return
	}
	// One notification per ownership per process. With a 5-min cron
	// and a 3-day warn window, a fresh process will fire ≤1 event
	// per ownership in that window. Restart → notification could
	// re-fire (acceptable for v1; persistent dedup is a future
	// `notification_log` table.)
	key := "expiring_soon|" + strconv.FormatInt(o.ID, 10)
	if !j.shouldNotify(key, now) {
		return
	}
	days := int(o.ExpiresAt.Sub(now).Hours() / 24)
	if days < 0 {
		days = 0
	}
	j.bus.PublishType(event.ClientExpiringSoon, ExpiringSoonPayload{
		OwnershipID:   o.ID,
		UserID:        o.UserID,
		UserEmail:     j.userEmailFor(ctx, o.UserID),
		NodeID:        o.NodeID,
		InboundTag:    o.InboundTag,
		ClientEmail:   o.ClientEmail,
		ExpiresAt:     *o.ExpiresAt,
		DaysRemaining: days,
	})
	j.log.Info("client expiring soon",
		"ownership_id", o.ID,
		"user_id", o.UserID,
		"days_remaining", days,
	)
}

// shouldNotify is the per-process dedup. Returns true exactly once
// per key; subsequent calls return false until the process restarts.
func (j *ExpiryJob) shouldNotify(key string, now time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, seen := j.notified[key]; seen {
		return false
	}
	j.notified[key] = now
	return true
}

// warnDays reads the configured `expiry_warn_days` setting. Returns
// the env-derived default (3) on any read error.
func (j *ExpiryJob) warnDays(ctx context.Context) int {
	if j.settings == nil {
		return 3
	}
	v, ok, err := j.settings.Get(ctx, model.SettingExpiryWarnDays)
	if err != nil || !ok || v == "" {
		return 3
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		j.log.Warn("invalid expiry_warn_days setting, using default 3",
			"value", v, "err", err)
		return 3
	}
	return n
}

// userEmailFor looks up the owning user's email for inclusion in the
// payload. Best-effort: returns "" on any error (the user may have
// been deleted between ownership creation and expiry).
func (j *ExpiryJob) userEmailFor(ctx context.Context, userID int64) string {
	if j.users == nil {
		return ""
	}
	u, err := j.users.Get(ctx, userID)
	if err != nil || u == nil || u.Email == nil {
		return ""
	}
	return *u.Email
}
