// Package notify bridges domain events to user-facing channels.
//
// Two delivery shapes coexist:
//
//  1. **Per-user lifecycle events** — client.expired / expiring_soon
//     / over_limit. We resolve the owning user's email, build a
//     Message with Recipient = user.email, and dispatch through the
//     channels routed for the event type. The email channel uses
//     Recipient; webhook-style channels (telegram/discord/feishu)
//     ignore Recipient and send to their configured admin target.
//
//  2. **Ops events** — node.offline / node.recovered / order.* —
//     no per-user recipient. Webhook channels send to admin targets;
//     the email channel falls back to its `opsRecipient` config.
//
// Persistent dedup: `notification_log` per (kind, ownership_id OR
// dedup_key). The kind suffix is per-channel (e.g. expiring_soon_telegram
// vs expiring_soon_email) so a redelivery to one channel doesn't
// block the other.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
)

// NotificationLogStore is the dedup gate the service uses. Defined
// locally as an interface so unit tests can stub it without standing
// up a Postgres instance — the production impl is
// repository.NotificationLogRepo.
//
// `surface` is model.SurfaceMessage or model.SurfaceNotification —
// dedup is per-surface so a "low_balance" sent to the user
// (message) and a "low_balance" sent to ops (notification) don't
// block each other.
type NotificationLogStore interface {
	AlreadySent(ctx context.Context, surface, kind string, ownershipID int64) (bool, error)
	MarkSent(ctx context.Context, surface, kind string, ownershipID int64, userEmail string) error
}

// Service wires the bus subscriber + channels + router + repos.
type Service struct {
	bus       *event.Bus
	router    *Router
	channels  map[string]Channel
	users     *repository.UserRepo
	ownership *repository.ClientOwnershipRepo
	logs      NotificationLogStore
	log       *slog.Logger
}

// New wires the service. `channels` is a flat list — the service
// indexes by Channel.Name(). The router decides which channels see
// each event type. `logs` is the NotificationLogStore interface so
// tests can pass a stub without a real DB.
func New(
	bus *event.Bus,
	router *Router,
	channels []Channel,
	users *repository.UserRepo,
	ownership *repository.ClientOwnershipRepo,
	logs NotificationLogStore,
	lg *slog.Logger,
) *Service {
	idx := make(map[string]Channel, len(channels))
	for _, c := range channels {
		if c == nil {
			continue
		}
		idx[c.Name()] = c
	}
	return &Service{
		bus:       bus,
		router:    router,
		channels:  idx,
		users:     users,
		ownership: ownership,
		logs:      logs,
		log:       lg.With(slog.String("component", "service.notify")),
	}
}

// Start subscribes to every event type the service handles.
// Idempotent in the sense that re-calling registers extra handlers
// — only call once.
func (s *Service) Start() {
	s.bus.Subscribe(event.ClientExpired, s.onExpired)
	s.bus.Subscribe(event.ClientExpiringSoon, s.onExpiringSoon)
	s.bus.Subscribe(event.ClientOverLimit, s.onOverLimit)
	s.bus.Subscribe(event.NodeOffline, s.onNodeOffline)
	s.bus.Subscribe(event.NodeRecovered, s.onNodeRecovered)
	s.bus.Subscribe(event.OrderPaymentConfirmed, s.onOrderPaymentConfirmed)
	s.bus.Subscribe(event.OrderPaymentFailed, s.onOrderPaymentFailed)
	s.bus.Subscribe(event.OrderPaymentExpired, s.onOrderPaymentExpired)
	s.bus.Subscribe(event.OrderFailed, s.onOrderFailed)
}

// ---- per-user client lifecycle handlers ------------------------------------

