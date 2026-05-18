package event

import (
	"sync/atomic"
	"testing"
)

func TestPublish_DirectMatch(t *testing.T) {
	b := New()
	var got atomic.Int32
	b.Subscribe(NodeOnline, func(e Event) { got.Add(1) })
	b.PublishType(NodeOnline, nil)
	if got.Load() != 1 {
		t.Errorf("direct match: got %d, want 1", got.Load())
	}
	b.PublishType(NodeOffline, nil)
	if got.Load() != 1 {
		t.Errorf("non-match should not fire: got %d", got.Load())
	}
}

func TestPublish_WildcardSuffix(t *testing.T) {
	b := New()
	var got atomic.Int32
	b.Subscribe("node.*", func(e Event) { got.Add(1) })
	b.PublishType(NodeOnline, nil)
	b.PublishType(NodeOffline, nil)
	b.PublishType(OrderCreated, nil)
	if got.Load() != 2 {
		t.Errorf("wildcard: got %d, want 2", got.Load())
	}
}

func TestPublish_StarMatchesEverything(t *testing.T) {
	b := New()
	var got atomic.Int32
	b.Subscribe("*", func(e Event) { got.Add(1) })
	b.PublishType("foo", nil)
	b.PublishType("bar.baz", nil)
	if got.Load() != 2 {
		t.Errorf("star: got %d, want 2", got.Load())
	}
}

func TestPublish_MultipleSubscribersAllFire(t *testing.T) {
	b := New()
	var a, c atomic.Int32
	b.Subscribe(NodeOnline, func(e Event) { a.Add(1) })
	b.Subscribe(NodeOnline, func(e Event) { c.Add(1) })
	b.PublishType(NodeOnline, nil)
	if a.Load() != 1 || c.Load() != 1 {
		t.Errorf("each subscriber should fire once: a=%d c=%d", a.Load(), c.Load())
	}
}

func TestPublish_SetsTimeIfZero(t *testing.T) {
	b := New()
	var seen Event
	b.Subscribe(NodeOnline, func(e Event) { seen = e })
	b.PublishType(NodeOnline, nil)
	if seen.Time.IsZero() {
		t.Error("Time should have been set automatically")
	}
}
