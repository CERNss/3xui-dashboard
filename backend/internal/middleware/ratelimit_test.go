package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newLimitEngine(burst, refill int, window time.Duration) *gin.Engine {
	e := gin.New()
	e.Use(LoginRateLimiter(burst, refill, window))
	e.POST("/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return e
}

func TestLoginRateLimiter_AllowsBurstThenBlocks(t *testing.T) {
	e := newLimitEngine(3, 3, time.Minute)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("burst[%d] status = %d, want 200", i, w.Code)
		}
	}
	// 4th request blows the bucket.
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4:5555"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Errorf("Retry-After missing on 429")
	}
}

func TestLoginRateLimiter_IsPerIP(t *testing.T) {
	e := newLimitEngine(2, 2, time.Minute)

	// IP1 hits the limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if i < 2 && w.Code != http.StatusOK {
			t.Errorf("IP1 attempt %d should pass, got %d", i, w.Code)
		}
		if i == 2 && w.Code != http.StatusTooManyRequests {
			t.Errorf("IP1 attempt %d should 429, got %d", i, w.Code)
		}
	}

	// IP2 still has full burst — limit is per-IP.
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.2:2222"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("IP2 should be unaffected by IP1, got %d", w.Code)
	}
}

func TestLoginRateLimiter_RefillsOverTime(t *testing.T) {
	// 2 tokens / 100ms — easy to exercise in a unit test.
	e := newLimitEngine(2, 2, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.0.2.10:1111"
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if i < 2 && w.Code != http.StatusOK {
			t.Errorf("burst attempt %d = %d, want 200", i, w.Code)
		}
	}
	// Wait past the full refill window — bucket should be full again.
	time.Sleep(150 * time.Millisecond)
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.0.2.10:1111"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("post-refill status = %d, want 200", w.Code)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "1"},   // clamped
		{-5, "1"},  // clamped
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{42, "42"},
		{1000, "1000"},
	}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
