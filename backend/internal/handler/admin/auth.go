// Package admin holds the HTTP handlers for the /api/admin/* surface.
package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
)

// AuthHandler implements POST /api/admin/auth/login.
type AuthHandler struct {
	auth *auth.Service
}

// NewAuthHandler wires the handler to the auth service.
func NewAuthHandler(a *auth.Service) *AuthHandler { return &AuthHandler{auth: a} }

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

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
	c.JSON(http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: exp.Unix(),
		Username:  req.Username,
	})
}

// RegisterRoutes mounts /auth/login under the supplied router group.
// Designed to be invoked from a parent like r.Group("/api/admin").
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
}
