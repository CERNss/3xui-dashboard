package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouter() (*gin.Engine, *auth.Service) {
	s := auth.New("test-secret", time.Hour, "admin", "letmein")
	r := gin.New()
	r.GET("/api/admin/whoami", RequireAdmin(s), func(c *gin.Context) {
		cl := Claims(c)
		c.JSON(http.StatusOK, gin.H{"sub": cl.Subject})
	})
	r.GET("/api/user/whoami", requireAudience(s, auth.AudUser), func(c *gin.Context) {
		cl := Claims(c)
		c.JSON(http.StatusOK, gin.H{"sub": cl.Subject})
	})
	return r, s
}

func bearer(t string) string { return "Bearer " + t }

func TestRequireAdmin_AcceptsAdminToken(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueAdminToken("admin", time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.Header.Set("Authorization", bearer(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin token on admin route: got %d, want 200", w.Code)
	}
}

func TestRequireAdmin_RejectsUserToken(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueUserToken(42, time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.Header.Set("Authorization", bearer(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("user token on admin route: got %d, want 403", w.Code)
	}
}

func TestRequireUser_RejectsAdminToken(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueAdminToken("admin", time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/user/whoami", nil)
	req.Header.Set("Authorization", bearer(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin token on user route: got %d, want 403", w.Code)
	}
}

func TestRequireAdmin_MissingHeader(t *testing.T) {
	r, _ := newRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing header: got %d, want 401", w.Code)
	}
}

func TestRequireAdmin_MalformedHeader(t *testing.T) {
	r, _ := newRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.Header.Set("Authorization", "Token abc")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("malformed header: got %d, want 401", w.Code)
	}
}

func TestRequireAdmin_ExpiredToken(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueAdminToken("admin", time.Now().Add(-2*time.Hour))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.Header.Set("Authorization", bearer(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: got %d, want 401", w.Code)
	}
}

// Cookie-based auth: the browser SPA authenticates via the httpOnly
// session cookie, never the Authorization header. The cookie names are
// hardcoded here on purpose — they're a wire contract, so a rename
// should make this fail loudly.
func adminSessionCookie(tok string) *http.Cookie {
	return &http.Cookie{Name: "3xui_admin_session", Value: tok}
}

func userSessionCookie(tok string) *http.Cookie {
	return &http.Cookie{Name: "3xui_user_session", Value: tok}
}

func TestRequireAdmin_AcceptsAdminCookie(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueAdminToken("admin", time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.AddCookie(adminSessionCookie(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin cookie on admin route: got %d, want 200", w.Code)
	}
}

func TestRequireAdmin_RejectsUserTokenInCookie(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueUserToken(42, time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/whoami", nil)
	req.AddCookie(adminSessionCookie(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("user token in admin cookie: got %d, want 403", w.Code)
	}
}

func TestRequireUser_AcceptsUserCookie(t *testing.T) {
	r, s := newRouter()
	tok, _, _ := s.IssueUserToken(7, time.Now())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/user/whoami", nil)
	req.AddCookie(userSessionCookie(tok))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("user cookie on user route: got %d, want 200", w.Code)
	}
}
