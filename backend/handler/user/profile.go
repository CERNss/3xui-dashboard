package user

import (
	"net/http"

	"github.com/cern/3xui-dashboard/middleware"
	"github.com/cern/3xui-dashboard/service"
	"github.com/gin-gonic/gin"
)

// ProfileHandler manages user profile operations.
type ProfileHandler struct {
	users *service.UserService
}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler(u *service.UserService) *ProfileHandler {
	return &ProfileHandler{users: u}
}

type updateProfileRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// Get handles GET /api/user/profile.
func (h *ProfileHandler) Get(ctx *gin.Context) {
	userID, _ := ctx.Get(middleware.ContextUserID)
	user, err := h.users.GetByID(userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// Update handles PUT /api/user/profile.
func (h *ProfileHandler) Update(ctx *gin.Context) {
	userID, _ := ctx.Get(middleware.ContextUserID)
	var req updateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.users.UpdateProfile(userID.(uint), req.Email)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// ChangePassword handles POST /api/user/change-password.
func (h *ProfileHandler) ChangePassword(ctx *gin.Context) {
	userID, _ := ctx.Get(middleware.ContextUserID)
	var req changePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.users.ChangePassword(userID.(uint), req.OldPassword, req.NewPassword); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "password updated successfully"})
}
