package user

import (
	"net/http"

	"github.com/cern/3xui-dashboard/middleware"
	"github.com/cern/3xui-dashboard/service"
	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
)

// TrafficHandler returns traffic stats for the current user.
type TrafficHandler struct {
	users *service.UserService
	xui   *xui.Client
}

// NewTrafficHandler constructs a TrafficHandler.
func NewTrafficHandler(u *service.UserService, c *xui.Client) *TrafficHandler {
	return &TrafficHandler{users: u, xui: c}
}

// Get handles GET /api/user/traffic.
func (h *TrafficHandler) Get(ctx *gin.Context) {
	userID, _ := ctx.Get(middleware.ContextUserID)
	user, err := h.users.GetByID(userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.XUIClientEmail == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "no xui client linked to this account"})
		return
	}
	data, err := h.xui.GetClientTraffics(user.XUIClientEmail)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}
