// Package middleware holds Gin middleware shared across handler
// groups. The auth middleware verifies JWTs and gates access by
// audience: admin routes only admit aud=admin tokens; user routes only
// admit aud=user tokens.
//
// The token is read from the audience's httpOnly session cookie first,
// then from an `Authorization: Bearer` header as a fallback. The
// browser SPA only ever uses the cookie (it stores no token, closing
// the localStorage-XSS hole); the Bearer fallback serves the test
// harness and any programmatic clients.
package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/auth"
	"github.com/cern/3xui-dashboard/internal/session"
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

// RequireActiveUser verifies a user JWT and then checks the current
// database row is still active. It prevents a suspended account from
// continuing to use an old, otherwise-valid JWT until expiry.
func RequireActiveUser(a *auth.Service, users *repository.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !verifyClaims(c, a, auth.AudUser) {
			return
		}
		claims := Claims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
			return
		}
		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
			return
		}
		u, err := users.Get(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "user lookup failed"})
			return
		}
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if u.Status == model.UserStatusSuspended {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account suspended"})
			return
		}
		c.Next()
	}
}

func requireAudience(a *auth.Service, want string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if verifyClaims(c, a, want) {
			c.Next()
		}
	}
}

// verifyClaims pulls the token from the audience's session cookie, or
// the Authorization header if there's no cookie, then verifies it.
func verifyClaims(c *gin.Context, a *auth.Service, want string) bool {
	tok := session.ReadSession(c, want)
	if tok == "" {
		tok = bearerToken(c.GetHeader("Authorization"))
	}
	if tok == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing credentials"})
		return false
	}
	claims, err := a.VerifyToken(tok, want)
	switch {
	case err == nil:
		c.Set(ContextKey, claims)
		return true
	case errors.Is(err, auth.ErrWrongAudience):
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "wrong audience"})
	case errors.Is(err, auth.ErrTokenExpired):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
	case errors.Is(err, auth.ErrInvalidToken):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
	default:
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token verification failed"})
	}
	return false
}

// Claims pulls the verified claims out of the Gin context. Safe to
// call from any handler protected by RequireAdmin/RequireActiveUser;
// the middleware guarantees presence.
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
