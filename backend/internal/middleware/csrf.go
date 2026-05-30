package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/session"
)

// CSRF implements double-submit-cookie CSRF protection for cookie-based
// sessions. Mount it on the authenticated route groups (after the auth
// middleware, so only established sessions are issued tokens).
//
// Behaviour:
//
//   - Safe methods (GET/HEAD/OPTIONS/TRACE): if no CSRF cookie is
//     present, lazily issue one. The SPA loads data (GETs) before it
//     ever mutates, so by the time a POST fires the readable cookie is
//     always there to echo. This also self-heals a dropped cookie.
//   - Unsafe methods (POST/PUT/PATCH/DELETE): require the X-CSRF-Token
//     header to equal the CSRF cookie (constant-time). A forged
//     cross-site request can ride the SameSite=Lax session cookie only
//     in edge cases, but can never read the CSRF cookie to echo it.
//   - Bearer-authenticated requests are exempt: an attacker cannot set
//     an Authorization header on a cross-site request (it forces a CORS
//     preflight the evil origin can't pass), so the Bearer path is
//     CSRF-immune by construction. This is what keeps the test harness
//     and any programmatic clients working without a CSRF dance.
func CSRF(sess *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isCSRFSafeMethod(c.Request.Method) {
			if session.ReadCSRF(c) == "" {
				sess.IssueCSRF(c)
			}
			c.Next()
			return
		}
		// Bearer-authenticated → not forgeable cross-site, skip CSRF.
		if c.GetHeader("Authorization") != "" {
			c.Next()
			return
		}
		cookie := session.ReadCSRF(c)
		header := c.GetHeader(session.CSRFHeader)
		if cookie == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf token invalid"})
			return
		}
		c.Next()
	}
}

func isCSRFSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
