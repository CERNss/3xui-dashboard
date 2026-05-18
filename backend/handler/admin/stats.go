package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
)

// StatsHandler returns aggregate stats from 3x-ui.
type StatsHandler struct {
	xui *xui.Client
}

// NewStatsHandler constructs a StatsHandler.
func NewStatsHandler(c *xui.Client) *StatsHandler {
	return &StatsHandler{xui: c}
}

type inboundSummary struct {
	Id          int    `json:"id"`
	Enable      bool   `json:"enable"`
	ExpiryTime  int64  `json:"expiryTime"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
	ClientStats []struct {
		Enable     bool  `json:"enable"`
		ExpiryTime int64 `json:"expiryTime"`
	} `json:"clientStats"`
}

type statsResponse struct {
	TotalClients  int   `json:"totalClients"`
	ActiveClients int   `json:"activeClients"`
	TotalUp       int64 `json:"totalUp"`
	TotalDown     int64 `json:"totalDown"`
	NodesOnline   int   `json:"nodesOnline"`
}

// Get handles GET /api/admin/stats.
func (h *StatsHandler) Get(ctx *gin.Context) {
	inboundsRaw, err := h.xui.ListInbounds()
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	var inbounds []inboundSummary
	_ = json.Unmarshal(inboundsRaw, &inbounds)

	now := time.Now().UnixMilli()
	stats := statsResponse{}
	for _, ib := range inbounds {
		for _, cs := range ib.ClientStats {
			stats.TotalClients++
			notExpired := cs.ExpiryTime == 0 || cs.ExpiryTime > now
			if cs.Enable && notExpired {
				stats.ActiveClients++
			}
		}
		stats.TotalUp += ib.Up
		stats.TotalDown += ib.Down
	}

	// Count online nodes
	nodesRaw, err := h.xui.ListNodes()
	if err == nil {
		var nodes []struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(nodesRaw, &nodes) == nil {
			for _, n := range nodes {
				if n.Status == "online" {
					stats.NodesOnline++
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, stats)
}
