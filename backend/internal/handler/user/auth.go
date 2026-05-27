package user

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/auth"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
)

// AuthHandler serves /api/user/auth/*.
type AuthHandler struct {
	users  *usersvc.Service
	auth   *auth.Service
	verify *verification.Service
	smtpOn bool // true when verification mail can actually be sent
}

// NewAuthHandler wires the handler. `smtpOn` controls whether the
// register endpoint enforces a verification code: in dev (SMTP off)
// the code field is optional so testing doesn't require a mail server.
func NewAuthHandler(users *usersvc.Service, a *auth.Service, v *verification.Service, smtpOn bool) *AuthHandler {
	return &AuthHandler{users: users, auth: a, verify: v, smtpOn: smtpOn}
}

// RegisterRoutes mounts /auth under rg (rg is the public user group).
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/email-verification/start", h.EmailVerificationStart)
	rg.POST("/auth/email-verification/confirm", h.EmailVerificationConfirm)
	rg.GET("/auth/registration-policy", h.RegistrationPolicy)
	rg.GET("/auth/oidc/providers", h.OIDCProviders)
	rg.POST("/auth/oidc/start", h.OIDCStart)
	rg.POST("/auth/oidc/callback", h.OIDCCallback)
	rg.POST("/auth/oidc/bind-existing", h.OIDCBindExisting)
	rg.POST("/auth/oidc/create-account", h.OIDCCreateAccount)
}

// oidcProvider mirrors the frontend's OIDCProvider shape. Keep in sync
// with src/api/portal/auth.ts.
type oidcProvider struct {
	Key      string `json:"key,omitempty"`
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	StartURL string `json:"start_url,omitempty"`
	LoginURL string `json:"login_url"`
}

// OIDCProviders returns the list of configured OIDC providers. Returns
// an empty array (not 404) when OIDC isn't configured — frontend treats
// the empty list as "no providers, hide the button row".
func (h *AuthHandler) OIDCProviders(c *gin.Context) {
	providers, err := h.users.ListOIDCProviders(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]oidcProvider, 0, len(providers))
	for _, provider := range providers {
		out = append(out, oidcProvider{
			Key:      provider.ProviderKey,
			Name:     provider.DisplayName,
			Icon:     provider.IconURL,
			StartURL: provider.StartURL,
			LoginURL: "",
		})
	}
	c.JSON(http.StatusOK, out)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	Token         string `json:"token"`
	ExpiresAt     int64  `json:"expires_at"`
	UserID        int64  `json:"user_id"`
	Email         string `json:"email"`
	RedirectAfter string `json:"redirect_after,omitempty"`
	Next          string `json:"next,omitempty"`
}

type oidcPendingResponse struct {
	Status                string           `json:"status"`
	PendingToken          string           `json:"pending_token"`
	Provider              oidcProviderMeta `json:"provider"`
	ProviderEmail         string           `json:"provider_email"`
	ProviderEmailVerified bool             `json:"provider_email_verified"`
	ExistingUser          bool             `json:"existing_user"`
	Email                 string           `json:"email"`
	EmailVerified         bool             `json:"email_verified"`
	ExistingUserID        int64            `json:"existing_user_id,omitempty"`
	ExistingHasOIDC       bool             `json:"existing_has_oidc,omitempty"`
	ExpiresAt             int64            `json:"expires_at"`
	RedirectAfter         string           `json:"redirect_after,omitempty"`
	Next                  string           `json:"next,omitempty"`
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
	Code     string `json:"code"` // 6-digit, required iff SMTP is enabled
}

type emailVerificationRequest struct {
	Email   string `json:"email" binding:"required"`
	Purpose string `json:"purpose" binding:"required"`
}

