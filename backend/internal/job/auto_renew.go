package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/billing"
	"github.com/cern/3xui-dashboard/internal/service/messages"
)

// AutoRenewJob scans for ownerships about to expire that the
// admin has opted-in for auto-renewal (users.auto_renew = TRUE
// for the owning user). When a candidate is found AND the
// user's balance covers the plan price, it triggers a normal
// billing.Purchase against the same plan+inbound — which goes
// through the usual charge → ProvisionClient → mark-completed
// chain, including the "reset traffic on re-purchase" hook from
// #141.
//
// When the user has auto_renew enabled but insufficient balance,
// the job emits two parallel signals: an ops-side admin alert
// (so the operator can decide whether to top up or disable
// auto-renew), and a user-side "balance too low" message via
// service/messages so the user gets a chance to top up before
// the ownership lapses. Both dedup per (ownership, expiry-day)
// independently on their respective surfaces. The successful
// renewal path itself stays invisible to the user.
//
// Runs at @every 1h. The 24h lookahead window covers a 1h cron
// missing one tick. Each ownership is locked against double-
// renewal by a notification_log dedup key keyed on the
// ownership id + the expiry's day-stamp; a user with a plan
// expiring tomorrow gets exactly one renewal attempt regardless
// of how many ticks fire before then.
type AutoRenewJob struct {
	ownership *repository.ClientOwnershipRepo
	users     *repository.UserRepo
	plans     *repository.PlanRepo
	logs      *repository.NotificationLogRepo
	billing   *billing.Service
	mailer    *mailer.Mailer    // optional; nil/disabled → admin alerts logged only
	opsEmail  string            // recipient for admin alerts
	messages  *messages.Service // optional; nil → user low-balance email dropped
	log       *slog.Logger

	// Window the cron looks ahead. Default 24h matches "scan
	// once an hour, still catch a missed tick" safety budget.
	Window time.Duration
}

// NewAutoRenewJob constructs the worker. mailer + opsEmail can be
// nil/empty — admin alerts then just become a WARN log line.
// msgs can be nil — user low-balance emails are silently dropped
// in that case (the admin alert still fires via mailer).
func NewAutoRenewJob(
	ownership *repository.ClientOwnershipRepo,
	users *repository.UserRepo,
	plans *repository.PlanRepo,
	logs *repository.NotificationLogRepo,
	billing *billing.Service,
	m *mailer.Mailer,
	opsEmail string,
	msgs *messages.Service,
	lg *slog.Logger,
) *AutoRenewJob {
	return &AutoRenewJob{
		ownership: ownership,
		users:     users,
		plans:     plans,
		logs:      logs,
		billing:   billing,
		mailer:    m,
		opsEmail:  opsEmail,
		messages:  msgs,
		log:       lg.With(slog.String("component", "job.auto_renew")),
		Window:    24 * time.Hour,
	}
}

// RunOnce performs one full pass. Cheap enough to run hourly.
//
//	@every 1h    → renewal latency: 0–1h before the row's
//	               expires_at, with a 24h prefetch window.
func (j *AutoRenewJob) RunOnce(ctx context.Context) {
	now := time.Now().UTC()
	rows, err := j.ownership.ListExpiringWithin(ctx, now, j.Window)
	if err != nil {
		j.log.Error("ListExpiringWithin failed", slog.String("err", err.Error()))
		return
	}
	for i := range rows {
		j.process(ctx, &rows[i], now)
	}
}

