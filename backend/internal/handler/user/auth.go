package user

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/service/auth"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
)

// AuthHandler serves /api/user/auth/*.
type AuthHandler struct {
	users  *usersvc.Service
	auth   *auth.Service
	verify *verification.Service
	oidc   config.OIDC
	smtpOn bool // true when verification mail can actually be sent
}

// NewAuthHandler wires the handler. `smtpOn` controls whether the
// register endpoint enforces a verification code: in dev (SMTP off)
// the code field is optional so testing doesn't require a mail server.
//
// `oidcCfg` lets the providers endpoint expose the operator's configured
// OIDC IdP (display name + icon) so the login page can render the
// "使用 X 登录" button. Empty config → empty list → button hides.
func NewAuthHandler(users *usersvc.Service, a *auth.Service, v *verification.Service, oidcCfg config.OIDC, smtpOn bool) *AuthHandler {
	return &AuthHandler{users: users, auth: a, verify: v, oidc: oidcCfg, smtpOn: smtpOn}
}

// RegisterRoutes mounts /auth under rg (rg is the public user group).
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/send-code", h.SendCode)
	rg.GET("/auth/oidc/providers", h.OIDCProviders)
	rg.POST("/auth/oidc/start", h.OIDCStart)
	rg.POST("/auth/oidc/callback", h.OIDCCallback)
}

// oidcProvider mirrors the frontend's OIDCProvider shape. Keep in sync
// with src/api/portal/auth.ts.
type oidcProvider struct {
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	LoginURL string `json:"login_url"`
}

// OIDCProviders returns the list of configured OIDC providers. Returns
// an empty array (not 404) when OIDC isn't configured — frontend treats
// the empty list as "no providers, hide the button row".
func (h *AuthHandler) OIDCProviders(c *gin.Context) {
	if !h.oidc.Enabled() {
		c.JSON(http.StatusOK, []oidcProvider{})
		return
	}
	name := h.oidc.DisplayName
	if name == "" {
		// Fall back to issuer hostname so the button has *something* useful
		// to show even if the operator didn't set OIDC_DISPLAY_NAME.
		if u, err := url.Parse(h.oidc.Issuer); err == nil && u.Host != "" {
			name = u.Host
		} else {
			name = "OIDC"
		}
	}
	c.JSON(http.StatusOK, []oidcProvider{{
		Name:     name,
		Icon:     h.oidc.IconURL,
		LoginURL: "/api/user/auth/oidc/start",
	}})
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
	Code     string `json:"code"` // 6-digit, required iff SMTP is enabled
}

type sendCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

// SendCode dispatches a fresh 6-digit verification code to the given email.
// Rate-limited per (email, purpose). Returns 204 on success.
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	err := h.verify.SendCode(c.Request.Context(), req.Email, verification.PurposeRegister)
	if err != nil {
		switch {
		case errors.Is(err, verification.ErrRateLimited):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "请稍等再发，验证码 60 秒内只能发一次"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// Register creates a new portal account. Behaviour matches
// usersvc.Register — gated by PUBLIC_REGISTRATION + EMAIL_DOMAIN_ALLOWLIST.
// When SMTP is enabled, the request must include a valid 6-digit code that
// was previously delivered via SendCode.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if h.smtpOn {
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

// OIDCStart returns the IDP's authorize URL. Frontend navigates
// the user there via window.location.href. Body optional:
// {redirect_after: "/portal/subscription"} preserves a post-login
// target if the user was deep-linked.
func (h *AuthHandler) OIDCStart(c *gin.Context) {
	var req struct {
		RedirectAfter string `json:"redirect_after"`
	}
	_ = c.ShouldBindJSON(&req)
	authURL, err := h.users.OIDCStart(c.Request.Context(), req.RedirectAfter)
	if err != nil {
		if errors.Is(err, usersvc.ErrNotImplemented) {
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
	u, err := h.users.OIDCCallback(c.Request.Context(), req.Code, req.State)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrOIDCStateInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrOIDCBadIDToken):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrNotImplemented):
			c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC not configured"})
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