type emailVerificationConfirmRequest struct {
	Email   string `json:"email" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Purpose string `json:"purpose" binding:"required"`
}

// RegistrationPolicy exposes public registration affordances so the
// login page can hide the verification-code UI when the operator has
// disabled that requirement.
func (h *AuthHandler) RegistrationPolicy(c *gin.Context) {
	required, err := h.users.EmailVerificationRequired(c.Request.Context(), h.smtpOn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"email_verification_required": required,
	})
}

func (h *AuthHandler) EmailVerificationStart(c *gin.Context) {
	var req emailVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	purpose, ok := parseVerificationPurpose(req.Purpose)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification purpose"})
		return
	}
	res, err := h.verify.Start(c.Request.Context(), req.Email, purpose)
	if err != nil {
		h.writeVerificationError(c, err)
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

func (h *AuthHandler) EmailVerificationConfirm(c *gin.Context) {
	var req emailVerificationConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	purpose, ok := parseVerificationPurpose(req.Purpose)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification purpose"})
		return
	}
	res, err := h.verify.Confirm(c.Request.Context(), req.Email, req.Code, purpose)
	if err != nil {
		h.writeVerificationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"verification_token": res.Token,
		"expires_at":         res.ExpiresAt,
	})
}

// Register creates a new portal account. Behaviour matches
// usersvc.Register — gated by PUBLIC_REGISTRATION + EMAIL_DOMAIN_ALLOWLIST.
// When SMTP is enabled, the request must include a valid 6-digit code that
// was previously delivered by the public email-verification start flow.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	emailVerificationRequired, err := h.users.EmailVerificationRequired(c.Request.Context(), h.smtpOn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if emailVerificationRequired {
		if req.Code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少邮箱验证码"})
			return
		}
		if err := h.verify.Consume(c.Request.Context(), req.Email, req.Code, verification.PurposeRegister); err != nil {
			switch {
			case errors.Is(err, verification.ErrCodeMismatch),
				errors.Is(err, verification.ErrCodeNotFound):
				c.JSON(http.StatusBadRequest, gin.H{"error": "验证码不正确"})
			case errors.Is(err, verification.ErrCodeExpired):
				c.JSON(http.StatusBadRequest, gin.H{"error": "验证码已过期，请重新发送"})
			case errors.Is(err, verification.ErrTooManyAttempts):
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "验证次数过多，请重新发送验证码"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
	}

	u, err := h.users.Register(c.Request.Context(), usersvc.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Verified: emailVerificationRequired,
	})
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

// OIDCStart returns the IDP's authorize URL. Frontend navigates
// the user there via window.location.href. Body optional:
// {redirect_after: "/portal/subscription"} preserves a post-login
// target if the user was deep-linked.
func (h *AuthHandler) OIDCStart(c *gin.Context) {
	var req struct {
		RedirectAfter string `json:"redirect_after"`
		ProviderKey   string `json:"provider_key"`
	}
	_ = c.ShouldBindJSON(&req)
	authURL, err := h.users.OIDCStartForProvider(c.Request.Context(), req.ProviderKey, req.RedirectAfter)
	if req.ProviderKey == "" {
		authURL, err = h.users.OIDCStart(c.Request.Context(), req.RedirectAfter)
	}
	if err != nil {
		if errors.Is(err, usersvc.ErrNotImplemented) || errors.Is(err, usersvc.ErrOIDCProviderNotFound) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorize_url": authURL})
}

// OIDCCallback is called by the frontend's callback route after
// the IDP redirected with ?code=&state=. Returns the issued JWT
// so the SPA can store it + navigate to the portal.
func (h *AuthHandler) OIDCCallback(c *gin.Context) {
	var req struct {
		Code  string `json:"code"  binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code+state required"})
		return
	}
	result, err := h.users.OIDCCallback(c.Request.Context(), req.Code, req.State)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrOIDCStateInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrOIDCBadIDToken):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrOIDCEmailConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrOIDCEmailMismatch):
			c.JSON(http.StatusConflict, gin.H{"error": "OIDC email does not match current account"})
		case errors.Is(err, usersvc.ErrOIDCEmailRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC email claim is required"})
		case errors.Is(err, usersvc.ErrOIDCEmailUnverified):
			c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC verified email claim is required", "code": "oidc_verified_email_required"})
		case errors.Is(err, usersvc.ErrDomainNotAllowed):
			c.JSON(http.StatusForbidden, gin.H{"error": "email domain not allowed"})
		case errors.Is(err, usersvc.ErrUserSuspended):
			c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		case errors.Is(err, usersvc.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		case errors.Is(err, usersvc.ErrNotImplemented):
			c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if result.Pending != nil {
		c.JSON(http.StatusOK, oidcPendingResponse{
			Status:          "pending",
			PendingToken:    result.Pending.Token,
			Email:           result.Pending.Email,
			EmailVerified:   result.Pending.EmailVerified,
			ExistingUserID:  result.Pending.ExistingUserID,
			ExistingHasOIDC: result.Pending.ExistingHasOIDC,
			ExpiresAt:       result.Pending.ExpiresAt.Unix(),
			RedirectAfter:   result.RedirectAfter,
			Next:            result.RedirectAfter,
			Provider: oidcProviderMeta{
				Key:  result.Pending.ProviderKey,
				Name: result.Pending.ProviderDisplayName,
				Icon: result.Pending.ProviderIconURL,
			},
			ProviderEmail:         result.Pending.Email,
			ProviderEmailVerified: result.Pending.EmailVerified,
			ExistingUser:          result.Pending.ExistingUser,
		})
		return
	}
	if result.User == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OIDC login did not resolve a user"})
		return
	}
	h.writeToken(c, http.StatusOK, result.User, result.RedirectAfter)
}

