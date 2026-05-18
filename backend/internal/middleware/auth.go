// Package middleware holds Gin middleware shared across handler
// groups. The auth middleware verifies JWT bearer tokens and gates
// access by audience: admin routes only admit aud=admin tokens; user
// routes only admit aud=user tokens.
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
)

// ContextKey is the Gin context key under which verified claims are
// stored after the middleware runs.
const ContextKey = "auth.claims"

// RequireAdmin returns a middleware that 401s on a missing/invalid/
// expired bearer token and 403s on a token with the wrong audience.
// Verified claims are attached to the request context for downstream
// handlers.
func RequireAdmin(a *auth.Service) gin.HandlerFunc {
	return requireAudience(a, auth.AudAdmin)
}

// RequireUser is the portal-side equivalent of RequireAdmin.
func RequireUser(a *auth.Service) gin.HandlerFunc {
	return requireAudience(a, auth.AudUser)
}

func requireAudience(a *auth.Service, want string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := bearerToken(c.GetHeader("Authorization"))
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := a.VerifyToken(tok, want)
		switch {
		case err == nil:
			c.Set(ContextKey, claims)
			c.Next()
		case errors.Is(err, auth.ErrWrongAudience):
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "wrong audience"})
		case errors.Is(err, auth.ErrTokenExpired):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		case errors.Is(err, auth.ErrInvalidToken):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		default:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token verification failed"})
		}
	}
}

// Claims pulls the verified claims out of the Gin context. Safe to
// call from any handler protected by RequireAdmin/RequireUser; the
// middleware guarantees presence.
func Claims(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ContextKey)
	if !ok {
		return nil
	}
	cl, _ := v.(*auth.Claims)
	return cl
}

// bearerToken extracts the token from an "Authorization: Bearer ..."
// header. Returns "" on any malformed input.
func bearerToken(h string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
