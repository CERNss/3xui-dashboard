package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/repository"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
)

// AccountHandler serves /api/user/profile, /change-password, /bind-email.
type AccountHandler struct {
	users *usersvc.Service
	repo  *repository.UserRepo
}

// NewAccountHandler wires the handler.
func NewAccountHandler(users *usersvc.Service, repo *repository.UserRepo) *AccountHandler {
	return &AccountHandler{users: users, repo: repo}
}

// RegisterRoutes mounts the account endpoints under rg (RequireUser).
func (h *AccountHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/profile", h.Profile)
	rg.GET("/login-methods", h.LoginMethods)
	rg.POST("/change-password", h.ChangePassword)
	rg.POST("/bind-email", h.BindEmail)
	rg.POST("/oidc/link/start", h.OIDCLinkStart)
	rg.POST("/rotate-sub-id", h.RotateSubID)
}

// Profile returns the authenticated user's row.
func (h *AccountHandler) Profile(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	u, err := h.repo.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// LoginMethods returns the authenticated user's linked sign-in methods.
func (h *AccountHandler) LoginMethods(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	methods, err := h.users.LoginMethods(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, methods)
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword changes the authenticated user's password.
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.users.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, usersvc.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "old password mismatch"})
		case errors.Is(err, usersvc.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type bindEmailRequest struct {
	Email string `json:"email" binding:"required"`
}

// BindEmail attaches an email to the authenticated user's account.
func (h *AccountHandler) BindEmail(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req bindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.users.BindEmail(c.Request.Context(), userID, req.Email); err != nil {
		switch {
		case errors.Is(err, usersvc.ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrDomainNotAllowed):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// OIDCLinkStart starts provider binding for the authenticated user.
func (h *AccountHandler) OIDCLinkStart(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req struct {
		RedirectAfter string `json:"redirect_after"`
	}
	_ = c.ShouldBindJSON(&req)
	authURL, err := h.users.OIDCLinkStart(c.Request.Context(), userID, req.RedirectAfter)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrNotImplemented):
			c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
		case errors.Is(err, usersvc.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, usersvc.ErrUserSuspended):
			c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorize_url": authURL})
}

// RotateSubID rotates the authenticated user's sub_id. The old
// /sub/:oldID immediately 404s; clients need to re-import the URL.
// Empty body — no params to send.
func (h *AccountHandler) RotateSubID(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	newID, err := h.users.RotateSubID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sub_id": newID})
}

func (h *AccountHandler) subject(c *gin.Context) (int64, bool) {
	claims := middleware.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return 0, false
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return 0, false
	}
	return id, true
}
