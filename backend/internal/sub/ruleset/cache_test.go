package ruleset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestCache_Inline_NoFetch(t *testing.T) {
	c := NewCache(nil)
	rs := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceInline, Content: "DOMAIN-SUFFIX,x.com"}
	got, err := c.Content(context.Background(), rs)
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if got != "DOMAIN-SUFFIX,x.com" {
		t.Errorf("inline content = %q", got)
	}
}

func TestCache_RemoteFetchThenCached(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("payload-v1"))
	}))
	defer srv.Close()

	c := NewCache(srv.Client())
	rs := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceRemote, URL: srv.URL, TTLSeconds: 3600}

	for i := 0; i < 3; i++ {
		got, err := c.Content(context.Background(), rs)
		if err != nil {
			t.Fatalf("Content #%d: %v", i, err)
		}
		if got != "payload-v1" {
			t.Errorf("content = %q, want payload-v1", got)
		}
	}
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Errorf("upstream hit %d times, want 1 (cached within TTL)", h)
	}
}

func TestCache_RefetchAfterTTL(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	now := time.Unix(1_700_000_000, 0)
	c := NewCache(srv.Client())
	c.now = func() time.Time { return now }
	rs := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceRemote, URL: srv.URL, TTLSeconds: 60}

	if _, err := c.Content(context.Background(), rs); err != nil {
		t.Fatalf("first: %v", err)
	}
	now = now.Add(2 * time.Minute) // past the 60s TTL
	if _, err := c.Content(context.Background(), rs); err != nil {
		t.Fatalf("second: %v", err)
	}
	if h := atomic.LoadInt32(&hits); h != 2 {
		t.Errorf("upstream hit %d times, want 2 (TTL expired)", h)
	}
}

func TestCache_ServesStaleOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("good"))
	}))
	c := NewCache(srv.Client())
	c.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	rs := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceRemote, URL: srv.URL, TTLSeconds: 1}

	if _, err := c.Content(context.Background(), rs); err != nil {
		t.Fatalf("warm cache: %v", err)
	}
	srv.Close() // upstream now unreachable

	// TTL expired (now advances) so it tries to refetch, fails, and
	// should serve the stale cached body instead of erroring.
	c.now = func() time.Time { return time.Unix(1_700_001_000, 0) }
	got, err := c.Content(context.Background(), rs)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if got != "good" {
		t.Errorf("stale content = %q, want good", got)
	}
}

func TestCache_RemoteNoURL(t *testing.T) {
	c := NewCache(nil)
	rs := model.SubscriptionRuleset{Key: "k", SourceType: model.RulesetSourceRemote}
	if _, err := c.Content(context.Background(), rs); err == nil {
		t.Fatal("expected error for remote ruleset with no URL")
	}
}
