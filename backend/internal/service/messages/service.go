// Package messages delivers user-facing transactional emails over
// SMTP. It owns the "messages" surface in the messages/notifications
// split: single channel (SMTP), single recipient (the user's bound
// email), no multi-channel routing, no admin webhooks. Ops-facing
// alerts go through service/notify instead.
//
// Two entry points:
//
//  1. Direct send (Send) — for code paths that already have the
//     subject + body + recipient resolved, e.g. verification codes
//     and auto-renew's user-side low-balance email.
//
//  2. Bus subscriber (Start → on*) — for client lifecycle events
//     (client.expired / expiring_soon / over_limit) where the user
//     deserves a personal email. The handler resolves
//     ownership → user → email and calls Send internally.
//
// Dedup: per (model.SurfaceMessage, kind, ownership_id) in
// notification_log when the caller / handler provides both an
// ownership ID and a kind. Transactional one-shots (verification
// codes) skip the dedup log entirely — rate-limiting lives upstream
// where the code/token is generated.
package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
)

// Mailer is the subset of mailer.Mailer this service depends on.
// Declared as an interface so tests can inject a counter without
// standing up a real SMTP server. Production uses *mailer.Mailer
// which satisfies this shape.
type Mailer interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// NotificationLogStore is the subset of repository.NotificationLogRepo
// this service needs. Defined locally so tests can stub it.
type NotificationLogStore interface {
	AlreadySent(ctx context.Context, surface, kind string, ownershipID int64) (bool, error)
	MarkSent(ctx context.Context, surface, kind string, ownershipID int64, userEmail string) error
}

// Service wraps the mailer with messages-surface semantics. The
// mailer may be nil or unconfigured — Send becomes a no-op in
// that case (consistent with the rest of the codebase's "SMTP
// optional" stance). bus / users / ownership are required when
// Start() is called to subscribe to client-lifecycle events;
// callers that only use the direct Send API can pass nil for them.
type Service struct {
	mailer    Mailer
	logs      NotificationLogStore
	bus       *event.Bus
	users     *repository.UserRepo
	ownership *repository.ClientOwnershipRepo
	log       *slog.Logger
}

// New wires the service. lg must not be nil. m may be nil — Send
// then becomes a no-op (consistent with the SMTP-optional stance).
func New(
	m Mailer,
	logs NotificationLogStore,
	bus *event.Bus,
	users *repository.UserRepo,
	ownership *repository.ClientOwnershipRepo,
	lg *slog.Logger,
) *Service {
	return &Service{
		mailer:    m,
		logs:      logs,
		bus:       bus,
		users:     users,
		ownership: ownership,
		log:       lg.With(slog.String("component", "service.messages")),
	}
}

// Start subscribes to bus events that have a user-facing payload.
// Idempotent in the sense that re-calling registers extra handlers
// — only call once.
//
// notify.Service subscribes to the SAME events for ops-channel
// fanout; the two subscribers run independently (different surfaces,
// different dedup keys) so a single client.expired event produces
// at most one user email AND at most one ops alert per configured
// channel.
func (s *Service) Start() {
	if s.bus == nil {
		// Tests constructed without a bus — no subscribers to wire.
		return
	}
	s.bus.Subscribe(event.ClientExpired, s.onClientExpired)
	s.bus.Subscribe(event.ClientExpiringSoon, s.onClientExpiringSoon)
	s.bus.Subscribe(event.ClientOverLimit, s.onClientOverLimit)
}

// Enabled reports whether the underlying mailer is configured.
// Callers can branch on this for dev-mode behavior (e.g.
// verification logs the code to stderr when SMTP is off).
func (s *Service) Enabled() bool {
	return s.mailer != nil && s.mailer.Enabled()
}

// Send delivers one transactional email. Returns nil + logs at
// debug when the mailer is disabled — callers treat that as a
// soft success.
//
// Dedup: when dedupOwnershipID > 0 AND dedupKind != "", the
// service checks notification_log for an existing
// (SurfaceMessage, dedupKind, dedupOwnershipID) row and skips if
// found. After a successful send, a row is recorded. Pass zero
// values for dedupOwnershipID / empty dedupKind to disable dedup
// (rate-limited one-shots like verification codes do this).
//
// Errors from mailer.Send are returned wrapped; dedup-log errors
// after a successful send are logged but not returned (delivery
// already happened — the caller shouldn't retry).
func (s *Service) Send(
	ctx context.Context,
	to, subject, body string,
	dedupKind string,
	dedupOwnershipID int64,
) error {
	if !s.Enabled() {
		s.log.Debug("messages.Send: mailer disabled, dropping",
			slog.String("to", to), slog.String("subject", subject))
		return nil
	}
	if to == "" {
		return fmt.Errorf("messages.Send: empty recipient")
	}

	if dedupOwnershipID > 0 && dedupKind != "" {
		already, err := s.logs.AlreadySent(ctx, model.SurfaceMessage, dedupKind, dedupOwnershipID)
		if err != nil {
			// Fall through — better to risk a dup than miss the message.
			s.log.Warn("messages: dedup check failed (proceeding)",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID),
				slog.String("err", err.Error()))
		}
		if already {
			s.log.Debug("messages: dedup hit, skipping",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID))
			return nil
		}
	}

	if err := s.mailer.Send(to, subject, body); err != nil {
		return fmt.Errorf("messages.Send mailer: %w", err)
	}

	if dedupOwnershipID > 0 && dedupKind != "" {
		if err := s.logs.MarkSent(ctx, model.SurfaceMessage, dedupKind, dedupOwnershipID, to); err != nil {
			s.log.Warn("messages: MarkSent failed (delivery succeeded)",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID),
				slog.String("err", err.Error()))
		}
	}

	s.log.Info("messages delivered",
		slog.String("to", to), slog.String("subject", subject),
		slog.String("kind", dedupKind))
	return nil
}

