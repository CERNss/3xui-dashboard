// Package notify bridges domain events to user-facing channels —
// today email (via mailer); future Telegram / Discord / Slack
// subscribers slot in here too.
//
// The bridge subscribes to the in-process event bus for client-
// lifecycle events (expired / expiring_soon / over_limit), resolves
// the owning user's email, formats a templated body, and dispatches
// via mailer.Send.
//
// Persistent dedup: we write a `notification_log` row before sending
// so a restart in the middle of a warn window doesn't re-spam the
// user. The row is upserted by (kind, ownership_id) and the send
// happens only if the insert is new — DB conflict means we already
// notified.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
	jobpkg "github.com/cern/3xui-dashboard/internal/job"
)

// kind enumerates the notification flavours this bridge handles.
// Kept narrow so a future change adding a new event type forces an
// explicit code update (rather than silently silencing the new
// event).
type kind string

const (
	kindExpired      kind = "expired_email"
	kindExpiringSoon kind = "expiring_soon_email"
	kindOverLimit    kind = "over_limit_email"
)

// Service wires the bus subscriber + mailer + repos. It's started
// once at app boot; there is no Stop() — the bus is process-scoped
// and dies with the binary.
type Service struct {
	bus       *event.Bus
	mailer    *mailer.Mailer
	users     *repository.UserRepo
	ownership *repository.ClientOwnershipRepo
	logs      *repository.NotificationLogRepo
	log       *slog.Logger
}

// New wires the service. Call Start() to register handlers.
func New(
	bus *event.Bus,
	mailer *mailer.Mailer,
	users *repository.UserRepo,
	ownership *repository.ClientOwnershipRepo,
	logs *repository.NotificationLogRepo,
	lg *slog.Logger,
) *Service {
	return &Service{
		bus:       bus,
		mailer:    mailer,
		users:     users,
		ownership: ownership,
		logs:      logs,
		log:       lg.With(slog.String("component", "service.notify")),
	}
}

// Start subscribes to the relevant event types. Idempotent in the
// sense that re-calling registers extra handlers — only call once.
func (s *Service) Start() {
	s.bus.Subscribe(event.ClientExpired, s.onExpired)
	s.bus.Subscribe(event.ClientExpiringSoon, s.onExpiringSoon)
	s.bus.Subscribe(event.ClientOverLimit, s.onOverLimit)
}

// ---- handlers --------------------------------------------------------------

