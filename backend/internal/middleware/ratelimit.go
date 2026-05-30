package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// IPRateLimiter returns a per-IP rate-limit middleware. Token
// bucket: `burst` tokens replenish at `refill` over `window`.
// Callers above the rate get HTTP 429 with a Retry-After hint.
//
// Used by:
//   - /login (block password-spray)
//   - /sub/:subId (block sub_id enumeration — leaking sub_id =
//     handing out a user's WG private keys + traffic stats,
//     so the bruteforce surface needs to be slow)
//
// The bucket map is unbounded (one entry per source IP) but each
// entry is small and self-prunes on access — older entries are
// evicted on the next sweep that finds them idle for >2×window.
// For these endpoints that's fine; for a high-fan-in API we'd
// want a proper LRU.
func IPRateLimiter(burst int, refill int, window time.Duration) gin.HandlerFunc {
	if burst <= 0 {
		burst = 10
	}
	if refill <= 0 {
		refill = burst
	}
	if window <= 0 {
		window = time.Minute
	}

	type bucket struct {
		tokens   int
		updated  time.Time
		lastSeen time.Time
	}
	var (
		mu        sync.Mutex
		buckets   = make(map[string]*bucket)
		lastSweep time.Time
	)

	refillTokens := func(b *bucket, now time.Time) {
		elapsed := now.Sub(b.updated)
		if elapsed <= 0 {
			return
		}
		add := int(elapsed.Seconds() * (float64(refill) / window.Seconds()))
		if add > 0 {
			b.tokens += add
			if b.tokens > burst {
				b.tokens = burst
			}
			b.updated = now
		}
	}

	sweep := func(now time.Time) {
		if now.Sub(lastSweep) < window {
			return
		}
		lastSweep = now
		threshold := 2 * window
		for k, b := range buckets {
			if now.Sub(b.lastSeen) > threshold {
				delete(buckets, k)
			}
		}
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			c.Next()
			return
		}
		now := time.Now()

		mu.Lock()
		sweep(now)
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{tokens: burst, updated: now, lastSeen: now}
			buckets[ip] = b
		} else {
			refillTokens(b, now)
		}
		b.lastSeen = now
		if b.tokens <= 0 {
			// Estimate seconds until the next token is available.
			perToken := window.Seconds() / float64(refill)
			retry := int(perToken)
			if retry < 1 {
				retry = 1
			}
			mu.Unlock()
			c.Writer.Header().Set("Retry-After", itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests — slow down and try again shortly",
			})
			return
		}
		b.tokens--
		mu.Unlock()

		c.Next()
	}
}

// UserRateLimiter is the same token-bucket as IPRateLimiter but keyed
// on the authenticated user ID rather than the source IP. Must mount
// AFTER RequireActiveUser / RequireAdmin so the claims are already on
// the gin context — requests without claims fall through unrestricted
// (the auth middleware in front would have already 401'd them).
//
// Used by /api/user/purchase and similar money-spending or
// resource-burning endpoints where a single authenticated user could
// otherwise hammer the dashboard from a single IP.
func UserRateLimiter(burst int, refill int, window time.Duration) gin.HandlerFunc {
	if burst <= 0 {
		burst = 10
	}
	if refill <= 0 {
		refill = burst
	}
	if window <= 0 {
		window = time.Minute
	}

	type bucket struct {
		tokens   int
		updated  time.Time
		lastSeen time.Time
	}
	var (
		mu        sync.Mutex
		buckets   = make(map[string]*bucket)
		lastSweep time.Time
	)

	refillTokens := func(b *bucket, now time.Time) {
		elapsed := now.Sub(b.updated)
		if elapsed <= 0 {
			return
		}
		add := int(elapsed.Seconds() * (float64(refill) / window.Seconds()))
		if add > 0 {
			b.tokens += add
			if b.tokens > burst {
				b.tokens = burst
			}
			b.updated = now
		}
	}

	sweep := func(now time.Time) {
		if now.Sub(lastSweep) < window {
			return
		}
		lastSweep = now
		threshold := 2 * window
		for k, b := range buckets {
			if now.Sub(b.lastSeen) > threshold {
				delete(buckets, k)
			}
		}
	}

	return func(c *gin.Context) {
		claims := Claims(c)
		if claims == nil || claims.Subject == "" {
			c.Next()
			return
		}
		key := claims.Subject
		now := time.Now()

		mu.Lock()
		sweep(now)
		b, ok := buckets[key]
		if !ok {
			b = &bucket{tokens: burst, updated: now, lastSeen: now}
			buckets[key] = b
		} else {
			refillTokens(b, now)
		}
		b.lastSeen = now
		if b.tokens <= 0 {
			perToken := window.Seconds() / float64(refill)
			retry := int(perToken)
			if retry < 1 {
				retry = 1
			}
			mu.Unlock()
			c.Writer.Header().Set("Retry-After", itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests for this account — slow down and try again shortly",
			})
			return
		}
		b.tokens--
		mu.Unlock()

		c.Next()
	}
}

// itoa is a tiny strconv.Itoa replacement so we don't pull strconv
// in for a one-liner. Returns "1" for non-positive input.
func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	buf := make([]byte, 0, 4)
	for n > 0 {
		buf = append([]byte{digits[n%10]}, buf...)
		n /= 10
	}
	return string(buf)
}
