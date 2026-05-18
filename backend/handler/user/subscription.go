package user

import (
	"fmt"
	"net/http"

	"github.com/cern/3xui-dashboard/config"
	"github.com/cern/3xui-dashboard/middleware"
	"github.com/cern/3xui-dashboard/service"
	"github.com/gin-gonic/gin"
)

// SubscriptionHandler returns subscription link info for the current user.
type SubscriptionHandler struct {
	users *service.UserService
}

// NewSubscriptionHandler constructs a SubscriptionHandler.
func NewSubscriptionHandler(u *service.UserService) *SubscriptionHandler {
	return &SubscriptionHandler{users: u}
}

type subscriptionResponse struct {
	SubID  string `json:"subId"`
	SubURL string `json:"subUrl"`
	Email  string `json:"email"`
}

// Get handles GET /api/user/subscription.
func (h *SubscriptionHandler) Get(ctx *gin.Context) {
	userID, _ := ctx.Get(middleware.ContextUserID)
	user, err := h.users.GetByID(userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.XUISubID == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "no subscription linked to this account"})
		return
	}
	subURL := fmt.Sprintf("%s%s/sub/%s", config.C.XUI.BaseURL, config.C.XUI.BasePath, user.XUISubID)
	ctx.JSON(http.StatusOK, subscriptionResponse{
		SubID:  user.XUISubID,
		SubURL: subURL,
		Email:  user.XUIClientEmail,
	})
}