// ---- bus handlers: per-user client lifecycle -----------------------------

func (s *Service) onClientExpired(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		o         *model.ClientOwnership
		expiredAt time.Time
	)
	switch p := e.Data.(type) {
	case payload.ClientExpired:
		o = &model.ClientOwnership{
			ID: p.OwnershipID, UserID: p.UserID,
			NodeID: p.NodeID, InboundTag: p.InboundTag, ClientEmail: p.ClientEmail,
		}
		expiredAt = p.ExpiredAt
	case payload.TrafficClientExpired:
		got, err := s.resolveOwnership(ctx, p.NodeID, p.InboundTag, p.ClientEmail)
		if err != nil || got == nil {
			s.log.Warn("messages: client.expired without resolvable ownership",
				"node_id", p.NodeID, "client_email", p.ClientEmail, "err", err)
			return
		}
		o = got
		expiredAt = p.ExpiredAt
	default:
		s.log.Warn("messages: client.expired with unknown payload type",
			"type", fmt.Sprintf("%T", e.Data))
		return
	}

	to := s.userEmail(ctx, o.UserID)
	if to == "" {
		return
	}
	subject := "您的服务已到期"
	body := fmt.Sprintf(
		"您订阅的客户端已到期，已自动停用。如需继续使用，请登录控制台购买新套餐。\n\n"+
			"节点：#%d\n入站：%s\n客户端：%s\n到期时间：%s\n",
		o.NodeID, o.InboundTag, o.ClientEmail, expiredAt.Format(time.RFC3339),
	)
	if err := s.Send(ctx, to, subject, body, "client_expired", o.ID); err != nil {
		s.log.Warn("messages: client.expired send failed",
			slog.Int64("ownership_id", o.ID), slog.String("err", err.Error()))
	}
}

func (s *Service) onClientExpiringSoon(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.ClientExpiringSoon)
	if !ok {
		s.log.Warn("messages: client.expiring_soon with unknown payload type",
			"type", fmt.Sprintf("%T", e.Data))
		return
	}
	to := s.userEmail(ctx, p.UserID)
	if to == "" {
		return
	}
	subject := "您的服务即将到期"
	body := fmt.Sprintf(
		"您订阅的客户端将在 %d 天后到期，请提前续费。\n\n"+
			"节点：#%d\n入站：%s\n客户端：%s\n到期时间：%s\n",
		p.DaysRemaining, p.NodeID, p.InboundTag, p.ClientEmail, p.ExpiresAt.Format(time.RFC3339),
	)
	if err := s.Send(ctx, to, subject, body, "client_expiring_soon", p.OwnershipID); err != nil {
		s.log.Warn("messages: client.expiring_soon send failed",
			slog.Int64("ownership_id", p.OwnershipID), slog.String("err", err.Error()))
	}
}

func (s *Service) onClientOverLimit(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.ClientThreshold)
	if !ok {
		s.log.Warn("messages: client.over_limit with unknown payload type",
			"type", fmt.Sprintf("%T", e.Data))
		return
	}
	o, err := s.resolveOwnership(ctx, p.NodeID, p.InboundTag, p.ClientEmail)
	if err != nil || o == nil {
		s.log.Warn("messages: client.over_limit without resolvable ownership",
			"node_id", p.NodeID, "client_email", p.ClientEmail, "err", err)
		return
	}
	to := s.userEmail(ctx, o.UserID)
	if to == "" {
		return
	}
	subject := "流量已用尽"
	body := fmt.Sprintf(
		"您订阅的客户端流量已用尽，连接将受限。\n\n"+
			"节点：%s (#%d)\n入站：%s\n客户端：%s\n上行 / 下行：%d / %d 字节\n上限：%d 字节\n",
		p.NodeName, p.NodeID, p.InboundTag, p.ClientEmail, p.Up, p.Down, p.Limit,
	)
	if err := s.Send(ctx, to, subject, body, "client_over_limit", o.ID); err != nil {
		s.log.Warn("messages: client.over_limit send failed",
			slog.Int64("ownership_id", o.ID), slog.String("err", err.Error()))
	}
}

// ---- helpers --------------------------------------------------------------

// resolveOwnership looks up an ownership row from a
// (nodeID, inboundTag, clientEmail) triple. Returns (nil, nil)
// when the row doesn't exist — caller logs and drops.
func (s *Service) resolveOwnership(ctx context.Context, nodeID int64, inboundTag, clientEmail string) (*model.ClientOwnership, error) {
	if s.ownership == nil {
		return nil, fmt.Errorf("messages: ownership repo not wired")
	}
	o, err := s.ownership.GetByTriple(ctx, nodeID, inboundTag, clientEmail)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return o, nil
}

// userEmail loads the owning user and returns their email. Empty
// string means "skip the send" — either the user has no email
// bound or the lookup failed. Both are warned.
func (s *Service) userEmail(ctx context.Context, userID int64) string {
	if s.users == nil {
		return ""
	}
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		s.log.Warn("messages: user lookup failed",
			slog.Int64("user_id", userID), slog.String("err", err.Error()))
		return ""
	}
	if u == nil || u.Email == nil || *u.Email == "" {
		return ""
	}
	return *u.Email
}
