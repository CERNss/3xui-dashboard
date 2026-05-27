package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/repository"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
)

// AccountHandler serves authenticated /api/user account endpoints.
type AccountHandler struct {
	users  *usersvc.Service
	repo   *repository.UserRepo
	verify *verification.Service
}

// NewAccountHandler wires the handler.
func NewAccountHandler(users *usersvc.Service, repo *repository.UserRepo, verify *verification.Service) *AccountHandler {
	return &AccountHandler{users: users, repo: repo, verify: verify}
}

// RegisterRoutes mounts the account endpoints under rg (RequireUser).
func (h *AccountHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/profile", h.Profile)
	rg.PATCH("/profile", h.UpdateProfile)
	rg.GET("/login-methods", h.LoginMethods)
	rg.POST("/email-verification/start", h.EmailVerificationStart)
	rg.POST("/email-verification/confirm", h.EmailVerificationConfirm)
	rg.POST("/change-email", h.ChangeEmail)
	rg.POST("/change-password", h.ChangePassword)
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

func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req usersvc.UpdateProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, err := h.users.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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

type accountEmailVerificationRequest struct {
	Email   string `json:"email" binding:"required"`
	Purpose string `json:"purpose" binding:"required"`
}

type accountEmailVerificationConfirmRequest struct {
	Email   string `json:"email" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Purpose string `json:"purpose" binding:"required"`
}

type changeEmailRequest struct {
	Email             string `json:"email" binding:"required"`
	VerificationToken string `json:"verification_token" binding:"required"`
}

func (h *AccountHandler) EmailVerificationStart(c *gin.Context) {
	var req accountEmailVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	purpose, ok := parseAccountVerificationPurpose(req.Purpose)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification purpose"})
		return
	}
	res, err := h.verify.Start(c.Request.Context(), req.Email, purpose)
	if err != nil {
		writeAccountVerificationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":               "ok",
		"expires_at":           res.ExpiresAt,
		"resend_at":            res.ResendAt,
		"cooldown_seconds":     res.CooldownSeconds,
		"resend_after_seconds": res.CooldownSeconds,
	})
}

func (h *AccountHandler) EmailVerificationConfirm(c *gin.Context) {
	var req accountEmailVerificationConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	purpose, ok := parseAccountVerificationPurpose(req.Purpose)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification purpose"})
		return
	}
	res, err := h.verify.Confirm(c.Request.Context(), req.Email, req.Code, purpose)
	if err != nil {
		writeAccountVerificationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"verification_token": res.Token,
		"expires_at":         res.ExpiresAt,
	})
}

func (h *AccountHandler) ChangeEmail(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req changeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.verify.ConsumeToken(c.Request.Context(), req.Email, verification.PurposeChangeEmail, req.VerificationToken); err != nil {
		writeAccountVerificationError(c, err)
		return
	}
	u, err := h.users.ChangeEmailVerified(c.Request.Context(), userID, req.Email)
	if err != nil {
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
	c.JSON(http.StatusOK, u)
}

// OIDCLinkStart starts provider binding for the authenticated user.
func (h *AccountHandler) OIDCLinkStart(c *gin.Context) {
	userID, ok := h.subject(c)
	if !ok {
		return
	}
	var req struct {
		RedirectAfter string `json:"redirect_after"`
		ProviderKey   string `json:"provider_key"`
	}
	_ = c.ShouldBindJSON(&req)
	authURL, err := h.users.OIDCLinkStartForProvider(c.Request.Context(), userID, req.ProviderKey, req.RedirectAfter)
	if req.ProviderKey == "" {
		authURL, err = h.users.OIDCLinkStart(c.Request.Context(), userID, req.RedirectAfter)
	}
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrNotImplemented), errors.Is(err, usersvc.ErrOIDCProviderNotFound):
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

func parseAccountVerificationPurpose(raw string) (verification.Purpose, bool) {
	switch verification.Purpose(raw) {
	case verification.PurposeChangeEmail, verification.PurposeOIDCCreateAccount:
		return verification.Purpose(raw), true
	default:
		return "", false
	}
}

func writeAccountVerificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, verification.ErrRateLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请稍等再发，验证码 60 秒内只能发一次"})
	case errors.Is(err, verification.ErrCodeMismatch),
		errors.Is(err, verification.ErrCodeNotFound),
		errors.Is(err, verification.ErrTokenInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码不正确"})
	case errors.Is(err, verification.ErrCodeExpired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码已过期，请重新发送"})
	case errors.Is(err, verification.ErrTooManyAttempts):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "验证次数过多，请重新发送验证码"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
