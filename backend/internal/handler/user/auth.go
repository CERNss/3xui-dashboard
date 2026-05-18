package user

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/auth"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
)

// AuthHandler serves /api/user/auth/*.
type AuthHandler struct {
	users *usersvc.Service
	auth  *auth.Service
}

// NewAuthHandler wires the handler.
func NewAuthHandler(users *usersvc.Service, a *auth.Service) *AuthHandler {
	return &AuthHandler{users: users, auth: a}
}

// RegisterRoutes mounts /auth under rg (rg is the public user group).
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/oidc/start", h.OIDCStart)        // 501 in v1
	rg.POST("/auth/oidc/callback", h.OIDCCallback)  // 501 in v1
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
}

// Login validates email+password and returns a user JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, err := h.users.Login(c.Request.Context(), usersvc.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		case errors.Is(err, usersvc.ErrUserSuspended):
			c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	token, exp, err := h.auth.IssueUserToken(u.ID, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token failed"})
		return
	}
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	c.JSON(http.StatusOK, tokenResponse{
		Token:     token,
		ExpiresAt: exp.Unix(),
		UserID:    u.ID,
		Email:     email,
	})
}

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register creates a new portal account. Behaviour matches
// usersvc.Register — gated by PUBLIC_REGISTRATION + EMAIL_DOMAIN_ALLOWLIST.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, err := h.users.Register(c.Request.Context(), usersvc.RegisterInput{Email: req.Email, Password: req.Password})
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrRegistrationOff):
			c.JSON(http.StatusForbidden, gin.H{"error": "public registration is disabled"})
		case errors.Is(err, usersvc.ErrDomainNotAllowed):
			c.JSON(http.StatusForbidden, gin.H{"error": "email domain not allowed"})
		case errors.Is(err, usersvc.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		case errors.Is(err, usersvc.ErrInvalidEmail), errors.Is(err, usersvc.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	// Auto-login after register: hand back a user token immediately.
	token, exp, _ := h.auth.IssueUserToken(u.ID, time.Now())
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	c.JSON(http.StatusCreated, tokenResponse{
		Token:     token,
		ExpiresAt: exp.Unix(),
		UserID:    u.ID,
		Email:     email,
	})
}

// OIDCStart is a v1 stub — returns 501 until OIDC lands.
func (h *AuthHandler) OIDCStart(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "OIDC login is not implemented in this build",
	})
}

// OIDCCallback is a v1 stub.
func (h *AuthHandler) OIDCCallback(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "OIDC login is not implemented in this build",
	})
}
