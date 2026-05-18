package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
)

// TrafficHandler serves /api/admin/traffic/*.
type TrafficHandler struct {
	svc       *traffic.Service
	ownership *repository.ClientOwnershipRepo
}

// NewTrafficHandler builds the handler.
func NewTrafficHandler(svc *traffic.Service, ownership *repository.ClientOwnershipRepo) *TrafficHandler {
	return &TrafficHandler{svc: svc, ownership: ownership}
}

// RegisterRoutes mounts every traffic endpoint under rg.
func (h *TrafficHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/traffic")
	g.GET("/clients/:userID", h.UserUsage)
	g.GET("/clients/:userID/history", h.UserHistory)
	g.POST("/reset/node/:nodeID", h.ResetNode)
	g.POST("/reset/node/:nodeID/inbound/:tag", h.ResetInbound)
	g.POST("/reset/node/:nodeID/inbound/:tag/client/:email", h.ResetClient)
}

func (h *TrafficHandler) UserUsage(c *gin.Context) {
	userID, ok := parseInt64(c, "userID")
	if !ok {
		return
	}
	from, to := parseRange(c, 7*24*time.Hour)
	rows, err := h.svc.UsageForUser(c.Request.Context(), userID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"from":    from.Unix(),
		"to":      to.Unix(),
		"clients": rows,
	})
}

func (h *TrafficHandler) UserHistory(c *gin.Context) {
	userID, ok := parseInt64(c, "userID")
	if !ok {
		return
	}
	from, to := parseRange(c, 24*time.Hour)
	bucket := parseDuration(c, "bucket", 5*time.Minute)

	owns, err := h.ownership.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type series struct {
		Ownership int64                  `json:"ownership_id"`
		Points    []traffic.BucketPoint  `json:"points"`
	}
	resp := make([]series, 0, len(owns))
	for i := range owns {
		pts, err := h.svc.HistoryForOwnership(c.Request.Context(), &owns[i], from, to, bucket)
		if err != nil {
			continue
		}
		resp = append(resp, series{Ownership: owns[i].ID, Points: pts})
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"from":    from.Unix(),
		"to":      to.Unix(),
		"bucket":  bucket.String(),
		"series":  resp,
	})
}

func (h *TrafficHandler) ResetClient(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	email := c.Param("email")
	if err := h.svc.ResetClient(c.Request.Context(), nodeID, tag, email); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TrafficHandler) ResetInbound(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	if err := h.svc.ResetInbound(c.Request.Context(), nodeID, tag); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TrafficHandler) ResetNode(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	if err := h.svc.ResetNode(c.Request.Context(), nodeID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func parseRange(c *gin.Context, defaultSpan time.Duration) (time.Time, time.Time) {
	now := time.Now().UTC()
	from := parseTimeQuery(c, "from", now.Add(-defaultSpan))
	to := parseTimeQuery(c, "to", now)
	return from, to
}

func parseDuration(c *gin.Context, key string, fallback time.Duration) time.Duration {
	if v := c.Query(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// keep go happy on unused imports if a method gets removed
var _ = strconv.Itoa