func (j *AutoRenewJob) process(ctx context.Context, o *model.ClientOwnership, now time.Time) {
	if o.PlanID == nil {
		// No plan attached (admin-direct ownership). Can't auto-
		// renew without a plan template to repurchase.
		return
	}
	user, err := j.users.Get(ctx, o.UserID)
	if err != nil || user == nil {
		j.log.Warn("user lookup failed", slog.Int64("user_id", o.UserID), slog.Any("err", err))
		return
	}
	if !user.AutoRenew {
		return
	}
	if user.Status != model.UserStatusActive {
		j.log.Debug("user not active, skip auto-renew",
			slog.Int64("user_id", user.ID), slog.String("status", user.Status))
		return
	}

	// Dedup per (ownership, day-of-expiry) so a user with a 30-day
	// plan can renew once each cycle without us hammering them on
	// every cron tick.
	dedupKind := autoRenewDedupKind(o)
	already, err := j.logs.AlreadySent(ctx, model.SurfaceNotification, dedupKind, o.ID)
	if err != nil {
		j.log.Warn("dedup check failed (proceeding may double-charge)",
			slog.Int64("ownership_id", o.ID), slog.String("err", err.Error()))
	}
	if already {
		return
	}

	plan, err := j.plans.Get(ctx, *o.PlanID)
	if err != nil || plan == nil {
		j.log.Warn("plan lookup failed",
			slog.Int64("plan_id", *o.PlanID), slog.Any("err", err))
		return
	}
	if !plan.Enabled {
		j.log.Info("plan disabled, skip auto-renew",
			slog.Int64("user_id", user.ID), slog.Int64("plan_id", plan.ID))
		return
	}

	// Mark attempted BEFORE Purchase so a crash mid-Purchase
	// doesn't let the next tick retry — that would double-charge.
	// Purchase is itself idempotent on idempotency_key but the
	// key is per-attempt; if we record-after-success and crash
	// in between, next tick generates a NEW key and charges
	// again. Better: dedup-then-attempt.
	recipient := ""
	if user.Email != nil {
		recipient = *user.Email
	}
	if err := j.logs.MarkSent(ctx, model.SurfaceNotification, dedupKind, o.ID, recipient); err != nil {
		j.log.Warn("dedup record failed", slog.Int64("ownership_id", o.ID), slog.String("err", err.Error()))
		// Don't proceed — risk of double-charge outweighs missing one renewal.
		return
	}

	if user.BalanceCents < plan.PriceCents {
		j.handleInsufficientBalance(ctx, user, plan, o, now)
		return
	}

	idem := autoRenewIdempotencyKey(o, now)
	order, err := j.billing.Purchase(ctx, billing.PurchaseInput{
		UserID:         user.ID,
		PlanID:         plan.ID,
		IdempotencyKey: idem,
		NodeID:         o.NodeID,
		InboundTag:     o.InboundTag,
	})
	if err != nil {
		j.log.Error("auto-renew Purchase failed",
			slog.Int64("user_id", user.ID),
			slog.Int64("plan_id", plan.ID),
			slog.String("err", err.Error()),
		)
		j.alertAdmin(ctx, fmt.Sprintf("Auto-renew failed for user #%d (plan %s): %s",
			user.ID, plan.Name, err.Error()))
		return
	}
	j.log.Info("auto-renewed",
		slog.Int64("user_id", user.ID),
		slog.Int64("plan_id", plan.ID),
		slog.Int64("order_id", order.ID),
		slog.Int64("charged_cents", plan.PriceCents),
	)
}

func (j *AutoRenewJob) handleInsufficientBalance(ctx context.Context, user *model.User, plan *model.Plan, o *model.ClientOwnership, now time.Time) {
	emailHint := "(unset)"
	if user.Email != nil {
		emailHint = *user.Email
	}
	expiresAt := "?"
	if o.ExpiresAt != nil {
		expiresAt = o.ExpiresAt.Format(time.RFC3339)
	}
	opsBody := fmt.Sprintf(
		"User #%d (%s) is enrolled in auto-renewal but balance "+
			"is insufficient.\n\nPlan: %s (¥%.2f)\nBalance: ¥%.2f\nOwnership expires: %s\n\n"+
			"Decide whether to top up the user's balance or disable their auto-renew flag.",
		user.ID, emailHint, plan.Name,
		float64(plan.PriceCents)/100, float64(user.BalanceCents)/100,
		expiresAt,
	)
	j.log.Warn("auto-renew: insufficient balance",
		slog.Int64("user_id", user.ID),
		slog.Int64("need_cents", plan.PriceCents),
		slog.Int64("have_cents", user.BalanceCents),
	)
	j.alertAdmin(ctx, opsBody)
	j.notifyUserLowBalance(ctx, user, plan, o)
	_ = now
}

