package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that emits the right
// Access-Control-* headers based on `allowedOrigins`.
//
// Two modes:
//
//   - allowedOrigins == nil OR contains "*": permissive — echo the
//     request's Origin back (with credentials disabled, per the
//     CORS spec). Suitable for local dev or fully-public APIs.
//   - allowedOrigins is a list of exact origins (e.g.
//     "https://panel.example.com"): only those origins receive the
//     allow header. Credentials are enabled because the set is
//     closed.
//
// OPTIONS preflights short-circuit with 204 — gin's recovery still
// runs in case a handler-chain hook panics on the empty body.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	wildcard := len(allowedOrigins) == 0
	exact := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
			continue
		}
		if o != "" {
			exact[o] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			allow := false
			withCreds := false
			if wildcard {
				// Echo origin but DON'T enable credentials — browsers
				// reject "Access-Control-Allow-Origin: *" with creds.
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				allow = true
			} else if _, ok := exact[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				allow = true
				withCreds = true
			}
			if allow {
				c.Writer.Header().Add("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-CSRF-Token")
				c.Writer.Header().Set("Access-Control-Max-Age", "600")
				_ = withCreds
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