func (s *Service) onExpired(e event.Event) {
	// Two source shapes converge on this event:
	//   - job.ExpiredPayload — emitted by the DB-side ExpiryJob
	//     (we have UserID + OwnershipID directly)
	//   - traffic.ClientExpiredPayload — emitted by traffic.evaluateRules
	//     from the panel's reported ExpiryTime (no UserID; lookup needed)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var ownership *model.ClientOwnership
	var expiredAt time.Time

	switch p := e.Data.(type) {
	case jobpkg.ExpiredPayload:
		ownership = &model.ClientOwnership{
			ID: p.OwnershipID, UserID: p.UserID,
			NodeID: p.NodeID, InboundTag: p.InboundTag, ClientEmail: p.ClientEmail,
		}
		expiredAt = p.ExpiredAt
	case traffic.ClientExpiredPayload:
		o, err := s.lookupOwnership(ctx, p.NodeID, p.InboundTag, p.ClientEmail)
		if err != nil || o == nil {
			s.log.Warn("client.expired without resolvable ownership", "node_id", p.NodeID, "client_email", p.ClientEmail, "err", err)
			return
		}
		ownership = o
		expiredAt = p.ExpiredAt
	default:
		s.log.Warn("client.expired with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}

	s.dispatch(ctx, kindExpired, ownership, func(toEmail string) (string, string) {
		subject := "【3xui Central】您的服务已到期"
		body := fmt.Sprintf(
			"您订阅的客户端已到期，已自动停用：\n\n"+
				"  节点：#%d\n"+
				"  入站：%s\n"+
				"  客户端：%s\n"+
				"  到期时间：%s\n\n"+
				"如需继续使用，请登录控制台购买新套餐。\n",
			ownership.NodeID, ownership.InboundTag, ownership.ClientEmail, expiredAt.Format(time.RFC3339),
		)
		return subject, body
	})
}

func (s *Service) onExpiringSoon(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, ok := e.Data.(jobpkg.ExpiringSoonPayload)
	if !ok {
		s.log.Warn("client.expiring_soon with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	ownership := &model.ClientOwnership{
		ID: p.OwnershipID, UserID: p.UserID,
		NodeID: p.NodeID, InboundTag: p.InboundTag, ClientEmail: p.ClientEmail,
	}
	s.dispatch(ctx, kindExpiringSoon, ownership, func(toEmail string) (string, string) {
		subject := "【3xui Central】您的服务即将到期"
		body := fmt.Sprintf(
			"您订阅的客户端将在 %d 天后到期：\n\n"+
				"  节点：#%d\n"+
				"  入站：%s\n"+
				"  客户端：%s\n"+
				"  到期时间：%s\n\n"+
				"为避免服务中断，请提前登录控制台续费。\n",
			p.DaysRemaining, ownership.NodeID, ownership.InboundTag, ownership.ClientEmail, p.ExpiresAt.Format(time.RFC3339),
		)
		return subject, body
	})
}

func (s *Service) onOverLimit(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, ok := e.Data.(traffic.ClientThresholdPayload)
	if !ok {
		s.log.Warn("client.over_limit with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	o, err := s.lookupOwnership(ctx, p.NodeID, p.InboundTag, p.ClientEmail)
	if err != nil || o == nil {
		s.log.Warn("client.over_limit without resolvable ownership", "node_id", p.NodeID, "client_email", p.ClientEmail, "err", err)
		return
	}
	s.dispatch(ctx, kindOverLimit, o, func(toEmail string) (string, string) {
		subject := "【3xui Central】流量已用尽"
		body := fmt.Sprintf(
			"您订阅的客户端流量已用尽，连接将受限：\n\n"+
				"  节点：%s（#%d）\n"+
				"  入站：%s\n"+
				"  客户端：%s\n"+
				"  上行 / 下行：%d / %d 字节\n"+
				"  上限：%d 字节\n\n"+
				"如需继续使用，请登录控制台续费或购买流量包。\n",
			p.NodeName, p.NodeID, p.InboundTag, p.ClientEmail, p.Up, p.Down, p.Limit,
		)
		return subject, body
	})
}

// ---- helpers ---------------------------------------------------------------

// dispatch is the shared path: dedup via notification_log, fetch user
// email, build body, send via mailer. The builder closure is what
// makes the body per-kind.
func (s *Service) dispatch(
	ctx context.Context,
	k kind,
	o *model.ClientOwnership,
	build func(toEmail string) (subject, body string),
) {
	if o == nil || o.UserID == 0 || o.ID == 0 {
		s.log.Warn("notify.dispatch missing ownership identity", "kind", k)
		return
	}
	user, err := s.users.Get(ctx, o.UserID)
	if err != nil {
		s.log.Error("user lookup", "user_id", o.UserID, "err", err)
		return
	}
	if user == nil || user.Email == nil || *user.Email == "" {
		// No email to send to. Still log to dedup table so we don't
		// re-check on every cron tick.
		_ = s.logs.MarkSent(ctx, string(k), o.ID, "")
		s.log.Info("notify skipped — no email", "kind", k, "user_id", o.UserID)
		return
	}

	already, err := s.logs.AlreadySent(ctx, string(k), o.ID)
	if err != nil {
		s.log.Error("dedup check failed", "kind", k, "err", err)
		// fall through — we'd rather risk a duplicate email than skip
		// a critical notice
	}
	if already {
		return
	}

	subject, body := build(*user.Email)
	if err := s.mailer.Send(*user.Email, subject, body); err != nil {
		// Don't mark sent — let the next tick retry.
		s.log.Error("mailer.Send failed", "kind", k, "to", *user.Email, "err", err)
		return
	}
	if err := s.logs.MarkSent(ctx, string(k), o.ID, *user.Email); err != nil {
		s.log.Warn("MarkSent failed (email already delivered)", "kind", k, "err", err)
	}
	s.log.Info("notify delivered", "kind", k, "to", *user.Email, "ownership_id", o.ID)
}

// lookupOwnership resolves a (nodeID, inboundTag, email) triple to
// the owning row. Returns (nil, nil) when not found.
func (s *Service) lookupOwnership(ctx context.Context, nodeID int64, inboundTag, email string) (*model.ClientOwnership, error) {
	o, err := s.ownership.GetByTriple(ctx, nodeID, inboundTag, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return o, nil
}
