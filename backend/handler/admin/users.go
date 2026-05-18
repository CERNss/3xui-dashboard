package admin

import (
	"net/http"
	"strconv"

	"github.com/cern/3xui-dashboard/service"
	"github.com/gin-gonic/gin"
)

// UsersHandler manages dashboard user accounts.
type UsersHandler struct {
	users *service.UserService
}

// NewUsersHandler constructs a UsersHandler.
func NewUsersHandler(u *service.UserService) *UsersHandler {
	return &UsersHandler{users: u}
}

type updateUserRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Role           string `json:"role" binding:"required,oneof=admin user"`
	XUIClientEmail string `json:"xuiClientEmail"`
	XUISubID       string `json:"xuiSubId"`
}

// List handles GET /api/admin/users.
func (h *UsersHandler) List(ctx *gin.Context) {
	users, err := h.users.ListUsers()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, users)
}

// Update handles PUT /api/admin/users/:id.
func (h *UsersHandler) Update(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.users.UpdateUser(uint(id), req.Username, req.Email, req.Role, req.XUIClientEmail, req.XUISubID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// Delete handles DELETE /api/admin/users/:id.
func (h *UsersHandler) Delete(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.users.DeleteUser(uint(id)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.Status(http.StatusNoContent)
}
