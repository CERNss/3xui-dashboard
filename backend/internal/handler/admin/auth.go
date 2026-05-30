// Package admin holds the HTTP handlers for the /api/admin/* surface.
package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
	"github.com/cern/3xui-dashboard/internal/session"
)

// AuthHandler implements POST /api/admin/auth/login + logout.
type AuthHandler struct {
	auth *auth.Service
	sess *session.Manager
}

// NewAuthHandler wires the handler to the auth service + cookie manager.
func NewAuthHandler(a *auth.Service, sess *session.Manager) *AuthHandler {
	return &AuthHandler{auth: a, sess: sess}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// loginResponse still carries the JWT in the body so Bearer clients
// (the test harness, automation) can use it. The browser ignores this
// field and authenticates via the httpOnly session cookie set below.
type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"` // unix seconds
	Username  string `json:"username"`
}

// Login validates credentials against ADMIN_USERNAME/ADMIN_PASSWORD
// and returns a signed JWT with audience "admin".
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.auth.CheckAdminCredentials(req.Username, req.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "credentials check failed"})
		return
	}
	token, exp, err := h.auth.IssueAdminToken(req.Username, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token failed"})
		return
	}
	h.sess.WriteSession(c, auth.AudAdmin, token)
	h.sess.IssueCSRF(c)
	c.JSON(http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: exp.Unix(),
		Username:  req.Username,
	})
}

// Logout clears the admin session + CSRF cookies. Unauthenticated by
// design so it always succeeds (clearing a cookie the browser may no
// longer be able to prove it owns); a forged cross-site logout is
// low-harm and blocked by SameSite=Lax anyway.
func (h *AuthHandler) Logout(c *gin.Context) {
	h.sess.ClearSession(c, auth.AudAdmin)
	h.sess.ClearCSRF(c)
	c.Status(http.StatusNoContent)
}

// RegisterRoutes mounts /auth/login + /auth/logout under the supplied
// router group. Designed to be invoked from a parent like
// r.Group("/api/admin").
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/logout", h.Logout)
}
