// Package ruleset fetches and caches the rule lists a self-hosted
// subscription profile serves from /sub/ruleset/<key>. Remote rulesets
// are fetched over HTTP and cached in memory for their TTL; inline
// rulesets return their stored content directly.
//
// The set of fetchable URLs is bounded by the admin-configured rulesets
// (a /sub/ruleset/<key> request maps a key to a configured ruleset, not
// to a user-supplied URL), so this is not an open SSRF surface.
package ruleset

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

const (
	defaultMaxBytes = 8 << 20 // 8 MiB cap per ruleset body
	fetchTimeout    = 15 * time.Second
	fallbackTTL     = 24 * time.Hour
)

type entry struct {
	content   string
	fetchedAt time.Time
}

// Cache is a concurrency-safe in-memory ruleset body cache.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]entry
	client   *http.Client
	maxBytes int64
	now      func() time.Time
}

// NewCache returns a Cache. A nil client gets a default with a fetch
// timeout.
func NewCache(client *http.Client) *Cache {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &Cache{
		entries:  make(map[string]entry),
		client:   client,
		maxBytes: defaultMaxBytes,
		now:      time.Now,
	}
}

// Content returns the ruleset's usable body. Inline rulesets return
// their stored content. Remote rulesets return the cached body while
// within TTL, otherwise a fresh fetch — falling back to a stale cached
// body if the refresh fails (availability over freshness).
func (c *Cache) Content(ctx context.Context, rs model.SubscriptionRuleset) (string, error) {
	if rs.SourceType == model.RulesetSourceInline {
		return rs.Content, nil
	}
	if rs.URL == "" {
		return "", fmt.Errorf("ruleset %q: remote source has no URL", rs.Key)
	}

	ttl := time.Duration(rs.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = fallbackTTL
	}
	if e, ok := c.get(rs.Key); ok && c.now().Sub(e.fetchedAt) < ttl {
		return e.content, nil
	}

	body, err := c.fetch(ctx, rs.URL)
	if err != nil {
		if e, ok := c.get(rs.Key); ok {
			return e.content, nil // serve stale rather than fail
		}
		return "", err
	}
	c.put(rs.Key, body)
	return body, nil
}

func (c *Cache) fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ruleset fetch %s: unexpected status %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes))
	if err != nil {
		return "", fmt.Errorf("ruleset fetch %s: read body: %w", url, err)
	}
	return string(b), nil
}

func (c *Cache) get(key string) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
}

func (c *Cache) put(key, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{content: content, fetchedAt: c.now()}
}