func (s *Service) onExpired(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var ownership *model.ClientOwnership
	var expiredAt time.Time
	switch p := e.Data.(type) {
	case payload.ClientExpired:
		ownership = &model.ClientOwnership{
			ID: p.OwnershipID, UserID: p.UserID,
			NodeID: p.NodeID, InboundTag: p.InboundTag, ClientEmail: p.ClientEmail,
		}
		expiredAt = p.ExpiredAt
	case payload.TrafficClientExpired:
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

	s.dispatchClientEvent(ctx, event.ClientExpired, "expired", ownership, Message{
		Level: LevelError,
		Title: "您的服务已到期",
		Body: fmt.Sprintf(
			"您订阅的客户端已到期，已自动停用。\n如需继续使用，请登录控制台购买新套餐。",
		),
		Fields: []Field{
			{Key: "节点", Value: fmt.Sprintf("#%d", ownership.NodeID)},
			{Key: "入站", Value: ownership.InboundTag},
			{Key: "客户端", Value: ownership.ClientEmail},
			{Key: "到期时间", Value: expiredAt.Format(time.RFC3339)},
		},
		EventType:   event.ClientExpired,
		OwnershipID: ownership.ID,
	})
}

func (s *Service) onExpiringSoon(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.ClientExpiringSoon)
	if !ok {
		s.log.Warn("client.expiring_soon with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	ownership := &model.ClientOwnership{
		ID: p.OwnershipID, UserID: p.UserID,
		NodeID: p.NodeID, InboundTag: p.InboundTag, ClientEmail: p.ClientEmail,
	}
	s.dispatchClientEvent(ctx, event.ClientExpiringSoon, "expiring_soon", ownership, Message{
		Level: LevelWarn,
		Title: "您的服务即将到期",
		Body:  fmt.Sprintf("您订阅的客户端将在 %d 天后到期，请提前续费。", p.DaysRemaining),
		Fields: []Field{
			{Key: "节点", Value: fmt.Sprintf("#%d", ownership.NodeID)},
			{Key: "入站", Value: ownership.InboundTag},
			{Key: "客户端", Value: ownership.ClientEmail},
			{Key: "到期时间", Value: p.ExpiresAt.Format(time.RFC3339)},
		},
		EventType:   event.ClientExpiringSoon,
		OwnershipID: ownership.ID,
	})
}

func (s *Service) onOverLimit(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.ClientThreshold)
	if !ok {
		s.log.Warn("client.over_limit with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	o, err := s.lookupOwnership(ctx, p.NodeID, p.InboundTag, p.ClientEmail)
	if err != nil || o == nil {
		s.log.Warn("client.over_limit without resolvable ownership", "node_id", p.NodeID, "client_email", p.ClientEmail, "err", err)
		return
	}
	s.dispatchClientEvent(ctx, event.ClientOverLimit, "over_limit", o, Message{
		Level: LevelError,
		Title: "流量已用尽",
		Body:  "您订阅的客户端流量已用尽，连接将受限。",
		Fields: []Field{
			{Key: "节点", Value: fmt.Sprintf("%s (#%d)", p.NodeName, p.NodeID)},
			{Key: "入站", Value: p.InboundTag},
			{Key: "客户端", Value: p.ClientEmail},
			{Key: "上行 / 下行", Value: fmt.Sprintf("%d / %d 字节", p.Up, p.Down)},
			{Key: "上限", Value: fmt.Sprintf("%d 字节", p.Limit)},
		},
		EventType:   event.ClientOverLimit,
		OwnershipID: o.ID,
	})
}

// dispatchClientEvent resolves the user email, then walks the
// routed channels. Email channel gets Recipient = user.email; other
// channels ignore Recipient and use their configured target.
//
// Dedup is per-channel: the kind suffix uses the channel name so a
// failed telegram send doesn't block the email send.
func (s *Service) dispatchClientEvent(ctx context.Context, eventType, baseKind string, o *model.ClientOwnership, msg Message) {
	if o == nil || o.UserID == 0 || o.ID == 0 {
		s.log.Warn("notify.dispatch missing ownership identity", "event", eventType)
		return
	}
	user, err := s.users.Get(ctx, o.UserID)
	if err != nil {
		s.log.Error("user lookup", "user_id", o.UserID, "err", err)
		return
	}
	recipient := ""
	if user != nil && user.Email != nil {
		recipient = *user.Email
	}
	msg.Recipient = recipient

	for _, chanName := range s.router.Channels(eventType) {
		ch, ok := s.channels[chanName]
		if !ok || !ch.Enabled() {
			continue
		}
		// Email needs a recipient — if user has no email, skip the
		// email channel entirely but still log dedup so we don't
		// re-check every tick.
		if chanName == "email" && recipient == "" {
			_ = s.logs.MarkSent(ctx, model.SurfaceNotification, baseKind+"_"+chanName, o.ID, "")
			continue
		}
		s.dispatchOnce(ctx, ch, baseKind+"_"+chanName, o.ID, recipient, msg)
	}
}

// dispatchOnce runs the dedup check + send + mark for one channel.
// kind is the channel-specific log key suffix. All notify.Service
// dispatches book under model.SurfaceNotification — the ops-facing
// surface. User-facing messages go through service/messages with
// model.SurfaceMessage.
func (s *Service) dispatchOnce(ctx context.Context, ch Channel, kind string, dedupID int64, recipient string, msg Message) {
	already, err := s.logs.AlreadySent(ctx, model.SurfaceNotification, kind, dedupID)
	if err != nil {
		s.log.Error("dedup check failed", "kind", kind, "err", err)
		// Fall through — better to risk a dup than miss a critical notice
	}
	if already {
		return
	}
	if err := ch.Send(ctx, msg); err != nil {
		s.log.Error("channel send failed",
			"channel", ch.Name(), "kind", kind, "err", err)
		return
	}
	if err := s.logs.MarkSent(ctx, model.SurfaceNotification, kind, dedupID, recipient); err != nil {
		s.log.Warn("MarkSent failed (delivery already done)", "kind", kind, "err", err)
	}
	s.log.Info("notify delivered", "channel", ch.Name(), "kind", kind, "to", recipient)
}

// ---- ops event handlers ----------------------------------------------------

func (s *Service) onNodeOffline(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.NodeStatusChanged)
	if !ok {
		s.log.Warn("node.offline with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	s.dispatchOpsEvent(ctx, event.NodeOffline, fmt.Sprintf("node_offline_%d_%d", p.NodeID, e.Time.Unix()), Message{
		Level: LevelError,
		Title: fmt.Sprintf("节点离线：%s", nonEmpty(p.Name, fmt.Sprintf("#%d", p.NodeID))),
		Body:  "节点连续两次探测失败，已标记为 offline。",
		Fields: []Field{
			{Key: "Node ID", Value: strconv.FormatInt(p.NodeID, 10)},
			{Key: "Time", Value: e.Time.Format(time.RFC3339)},
		},
		EventType: event.NodeOffline,
		DedupKey:  fmt.Sprintf("node_offline_%d_%d", p.NodeID, e.Time.Unix()),
	})
}

func (s *Service) onNodeRecovered(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.NodeStatusChanged)
	if !ok {
		s.log.Warn("node.recovered with unknown payload type", "type", fmt.Sprintf("%T", e.Data))
		return
	}
	s.dispatchOpsEvent(ctx, event.NodeRecovered, fmt.Sprintf("node_recovered_%d_%d", p.NodeID, e.Time.Unix()), Message{
		Level: LevelInfo,
		Title: fmt.Sprintf("节点恢复：%s", nonEmpty(p.Name, fmt.Sprintf("#%d", p.NodeID))),
		Body:  "节点已恢复在线状态。",
		Fields: []Field{
			{Key: "Node ID", Value: strconv.FormatInt(p.NodeID, 10)},
			{Key: "Time", Value: e.Time.Format(time.RFC3339)},
		},
		EventType: event.NodeRecovered,
		DedupKey:  fmt.Sprintf("node_recovered_%d_%d", p.NodeID, e.Time.Unix()),
	})
}

