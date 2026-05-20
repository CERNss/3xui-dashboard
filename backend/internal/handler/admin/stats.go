package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/repository"
)

// StatsHandler returns the admin overview aggregates as a single
// payload. It replaces the previous client-side fan-out (3 list
// endpoints with limit=1000 each) so the page scales past the cap
// and stops paying for rows it never reads.
type StatsHandler struct {
	repo *repository.StatsRepo
}

// NewStatsHandler binds the handler to the stats repo.
func NewStatsHandler(repo *repository.StatsRepo) *StatsHandler {
	return &StatsHandler{repo: repo}
}

// RegisterRoutes wires GET /stats under the authed admin group.
func (h *StatsHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/stats", h.get)
}

// StatsResponse is the wire shape — keep it stable, the frontend
// destructures every field by name.
type StatsResponse struct {
	Users        repository.UserStats    `json:"users"`
	Plans        repository.PlanStats    `json:"plans"`
	Orders       repository.OrderStats   `json:"orders"`
	RecentOrders []repository.RecentOrder `json:"recent_orders"`
}

func (h *StatsHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	// "This month" uses UTC midnight on the 1st so monthly revenue
	// resets at the same instant for everyone, not based on the
	// server's local timezone.
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	users, err := h.repo.Users(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	plans, err := h.repo.Plans(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	orders, err := h.repo.Orders(ctx, monthStart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recent, err := h.repo.RecentOrders(ctx, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, StatsResponse{
		Users:        users,
		Plans:        plans,
		Orders:       orders,
		RecentOrders: recent,
	})
}
