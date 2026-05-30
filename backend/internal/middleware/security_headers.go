package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets a baseline set of HTTP response headers that
// harden the dashboard against XSS, clickjacking and MIME sniffing.
//
// CSP rationale (the most consequential one):
//   - script-src 'self'  → no inline scripts, no remote scripts. The
//     React bundle is served by go:embed from the same origin, so
//     'self' is enough. This is what shrinks the blast radius of
//     localStorage-stored JWTs: even if user-supplied content (a
//     pool name, a client comment) ever ended up rendered raw, the
//     attacker would still be unable to load a script to read the
//     token from window.localStorage.
//   - style-src 'self' 'unsafe-inline'  → AntD injects style tags at
//     runtime, so inline styles can't be banned without major rework.
//     Inline styles cannot execute JavaScript so the relaxation is
//     bounded.
//   - img-src 'self' data: https:  → admin can configure a brand
//     icon URL and QR codes embed inline data: URIs.
//   - connect-src 'self'  → XHR/fetch only back to the dashboard
//     origin. Combined with the OIDC SSRF guard upstream, this
//     prevents a compromised script (if any) from beaconing out.
//   - frame-ancestors 'none'  → defence against clickjacking. The
//     dashboard should never be iframed.
func SecurityHeaders() gin.HandlerFunc {
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// X-Frame-Options is now superseded by CSP frame-ancestors but
		// keep it for older user-agents that don't grok CSP level 2.
		h.Set("X-Frame-Options", "DENY")
		c.Next()
	}
}
