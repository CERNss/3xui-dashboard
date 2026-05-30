package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/session"
)

func newCSRFRouter() *gin.Engine {
	sess := session.NewManager(false, time.Hour)
	r := gin.New()
	g := r.Group("/api", CSRF(sess))
	g.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.POST("/do", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCSRF_SafeMethodIssuesCookie(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/ping", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET: got %d", w.Code)
	}
	if c := csrfFindCookie(w.Result().Cookies(), session.CSRFCookie); c == nil || c.Value == "" {
		t.Fatal("safe method should lazily issue a CSRF cookie")
	}
}

func TestCSRF_SafeMethodKeepsExistingCookie(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/ping", nil)
	req.AddCookie(&http.Cookie{Name: session.CSRFCookie, Value: "existing"})
	r.ServeHTTP(w, req)
	if csrfFindCookie(w.Result().Cookies(), session.CSRFCookie) != nil {
		t.Fatal("safe method with an existing CSRF cookie should not re-issue")
	}
}

func TestCSRF_UnsafeWithoutToken_403(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/do", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF token: got %d, want 403", w.Code)
	}
}

func TestCSRF_UnsafeMatchingToken_OK(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/do", nil)
	req.AddCookie(&http.Cookie{Name: session.CSRFCookie, Value: "tok-abc"})
	req.Header.Set(session.CSRFHeader, "tok-abc")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST with matching CSRF cookie+header: got %d, want 200", w.Code)
	}
}

func TestCSRF_UnsafeMismatch_403(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/do", nil)
	req.AddCookie(&http.Cookie{Name: session.CSRFCookie, Value: "tok-abc"})
	req.Header.Set(session.CSRFHeader, "tok-different")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST with mismatched CSRF: got %d, want 403", w.Code)
	}
}

func TestCSRF_UnsafeHeaderOnlyNoCookie_403(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/do", nil)
	req.Header.Set(session.CSRFHeader, "tok-abc") // header but no cookie to compare
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST with header but no cookie: got %d, want 403", w.Code)
	}
}

func TestCSRF_BearerExempt_OK(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/do", nil)
	req.Header.Set("Authorization", "Bearer sometoken") // can't be forged cross-site
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Bearer-authenticated POST should be CSRF-exempt: got %d, want 200", w.Code)
	}
}

func csrfFindCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
