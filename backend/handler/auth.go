package handler

import (
	"net/http"

	"github.com/cern/3xui-dashboard/model"
	"github.com/cern/3xui-dashboard/service"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	users *service.UserService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(users *service.UserService) *AuthHandler {
	return &AuthHandler{users: users}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.users.Login(req.Username, req.Password)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	token, err := service.GenerateToken(user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	ctx.JSON(http.StatusOK, authResponse{Token: token, User: user})
}

// Register handles POST /api/auth/register.
// The first registered user becomes an admin; subsequent ones are regular users.
func (h *AuthHandler) Register(ctx *gin.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := model.RoleUser
	adminCount, err := h.users.CountAdmins()
	if err == nil && adminCount == 0 {
		role = model.RoleAdmin
	}

	user, err := h.users.Register(req.Username, req.Email, req.Password, role)
	if err != nil {
		ctx.JSON(http.StatusConflict, gin.H{"error": "username or email already taken"})
		return
	}
	token, err := service.GenerateToken(user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	ctx.JSON(http.StatusCreated, authResponse{Token: token, User: user})
}
