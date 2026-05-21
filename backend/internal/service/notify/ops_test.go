package notify

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/event/payload"
)

// stubLogStore is a memory-backed NotificationLogStore. Records each
// MarkSent so tests can assert dedup keys.
type stubLogStore struct {
	mu    sync.Mutex
	sent  map[string]bool // "kind:ownershipID" → true
	calls []logCall
}

type logCall struct {
	kind        string
	ownershipID int64
}

func newStubLogStore() *stubLogStore {
	return &stubLogStore{sent: map[string]bool{}}
}

func (s *stubLogStore) AlreadySent(_ context.Context, surface, kind string, ownershipID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent[s.key(surface, kind, ownershipID)], nil
}

func (s *stubLogStore) MarkSent(_ context.Context, surface, kind string, ownershipID int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent[s.key(surface, kind, ownershipID)] = true
	s.calls = append(s.calls, logCall{kind, ownershipID})
	return nil
}

func (s *stubLogStore) key(surface, kind string, ownershipID int64) string {
	return surface + ":" + kind + ":" + boxInt(ownershipID)
}

func boxInt(n int64) string {
	const digits = "0123456789-"
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [22]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// newOpsService builds a Service with no DB-backed repos — just
// stubs. ops dispatch never touches users/ownership (those are
// per-user lifecycle paths), so passing nil is safe AS LONG AS the
// test doesn't publish a client.* event.
func newOpsService(channels []Channel, routerRaw string) (*Service, *stubLogStore) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	bus := event.New()
	router, _ := ParseRoutes(routerRaw)
	logs := newStubLogStore()
	svc := New(bus, router, channels, nil, nil, logs, logger)
	svc.Start()
	return svc, logs
}

func TestOpsDispatch_NodeOffline_RoutesToTelegram(t *testing.T) {
	telegram := newStubChannel("telegram")
	discord := newStubChannel("discord")
	svc, _ := newOpsService(
		[]Channel{telegram, discord},
		"node.offline:telegram",
	)
	svc.bus.PublishType(event.NodeOffline, payload.NodeStatusChanged{
		NodeID: 42, Name: "tokyo-1",
		Prior: "online", Now: "offline",
	})
	if got := telegram.Count(); got != 1 {
		t.Errorf("telegram count = %d, want 1", got)
	}
	if got := discord.Count(); got != 0 {
		t.Errorf("discord NOT routed should be 0, got %d", got)
	}
	// Check the message shape
	telegram.mu.Lock()
	msg := telegram.sends[0]
	telegram.mu.Unlock()
	if msg.Level != LevelError {
		t.Errorf("level = %v, want error", msg.Level)
	}
	if msg.Title == "" || !contains(msg.Title, "tokyo-1") {
		t.Errorf("title missing node name: %q", msg.Title)
	}
}

func TestOpsDispatch_FanOut(t *testing.T) {
	telegram := newStubChannel("telegram")
	discord := newStubChannel("discord")
	feishu := newStubChannel("feishu")
	svc, _ := newOpsService(
		[]Channel{telegram, discord, feishu},
		"node.offline:telegram,discord,feishu",
	)
	svc.bus.PublishType(event.NodeOffline, payload.NodeStatusChanged{
		NodeID: 1, Name: "n1",
	})
	if telegram.Count() != 1 || discord.Count() != 1 || feishu.Count() != 1 {
		t.Errorf("fan-out failed: tg=%d dc=%d fs=%d", telegram.Count(), discord.Count(), feishu.Count())
	}
}

func TestOpsDispatch_DedupPerChannel(t *testing.T) {
	tg := newStubChannel("telegram")
	svc, logs := newOpsService([]Channel{tg}, "node.offline:telegram")
	p := payload.NodeStatusChanged{NodeID: 42, Name: "x"}
	// Same event published twice — same time component in dedup key
	// (we use e.Time.Unix() in the key, and bus stamps each Publish
	// with the bus clock); to test true dedup we share the
	// (event_type, dedup_key) pair via a single bus event.
	svc.bus.PublishType(event.NodeOffline, p)
	// Manually call MarkSent for the same key would prove dedup;
	// publishing again actually produces a NEW dedup key because
	// e.Time advances. That's intentional — node.offline alerts
	// SHOULD re-fire if the node is still down on a later tick.
	// Verify a single Publish only writes one log row.
	if got := tg.Count(); got != 1 {
		t.Errorf("first publish should send: %d", got)
	}
	if got := len(logs.calls); got != 1 {
		t.Errorf("log rows = %d, want 1", got)
	}
}