func (s *Service) onOrderPaymentConfirmed(e event.Event) {
	s.opsOrderEvent(e, event.OrderPaymentConfirmed, LevelInfo, "订单已支付")
}
func (s *Service) onOrderPaymentFailed(e event.Event) {
	s.opsOrderEvent(e, event.OrderPaymentFailed, LevelWarn, "订单支付失败")
}
func (s *Service) onOrderPaymentExpired(e event.Event) {
	s.opsOrderEvent(e, event.OrderPaymentExpired, LevelWarn, "订单支付超时")
}
func (s *Service) onOrderFailed(e event.Event) {
	s.opsOrderEvent(e, event.OrderFailed, LevelError, "订单失败")
}

func (s *Service) opsOrderEvent(e event.Event, eventType string, lvl Level, title string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, ok := e.Data.(payload.Order)
	if !ok {
		s.log.Warn("order event with unknown payload type",
			"event", eventType, "type", fmt.Sprintf("%T", e.Data))
		return
	}
	msg := Message{
		Level: lvl,
		Title: title,
		Fields: []Field{
			{Key: "Order ID", Value: strconv.FormatInt(p.OrderID, 10)},
			{Key: "User ID", Value: strconv.FormatInt(p.UserID, 10)},
			{Key: "Plan ID", Value: strconv.FormatInt(p.PlanID, 10)},
			{Key: "Amount", Value: fmt.Sprintf("%.2f", float64(p.PriceCents)/100)},
		},
		EventType: eventType,
		DedupKey:  fmt.Sprintf("%s_%d", eventType, p.OrderID),
	}
	if p.Reason != "" {
		msg.Fields = append(msg.Fields, Field{Key: "Reason", Value: p.Reason})
	}
	s.dispatchOpsEvent(ctx, eventType, msg.DedupKey, msg)
}

// dispatchOpsEvent fans out to webhook-style channels (telegram /
// discord / feishu). Email also runs IF email is routed — in that
// case it sends to the channel's `opsRecipient` (since msg.Recipient
// stays empty for ops events).
//
// Ops dedup uses DedupKey (string) instead of OwnershipID. We hash
// the key onto a stable int64 so the existing notification_log table
// (typed BIGINT) still works without a schema change.
func (s *Service) dispatchOpsEvent(ctx context.Context, eventType, dedupKey string, msg Message) {
	dedupID := hashDedupKey(dedupKey)
	for _, chanName := range s.router.Channels(eventType) {
		ch, ok := s.channels[chanName]
		if !ok || !ch.Enabled() {
			continue
		}
		kind := strings.ReplaceAll(eventType, ".", "_") + "_" + chanName
		s.dispatchOnce(ctx, ch, kind, dedupID, "", msg)
	}
}

// ---- helpers --------------------------------------------------------------

// hashDedupKey turns a string dedup key into a stable int64 for the
// notification_log.ownership_id column. FNV-1a 64-bit — no
// collisions in practice for the per-event ID space we generate.
func hashDedupKey(s string) int64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	// notification_log uses BIGINT; cast may produce negatives but
	// that's fine — we're using it as an opaque ID.
	return int64(h)
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

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
