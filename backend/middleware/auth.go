package middleware

import (
	"net/http"
	"strings"

	"github.com/cern/3xui-dashboard/service"
	"github.com/gin-gonic/gin"
)

const (
	ContextUserID   = "userID"
	ContextUsername = "username"
	ContextRole     = "role"
)

// Auth validates the Authorization: Bearer <token> header and injects user info into the context.
func Auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if header == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header missing"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}
		claims, err := service.ParseToken(parts[1])
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		ctx.Set(ContextUserID, claims.UserID)
		ctx.Set(ContextUsername, claims.Username)
		ctx.Set(ContextRole, claims.Role)
		ctx.Next()
	}
}

// RequireAdmin aborts with 403 if the authenticated user is not an admin.
// Must be called after Auth().
func RequireAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, _ := ctx.Get(ContextRole)
		if role != "admin" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		ctx.Next()
	}
}