func TestOpsDispatch_OrderPaymentConfirmed(t *testing.T) {
	tg := newStubChannel("telegram")
	svc, _ := newOpsService([]Channel{tg}, "order.payment_confirmed:telegram")
	svc.bus.PublishType(event.OrderPaymentConfirmed, payload.Order{
		OrderID: 42, UserID: 7, PlanID: 3, PriceCents: 500,
	})
	if tg.Count() != 1 {
		t.Fatalf("count = %d, want 1", tg.Count())
	}
	tg.mu.Lock()
	msg := tg.sends[0]
	tg.mu.Unlock()
	// Check fields are populated from typed payload (not reflection)
	var hasOrderID, hasAmount bool
	for _, f := range msg.Fields {
		if f.Key == "Order ID" && f.Value == "42" {
			hasOrderID = true
		}
		if f.Key == "Amount" && f.Value == "5.00" {
			hasAmount = true
		}
	}
	if !hasOrderID {
		t.Error("Order ID field missing or wrong")
	}
	if !hasAmount {
		t.Error("Amount field missing or wrong format (want 5.00 for 500 cents)")
	}
}

func TestOpsDispatch_OrderFailed_IncludesReason(t *testing.T) {
	tg := newStubChannel("telegram")
	svc, _ := newOpsService([]Channel{tg}, "order.failed:telegram")
	svc.bus.PublishType(event.OrderFailed, payload.Order{
		OrderID: 42, UserID: 7, PlanID: 3, PriceCents: 500,
		Reason: "provisioning_failed",
	})
	tg.mu.Lock()
	msg := tg.sends[0]
	tg.mu.Unlock()
	var foundReason bool
	for _, f := range msg.Fields {
		if f.Key == "Reason" && f.Value == "provisioning_failed" {
			foundReason = true
		}
	}
	if !foundReason {
		t.Error("Reason field missing or wrong")
	}
}

func TestOpsDispatch_NoRoute_NoSend(t *testing.T) {
	tg := newStubChannel("telegram")
	// Route says order.completed → telegram. Publishing node.offline
	// should NOT reach telegram because no rule matches.
	svc, _ := newOpsService([]Channel{tg}, "order.completed:telegram")
	svc.bus.PublishType(event.NodeOffline, payload.NodeStatusChanged{NodeID: 1})
	if got := tg.Count(); got != 0 {
		t.Errorf("count = %d, want 0 (no route)", got)
	}
}

func TestOpsDispatch_ChannelDisabled_Skipped(t *testing.T) {
	tg := newStubChannel("telegram")
	tg.enabled = false
	svc, _ := newOpsService([]Channel{tg}, "node.offline:telegram")
	svc.bus.PublishType(event.NodeOffline, payload.NodeStatusChanged{NodeID: 1})
	if got := tg.Count(); got != 0 {
		t.Errorf("disabled channel should NOT receive: %d", got)
	}
}

func TestOpsDispatch_BadPayloadTypeWarned(t *testing.T) {
	tg := newStubChannel("telegram")
	svc, _ := newOpsService([]Channel{tg}, "node.offline:telegram")
	// Publish with wrong payload type — should be ignored, not panic.
	svc.bus.PublishType(event.NodeOffline, "not-a-struct")
	if got := tg.Count(); got != 0 {
		t.Errorf("bad payload should result in 0 sends, got %d", got)
	}
}

func TestOpsDispatch_NodeRecovered_RoutedSeparately(t *testing.T) {
	tg := newStubChannel("telegram")
	svc, _ := newOpsService([]Channel{tg}, "node.recovered:telegram")
	svc.bus.PublishType(event.NodeRecovered, payload.NodeStatusChanged{
		NodeID: 1, Name: "n1", Prior: "offline", Now: "online",
	})
	if tg.Count() != 1 {
		t.Errorf("recovered count = %d, want 1", tg.Count())
	}
	tg.mu.Lock()
	msg := tg.sends[0]
	tg.mu.Unlock()
	if msg.Level != LevelInfo {
		t.Errorf("recovered level = %v, want info", msg.Level)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
