// Package session is the cookie transport for the dashboard's auth
// sessions. It owns two things and nothing else:
//
//   - the httpOnly JWT session cookie (one name+path per audience), and
//   - the readable double-submit CSRF token cookie.
//
// It is deliberately transport-only: it does not mint or verify JWTs
// (that's service/auth). Keeping it separate lets both the auth
// middleware (which only reads cookies) and the auth handlers (which
// write them) share one definition of "what the cookies are called and
// how they're scoped" without dragging in JWT logic.
//
// Why cookies at all: the JWT used to live in localStorage, readable by
// any JavaScript. A single XSS anywhere in the SPA could exfiltrate a
// long-lived token. Moving it into an httpOnly cookie makes the token
// unreadable by JS, which is the real fix; CSP only ever shrank the
// blast radius. The cost of cookies is CSRF exposure, handled by the
// CSRF middleware + SameSite=Lax below.
package session

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
)

// Cookie names + the header the SPA echoes the CSRF token in. The admin
// and portal sessions use distinct names AND distinct paths so the two
// audiences never share a cookie on the same origin. The JWT `aud`
// claim remains the authoritative gate; path scoping is defence in
// depth (and keeps the session cookie off /sub, /api/public, etc.).
const (
	adminCookie = "3xui_admin_session"
	userCookie  = "3xui_user_session"

	adminPath = "/api/admin"
	userPath  = "/api/user"

	// CSRFCookie is NOT httpOnly — the SPA reads it and echoes the
	// value back in CSRFHeader for the double-submit check. It carries
	// no secret. Path "/" so JS can read it from any SPA route and so
	// it is sent to both /api/admin and /api/user.
	CSRFCookie = "3xui_csrf"
	csrfPath   = "/"

	// CSRFHeader is where unsafe cookie-authenticated requests must
	// echo the CSRFCookie value.
	CSRFHeader = "X-CSRF-Token"
)

// sessionSpec maps an audience to its cookie name + path. ok is false
// for an unknown audience so callers fail closed rather than writing a
// mystery cookie.
func sessionSpec(aud string) (name, path string, ok bool) {
	switch aud {
	case auth.AudAdmin:
		return adminCookie, adminPath, true
	case auth.AudUser:
		return userCookie, userPath, true
	default:
		return "", "", false
	}
}

// Manager writes session + CSRF cookies. The only state it carries is
// whether to mark cookies Secure (true in prod where the panel is
// behind TLS; false in dev/test over plain http://localhost, where a
// Secure cookie would be silently dropped) and the cookie lifetime,
// which tracks the access-token TTL so the cookie expires with the JWT.
type Manager struct {
	secure bool
	ttl    time.Duration
}

// NewManager builds a Manager. secure should be (cfg.Env == "prod").
func NewManager(secure bool, ttl time.Duration) *Manager {
	return &Manager{secure: secure, ttl: ttl}
}

// WriteSession sets the httpOnly JWT cookie for the given audience.
func (m *Manager) WriteSession(c *gin.Context, aud, token string) {
	name, path, ok := sessionSpec(aud)
	if !ok {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     path,
		MaxAge:   int(m.ttl.Seconds()),
		Secure:   m.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSession deletes the audience's session cookie. Path must match
// the one used to set it or the browser keeps the cookie.
func (m *Manager) ClearSession(c *gin.Context, aud string) {
	name, path, ok := sessionSpec(aud)
	if !ok {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		Secure:   m.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// WriteCSRF sets the readable CSRF cookie to an explicit value.
func (m *Manager) WriteCSRF(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CSRFCookie,
		Value:    token,
		Path:     csrfPath,
		MaxAge:   int(m.ttl.Seconds()),
		Secure:   m.secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCSRF deletes the CSRF cookie.
func (m *Manager) ClearCSRF(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CSRFCookie,
		Value:    "",
		Path:     csrfPath,
		MaxAge:   -1,
		Secure:   m.secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// IssueCSRF generates a fresh random token, sets it as the CSRF cookie,
// and returns it. Returns "" only if the system CSPRNG fails, in which
// case the double-submit check fails closed (no token to echo).
func (m *Manager) IssueCSRF(c *gin.Context) string {
	tok := randomToken()
	if tok == "" {
		return ""
	}
	m.WriteCSRF(c, tok)
	return tok
}

// ReadSession returns the raw JWT from the audience's session cookie,
// or "" if absent. A free function (no Manager needed) so the auth
// middleware can read without depending on cookie-write config.
func ReadSession(c *gin.Context, aud string) string {
	name, _, ok := sessionSpec(aud)
	if !ok {
		return ""
	}
	v, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return v
}

// ReadCSRF returns the CSRF cookie value, or "" if absent.
func ReadCSRF(c *gin.Context) string {
	v, err := c.Cookie(CSRFCookie)
	if err != nil {
		return ""
	}
	return v
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
