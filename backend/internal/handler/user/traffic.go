// Package user holds the HTTP handlers for the /api/user/* surface
// — the portal where end users see their own traffic + subscription.
package user

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
)

// TrafficHandler serves /api/user/traffic — scoped to the
// authenticated user's own ownerships.
type TrafficHandler struct{ svc *traffic.Service }

// NewTrafficHandler returns the handler.
func NewTrafficHandler(svc *traffic.Service) *TrafficHandler { return &TrafficHandler{svc: svc} }

// RegisterRoutes mounts /traffic under rg (rg already carries
// RequireUser middleware).
func (h *TrafficHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/traffic")
	g.GET("", h.Own)
}

// Own returns the authenticated user's per-ownership usage. The
// userID is read from the JWT subject; the path doesn't take one so
// users can't query other users.
func (h *TrafficHandler) Own(c *gin.Context) {
	claims := middleware.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)
	rows, err := h.svc.UsageForUser(c.Request.Context(), userID, from, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"clients": rows})
}
