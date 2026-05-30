package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
)

func init() { gin.SetMode(gin.TestMode) }

func TestWriteSession_CookieAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NewManager(true, time.Hour).WriteSession(c, auth.AudAdmin, "jwt-value")

	got := findCookie(w.Result().Cookies(), adminCookie)
	if got == nil {
		t.Fatalf("admin session cookie not set; got %v", w.Result().Cookies())
	}
	if got.Value != "jwt-value" {
		t.Errorf("value = %q, want jwt-value", got.Value)
	}
	if !got.HttpOnly {
		t.Error("session cookie must be HttpOnly so JS can't read the JWT")
	}
	if !got.Secure {
		t.Error("secure manager must set Secure")
	}
	if got.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", got.SameSite)
	}
	if got.Path != adminPath {
		t.Errorf("path = %q, want %q", got.Path, adminPath)
	}
	if got.MaxAge != int(time.Hour.Seconds()) {
		t.Errorf("maxage = %d, want %d", got.MaxAge, int(time.Hour.Seconds()))
	}
}

func TestWriteSession_InsecureInDev(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NewManager(false, time.Hour).WriteSession(c, auth.AudUser, "x")
	got := findCookie(w.Result().Cookies(), userCookie)
	if got == nil || got.Secure {
		t.Fatalf("dev manager must not set Secure (cookie would be dropped over http); got %+v", got)
	}
}

func TestReadSession_RoundTrip(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(&http.Cookie{Name: adminCookie, Value: "tok-123"})

	if got := ReadSession(c, auth.AudAdmin); got != "tok-123" {
		t.Errorf("ReadSession(admin) = %q, want tok-123", got)
	}
	// The user audience reads a different (absent) cookie — audiences
	// never cross-read.
	if got := ReadSession(c, auth.AudUser); got != "" {
		t.Errorf("ReadSession(user) = %q, want empty", got)
	}
}

func TestClearSession_Deletes(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NewManager(true, time.Hour).ClearSession(c, auth.AudAdmin)
	got := findCookie(w.Result().Cookies(), adminCookie)
	if got == nil || got.MaxAge >= 0 {
		t.Fatalf("clear must set MaxAge<0 with matching path; got %+v", got)
	}
	if got.Path != adminPath {
		t.Errorf("clear path = %q, want %q (must match to delete)", got.Path, adminPath)
	}
}

func TestIssueCSRF_SetsReadableToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tok := NewManager(true, time.Hour).IssueCSRF(c)
	if tok == "" {
		t.Fatal("IssueCSRF returned empty token")
	}
	got := findCookie(w.Result().Cookies(), CSRFCookie)
	if got == nil {
		t.Fatal("csrf cookie not set")
	}
	if got.HttpOnly {
		t.Error("csrf cookie must be readable by JS (not HttpOnly)")
	}
	if got.Value != tok {
		t.Errorf("cookie value %q != returned token %q", got.Value, tok)
	}
	if got.Path != csrfPath {
		t.Errorf("csrf path = %q, want %q", got.Path, csrfPath)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, ck := range cookies {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}