func (h *AuthHandler) OIDCBindExisting(c *gin.Context) {
	var req struct {
		PendingToken string `json:"pending_token" binding:"required"`
		Password     string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pending_token+password required"})
		return
	}
	u, redirectAfter, err := h.users.OIDCBindExisting(c.Request.Context(), usersvc.OIDCBindExistingInput{
		PendingToken: req.PendingToken,
		Password:     req.Password,
	})
	if err != nil {
		h.writeOIDCCompletionError(c, err)
		return
	}
	h.writeToken(c, http.StatusOK, u, redirectAfter)
}

func (h *AuthHandler) OIDCCreateAccount(c *gin.Context) {
	var req struct {
		PendingToken      string `json:"pending_token" binding:"required"`
		DisplayName       string `json:"display_name" binding:"required"`
		Email             string `json:"email" binding:"required"`
		Password          string `json:"password" binding:"required"`
		VerificationToken string `json:"verification_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.verify.CheckToken(c.Request.Context(), req.Email, verification.PurposeOIDCCreateAccount, req.VerificationToken); err != nil {
		h.writeVerificationError(c, err)
		return
	}
	input := usersvc.OIDCCreateAccountInput{
		PendingToken: req.PendingToken,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Password:     req.Password,
	}
	if err := h.users.ValidateOIDCCreateAccount(c.Request.Context(), input); err != nil {
		h.writeOIDCCompletionError(c, err)
		return
	}
	if err := h.verify.ConsumeToken(c.Request.Context(), req.Email, verification.PurposeOIDCCreateAccount, req.VerificationToken); err != nil {
		h.writeVerificationError(c, err)
		return
	}
	u, redirectAfter, err := h.users.OIDCCreateAccount(c.Request.Context(), input)
	if err != nil {
		h.writeOIDCCompletionError(c, err)
		return
	}
	h.writeToken(c, http.StatusCreated, u, redirectAfter)
}

func (h *AuthHandler) writeToken(c *gin.Context, status int, u *model.User, redirectAfter string) {
	token, exp, err := h.auth.IssueUserToken(u.ID, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token failed"})
		return
	}
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	c.JSON(status, tokenResponse{
		Token:         token,
		ExpiresAt:     exp.Unix(),
		UserID:        u.ID,
		Email:         email,
		RedirectAfter: redirectAfter,
		Next:          redirectAfter,
	})
}

type oidcProviderMeta struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

func parseVerificationPurpose(raw string) (verification.Purpose, bool) {
	switch verification.Purpose(raw) {
	case verification.PurposeRegister, verification.PurposeOIDCCreateAccount:
		return verification.Purpose(raw), true
	default:
		return "", false
	}
}

func (h *AuthHandler) writeVerificationError(c *gin.Context, err error) {
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

func (h *AuthHandler) writeOIDCCompletionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usersvc.ErrOIDCPendingInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC decision expired. Please sign in again."})
	case errors.Is(err, usersvc.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, usersvc.ErrInvalidEmail), errors.Is(err, usersvc.ErrPasswordTooShort):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, usersvc.ErrDomainNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "email domain not allowed"})
	case errors.Is(err, usersvc.ErrRegistrationOff):
		c.JSON(http.StatusForbidden, gin.H{"error": "public registration is disabled"})
	case errors.Is(err, usersvc.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
	case errors.Is(err, usersvc.ErrOIDCEmailConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, usersvc.ErrUserSuspended):
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
	case errors.Is(err, usersvc.ErrNotImplemented), errors.Is(err, usersvc.ErrOIDCProviderNotFound):
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
