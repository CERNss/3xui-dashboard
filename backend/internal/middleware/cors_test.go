package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newCORSEngine(t *testing.T, allowed []string) *gin.Engine {
	t.Helper()
	e := gin.New()
	e.Use(CORS(allowed))
	e.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	e.POST("/api/echo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return e
}

func TestCORS_Permissive_EchoesOriginNoCreds(t *testing.T) {
	e := newCORSEngine(t, nil) // empty list = permissive
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("Origin", "https://random.example.com")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://random.example.com" {
		t.Errorf("Allow-Origin = %q, want echo", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q, want empty in permissive mode", got)
	}
}

func TestCORS_ExactMatch_AllowsKnownOriginWithCreds(t *testing.T) {
	e := newCORSEngine(t, []string{"https://panel.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("Origin", "https://panel.example.com")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://panel.example.com" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

func TestCORS_ExactMatch_RejectsUnknownOrigin(t *testing.T) {
	e := newCORSEngine(t, []string{"https://panel.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	// Request still succeeds — the browser, not the server, enforces
	// CORS. We just don't emit the allow header.
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty for unknown origin, got %q", got)
	}
}

func TestCORS_Preflight_Returns204(t *testing.T) {
	e := newCORSEngine(t, []string{"https://panel.example.com"})
	req := httptest.NewRequest(http.MethodOptions, "/api/echo", nil)
	req.Header.Set("Origin", "https://panel.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("Allow-Methods missing on preflight")
	}
}

func TestCORS_WildcardInList(t *testing.T) {
	e := newCORSEngine(t, []string{"*", "https://panel.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://anything.example.com" {
		t.Errorf("wildcard mode should echo origin, got %q", got)
	}
}

func TestCORS_NoOriginHeader_Passthrough(t *testing.T) {
	e := newCORSEngine(t, []string{"https://panel.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (same-origin request, no CORS headers expected)", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty for same-origin request, got %q", got)
	}
}
