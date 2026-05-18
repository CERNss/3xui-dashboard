package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/repository"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
)

// UserHandler serves /api/admin/users/*.
type UserHandler struct {
	users *usersvc.Service
	repo  *repository.UserRepo
}

// NewUserHandler wires the handler.
func NewUserHandler(users *usersvc.Service, repo *repository.UserRepo) *UserHandler {
	return &UserHandler{users: users, repo: repo}
}

// RegisterRoutes mounts /users under rg.
func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/users")
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.POST("/:id/suspend", h.Suspend)
	g.POST("/:id/unsuspend", h.Unsuspend)
	g.POST("/:id/balance", h.AdjustBalance)
	g.DELETE("/:id", h.Delete)
}

func (h *UserHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := h.users.AdminList(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": rows, "limit": limit, "offset": offset})
}

func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	u, err := h.repo.Get(c.Request.Context(), id)
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

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var in usersvc.AdminUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	u, err := h.users.AdminUpdate(c.Request.Context(), id, in)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, usersvc.ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Suspend(c *gin.Context)   { h.setStatus(c, "suspended") }
func (h *UserHandler) Unsuspend(c *gin.Context) { h.setStatus(c, "active") }

func (h *UserHandler) setStatus(c *gin.Context, status string) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	statusVal := status
	_, err := h.users.AdminUpdate(c.Request.Context(), id, usersvc.AdminUpdateInput{Status: &statusVal})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "status": status})
}

type balanceAdjustRequest struct {
	Delta  int64  `json:"delta_cents" binding:"required"`
	Note   string `json:"note"`
	Reason string `json:"reason"` // defaults to admin_adjust
}

func (h *UserHandler) AdjustBalance(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var req balanceAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "admin_adjust"
	}
	newBal, err := h.repo.AdjustBalance(c.Request.Context(), id, req.Delta, reason, req.Note, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "balance_cents": newBal})
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.users.AdminDelete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