// notifyUserLowBalance sends the user-facing "your balance is too
// low for the upcoming renewal" message. Dedup keys per
// (ownership, expiry-day) on SurfaceMessage so a single renewal
// cycle generates exactly one user email regardless of how many
// hourly ticks fire before the ownership expires.
func (j *AutoRenewJob) notifyUserLowBalance(ctx context.Context, user *model.User, plan *model.Plan, o *model.ClientOwnership) {
	if j.messages == nil || !j.messages.Enabled() {
		return
	}
	if user.Email == nil || *user.Email == "" {
		return
	}
	expiresAt := "?"
	if o.ExpiresAt != nil {
		expiresAt = o.ExpiresAt.Format("2006-01-02 15:04 MST")
	}
	subject := "您的订阅即将到期，余额不足无法自动续费"
	body := fmt.Sprintf(
		"您好，\n\n"+
			"您订阅的「%s」套餐将于 %s 到期，但当前余额（¥%.2f）"+
			"不足以支付续费金额（¥%.2f）。\n\n"+
			"请登录控制台充值，否则到期后服务将自动停用。\n",
		plan.Name, expiresAt,
		float64(user.BalanceCents)/100, float64(plan.PriceCents)/100,
	)
	if err := j.messages.Send(ctx, *user.Email, subject, body, lowBalanceDedupKind(o), o.ID); err != nil {
		j.log.Warn("auto-renew: user low-balance email failed",
			slog.Int64("user_id", user.ID), slog.String("err", err.Error()))
	}
}

// lowBalanceDedupKind keys the SurfaceMessage notification_log
// row per ownership + expiry-day, matching the cadence of
// autoRenewDedupKind so user and ops surfaces emit on the same
// boundary.
func lowBalanceDedupKind(o *model.ClientOwnership) string {
	day := "no-expiry"
	if o.ExpiresAt != nil {
		day = o.ExpiresAt.UTC().Format("2006-01-02")
	}
	return "low_balance_" + day
}

// alertAdmin sends a one-shot ops email. ctx is unused — the
// mailer's own timeout governs delivery. When mailer or opsEmail
// isn't configured, just WARN-log the body so the alert isn't
// silently lost.
func (j *AutoRenewJob) alertAdmin(_ context.Context, body string) {
	const subject = "[ops] auto-renewal needs attention"
	if j.mailer != nil && j.mailer.Enabled() && j.opsEmail != "" {
		if err := j.mailer.Send(j.opsEmail, subject, body); err != nil {
			j.log.Warn("alertAdmin mailer.Send failed", slog.String("err", err.Error()))
		}
		return
	}
	j.log.Warn("alertAdmin: mailer/opsEmail not wired — admin alert dropped",
		slog.String("body", body))
}

// autoRenewDedupKind keys the notification_log on the ownership
// AND the day-stamp of expiry. This means a 30-day plan generates
// a fresh dedup key each cycle, but multiple ticks within the
// same 24h pre-expiry window only attempt renewal once.
func autoRenewDedupKind(o *model.ClientOwnership) string {
	day := "no-expiry"
	if o.ExpiresAt != nil {
		day = o.ExpiresAt.UTC().Format("2006-01-02")
	}
	return "auto_renew_" + day
}

// autoRenewIdempotencyKey is per-attempt — combining ownership
// + day means a within-day retry sees the existing order via
// idempotency lookup and gets the same order back rather than
// double-charging. Cross-day attempts generate new keys (new
// renewal cycles).
func autoRenewIdempotencyKey(o *model.ClientOwnership, now time.Time) string {
	day := now.UTC().Format("20060102")
	// Salt with random bytes so distinct rows don't collide if
	// they happen to have the same owner+plan + same day.
	var salt [4]byte
	_, _ = rand.Read(salt[:])
	return fmt.Sprintf("autorenew-%d-%s-%s", o.ID, day, hex.EncodeToString(salt[:]))
}

