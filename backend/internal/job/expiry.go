package job

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
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
// WGRemover is the optional hook ExpiryJob calls when an expired
// ownership lives on a WireGuard inbound. The unified UpdateClient
// path doesn't apply (the fork's WG inbound has no per-peer enable
// bit) — disabling means removing the peer entry entirely.
// Implemented by *service/client.WGProvisioner; app.Build wires
// it in when WG_MASTER_KEY is configured.
type WGRemover interface {
	RemovePeer(ctx context.Context, nodeID int64, inboundTag, clientEmail string) error
}

type ExpiryJob struct {
	ownership *repository.ClientOwnershipRepo
	settings  *repository.SettingRepo
	users     *repository.UserRepo
	logs      *repository.NotificationLogRepo
	rt        *runtime.Manager
	wg        WGRemover // optional; nil → WG ownerships fall back to DB-only disable
	bus       *event.Bus
	log       *slog.Logger
}

// SetWGRemover attaches the WG peer-removal hook. Idempotent.
// Called by app.Build when cfg.WireGuard.Enabled().
func (j *ExpiryJob) SetWGRemover(r WGRemover) { j.wg = r }

// NewExpiryJob constructs an ExpiryJob.
//
// `rt` is used to call UpdateClient(Enable=false) on the node so
// already-cached client credentials stop working immediately, not
// just on the dashboard side. `logs` provides persistent dedup so
// restarts in mid-warn-window don't re-spam users.
func NewExpiryJob(
	ownership *repository.ClientOwnershipRepo,
	settings *repository.SettingRepo,
	users *repository.UserRepo,
	logs *repository.NotificationLogRepo,
	rt *runtime.Manager,
	bus *event.Bus,
	lg *slog.Logger,
) *ExpiryJob {
	return &ExpiryJob{
		ownership: ownership,
		settings:  settings,
		users:     users,
		logs:      logs,
		rt:        rt,
		bus:       bus,
		log:       lg.With(slog.String("component", "job.expiry")),
	}
}

// ExpiredPayload aliases payload.ClientExpired so the publisher's
// own publishes stay readable. The canonical type is in
// internal/service/event/payload — subscribers import that directly.
type ExpiredPayload = payload.ClientExpired

// ExpiringSoonPayload aliases payload.ClientExpiringSoon — see
// ExpiredPayload for the alias rationale.
type ExpiringSoonPayload = payload.ClientExpiringSoon

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
	// 1) Flip DB enabled=false so Assembler.Build stops including
	// the row in subscription output.
	if err := j.ownership.SetEnabled(ctx, o.ID, false); err != nil {
		j.log.Error("disable expired ownership", "id", o.ID, "err", err)
		return
	}

	// 2) Aggressive node-side disable: push UpdateClient with
	// Enable=false so the user's existing app loses connection
	// immediately, not just on next sub refresh. Best-effort — if
	// the node is offline / unreachable, we log and continue. The
	// next traffic-snapshot cycle plus the dashboard sub filter
	// keep the system consistent.
	if err := j.disableOnNode(ctx, o); err != nil {
		j.log.Warn("node-side client disable failed (DB still flipped)",
			"ownership_id", o.ID, "node_id", o.NodeID, "err", err)
	}

	// 3) Publish event. Persistent dedup via notification_log (the
	// notify service is the primary subscriber but anyone listening
	// also gets called — bus is fire-and-forget). The DB flip already
	// guarantees ListExpired won't return this row on the next tick.
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

// disableOnNode pushes the appropriate node-side disable for an
// expired ownership. For Xray-family inbounds (VLESS/VMess/Trojan/
// SS/Hysteria) the unified `UpdateClient(Email, Enable=false)`
// flips the per-client enable bit. For WireGuard there is no such
// bit — the peer entry has to be removed from settings.peers[]
// via the WGProvisioner's advisory-locked RMW path.
//
// Fast path: the row's `protocol` column is populated by
// ProvisionClient on every forward provision, so the WG vs non-WG
// decision happens without a runtime lookup. Legacy rows from
// before migration 0008 have empty protocol — for those we fall
// back to a one-time GetInbound. Once the legacy row is re-touched
// (Upsert via re-provision / renewal), the column populates and
// future passes are O(0) on the runtime side.
//
// Skips silently if the runtime manager is nil (test fixtures) or
// the node is disabled — in either case pushing makes no sense.
func (j *ExpiryJob) disableOnNode(ctx context.Context, o *model.ClientOwnership) error {
	if j.rt == nil {
		return nil
	}
	r, err := j.rt.Get(ctx, o.NodeID)
	if err != nil {
		if errors.Is(err, runtime.ErrNodeDisabled) || errors.Is(err, runtime.ErrNodeNotFound) {
			return nil
		}
		return err
	}

	protocol := o.Protocol
	if protocol == "" {
		// Legacy row — one-time runtime lookup to decide.
		in, err := r.GetInbound(ctx, o.InboundTag)
		if err != nil {
			if errors.Is(err, runtime.ErrTagNotFound) {
				return nil
			}
			return err
		}
		protocol = in.Protocol
	}

	if protocol == "wireguard" {
		if j.wg == nil {
			// WG_MASTER_KEY not configured — we can't decrypt or
			// reconstitute the peer. Log + leave the DB flip as the
			// only enforcement layer.
			j.log.Warn("expired WG ownership but no WGRemover wired — DB-only disable",
				"ownership_id", o.ID, "node_id", o.NodeID, "inbound", o.InboundTag)
			return nil
		}
		return j.wg.RemovePeer(ctx, o.NodeID, o.InboundTag, o.ClientEmail)
	}
	// Push Enable=false. UpdateClient looks up the existing client
	// by email on the node and merges; passing only Enable + Email
	// is enough to flip the bit without touching uuid/password/flow.
	return r.UpdateClient(ctx, o.InboundTag, runtime.Client{
		Email:  o.ClientEmail,
		Enable: false,
	})
}

func (j *ExpiryJob) processExpiringSoon(ctx context.Context, o *model.ClientOwnership, now time.Time) {
	if o.ExpiresAt == nil {
		return
	}
	// Persistent dedup via notification_log — survives restarts.
	// Uses kind="expiring_soon_published" so it doesn't collide with
	// the notify service's own dedup keys (e.g. "expiring_soon_email"),
	// keeping the two layers independent: ExpiryJob dedups bus
	// publishes, notify dedups email sends. A future Telegram
	// subscriber would dedup with its own "expiring_soon_telegram".
	const publishKind = "expiring_soon_published"
	already, err := j.logs.AlreadySent(ctx, model.SurfaceNotification, publishKind, o.ID)
	if err != nil {
		j.log.Warn("AlreadySent check failed, proceeding (may double-notify)", "err", err)
	}
	if already {
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
	// Mark the publish dedup BEFORE returning so the next tick sees it.
	if err := j.logs.MarkSent(ctx, model.SurfaceNotification, publishKind, o.ID, ""); err != nil {
		j.log.Warn("MarkSent failed (may double-publish next tick)", "err", err)
	}
	j.log.Info("client expiring soon",
		"ownership_id", o.ID,
		"user_id", o.UserID,
		"days_remaining", days,
	)
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
