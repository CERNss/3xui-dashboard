package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newEngine() *gin.Engine {
	reset()
	e := gin.New()
	e.Use(Middleware())
	e.GET("/api/users/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.GET("/metrics", Handler())
	return e
}

func TestMiddleware_RecordsCounter(t *testing.T) {
	e := newEngine()
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("handler returned %d", w.Code)
		}
	}
	// Scrape /metrics
	scrape := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	sw := httptest.NewRecorder()
	e.ServeHTTP(sw, scrape)
	body := sw.Body.String()

	// Route template substituted, NOT the raw URL with id=42.
	if !strings.Contains(body, `path="/api/users/:id"`) {
		t.Errorf("expected route template /api/users/:id in metrics body, got:\n%s", body)
	}
	if strings.Contains(body, `path="/api/users/42"`) {
		t.Errorf("raw URL with id leaked into label — high-cardinality risk")
	}
	// Counter value should be 3 for that line.
	if !strings.Contains(body, `http_requests_total{method="GET",path="/api/users/:id",status="200"} 3`) {
		t.Errorf("expected counter value 3, got:\n%s", body)
	}
	// Duration histogram should also be present.
	if !strings.Contains(body, "http_request_duration_seconds_bucket") {
		t.Errorf("duration histogram missing")
	}
}

func TestMiddleware_UnmatchedRouteBucket(t *testing.T) {
	e := newEngine()
	req := httptest.NewRequest(http.MethodGet, "/wp-login.php", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	// gin returns 404 for unmatched
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	scrape := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	sw := httptest.NewRecorder()
	e.ServeHTTP(sw, scrape)
	body := sw.Body.String()

	// Bot scans collapse into a single "unmatched" bucket — anything
	// else would let an attacker explode cardinality by spraying URLs.
	if !strings.Contains(body, `path="unmatched"`) {
		t.Errorf(`expected path="unmatched" label, got:\n%s`, body)
	}
	if strings.Contains(body, `path="/wp-login.php"`) {
		t.Errorf("raw 404 URL leaked into label")
	}
}

func TestHandler_ServesMetrics(t *testing.T) {
	e := newEngine()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "# HELP") {
		t.Errorf("response missing prometheus HELP lines")
	}
}
