// Package event provides a tiny in-process pub/sub bus used by
// domain services to announce things that happen (node.online,
// order.completed, user.registered, …). Subscribers receive every
// matching event synchronously; long-running or networked work
// (e.g. webhook delivery) MUST be enqueued by the subscriber, not
// performed inline.
//
// The bus is intentionally synchronous and lock-protected — there
// is one Bus per process and event volume is low (handful per
// second at peak). Group 12 wires webhooks to the bus.
package event

import (
	"sync"
	"time"
)

// Event is what every Publish carries. Type is a dotted name like
// "node.online" / "order.completed"; Data is the per-type payload
// (any concrete struct, decoded by the subscriber).
type Event struct {
	Type string    `json:"type"`
	Time time.Time `json:"time"`
	Data any       `json:"data"`
}

// Well-known event types. Domain packages should reference these
// constants instead of bare strings so renames stay tractable.
const (
	NodeOnline       = "node.online"
	NodeOffline      = "node.offline"
	NodeProbeFailed  = "node.probe_failed"
	UserRegistered   = "user.registered"
	OrderCreated     = "order.created"
	OrderCompleted   = "order.completed"
	OrderFailed      = "order.failed"
	ClientExpired      = "client.expired"
	ClientExpiringSoon = "client.expiring_soon"
	ClientOverLimit    = "client.over_limit"
)

// Handler is invoked once per matching Publish. Handlers should
// return quickly; the bus is single-threaded across Publish calls.
type Handler func(Event)

// Bus is the singleton dispatcher.
type Bus struct {
	mu          sync.RWMutex
	wildcardSub []Handler
	subs        map[string][]Handler
	clock       func() time.Time
}

// New returns an empty Bus.
func New() *Bus {
	return &Bus{
		subs:  make(map[string][]Handler),
		clock: time.Now,
	}
}

// Subscribe registers fn for the given pattern. Pattern is either an
// exact event type ("node.online"), a wildcard suffix ("node.*"),
// or a bare "*" for everything. Wildcards are evaluated at Publish
// time and don't support more elaborate globbing.
func (b *Bus) Subscribe(pattern string, fn Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if pattern == "*" {
		b.wildcardSub = append(b.wildcardSub, fn)
		return
	}
	b.subs[pattern] = append(b.subs[pattern], fn)
}

// Publish dispatches event to every matching subscriber. event.Time
// is set to bus.clock() if zero.
func (b *Bus) Publish(event Event) {
	if event.Time.IsZero() {
		event.Time = b.clock()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, fn := range b.wildcardSub {
		fn(event)
	}
	if subs, ok := b.subs[event.Type]; ok {
		for _, fn := range subs {
			fn(event)
		}
	}
	// Prefix-wildcard subscribers: keys ending in ".*".
	for pat, subs := range b.subs {
		if pat == event.Type {
			continue
		}
		if !endsWith(pat, ".*") {
			continue
		}
		if hasPrefix(event.Type, pat[:len(pat)-1]) {
			for _, fn := range subs {
				fn(event)
			}
		}
	}
}

// PublishType is a tiny convenience for the common path.
func (b *Bus) PublishType(eventType string, data any) {
	b.Publish(Event{Type: eventType, Data: data})
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
