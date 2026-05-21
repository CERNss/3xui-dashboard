// Package notify is the ops-facing fanout surface. It dispatches
// domain events to admin/ops channels: env-configured email,
// Telegram, Discord, Feishu. The matching user-facing channel
// (SMTP to the user's bound email) lives in service/messages
// and subscribes to the bus independently.
//
// Events handled here:
//   - node.online / node.offline / node.recovered
//   - order.payment_confirmed / order.payment_failed / order.payment_expired / order.failed
//   - client.expired / client.expiring_soon / client.over_limit (the
//     ops copy — the user email for the same events is messages.Service's
//     responsibility, not ours)
//
// Persistent dedup: `notification_log` per (kind, ownership_id OR
// dedup_key) booked under model.SurfaceNotification. The kind suffix
// is per-channel (e.g. node_offline_feishu vs node_offline_telegram)
// so a redelivery to one channel doesn't block the other. Messages
// surface uses model.SurfaceMessage on the same table — the two
// dedup spaces are independent.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
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

// Service wires the bus subscriber + channels + router + dedup log.
// No user / ownership repos: all events handled here target ops,
// not the user — user-side mail lives in service/messages.
type Service struct {
	bus      *event.Bus
	router   *Router
	channels map[string]Channel
	logs     NotificationLogStore
	log      *slog.Logger
}

// New wires the service. `channels` is a flat list — the service
// indexes by Channel.Name(). The router decides which channels see
// each event type. `logs` is the NotificationLogStore interface so
// tests can pass a stub without a real DB.
func New(
	bus *event.Bus,
	router *Router,
	channels []Channel,
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
		bus:      bus,
		router:   router,
		channels: idx,
		logs:     logs,
		log:      lg.With(slog.String("component", "service.notify")),
	}
}

// Start subscribes to every ops event type the service handles.
// Idempotent in the sense that re-calling registers extra handlers
// — only call once. User-facing client lifecycle events
// (client.expired / expiring_soon / over_limit) are NOT subscribed
// here; messages.Service owns that surface. Admins who also want
// these on a Feishu / Telegram channel configure a webhook in
// /admin/webhooks instead — that's the configurable ops fanout.
func (s *Service) Start() {
	s.bus.Subscribe(event.NodeOffline, s.onNodeOffline)
	s.bus.Subscribe(event.NodeRecovered, s.onNodeRecovered)
	s.bus.Subscribe(event.OrderPaymentConfirmed, s.onOrderPaymentConfirmed)
	s.bus.Subscribe(event.OrderPaymentFailed, s.onOrderPaymentFailed)
	s.bus.Subscribe(event.OrderPaymentExpired, s.onOrderPaymentExpired)
	s.bus.Subscribe(event.OrderFailed, s.onOrderFailed)
}

// dispatchOnce runs the dedup check + send + mark for one channel.
// kind is the channel-specific log key suffix. All notify.Service
// dispatches book under model.SurfaceNotification — the ops-facing
// surface. User-facing messages go through service/messages with
// model.SurfaceMessage.
func (s *Service) dispatchOnce(ctx context.Context, ch Channel, kind string, dedupID int64, msg Message) {
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
	if err := s.logs.MarkSent(ctx, model.SurfaceNotification, kind, dedupID, ""); err != nil {
		s.log.Warn("MarkSent failed (delivery already done)", "kind", kind, "err", err)
	}
	s.log.Info("notify delivered", "channel", ch.Name(), "kind", kind)
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

// dispatchOpsEvent fans out to channels routed for the event type
// (email-to-ops, Discord, Feishu, Telegram). Each channel sends to
// its configured ops target — no per-user recipient routing.
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
		s.dispatchOnce(ctx, ch, kind, dedupID, msg)
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

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
