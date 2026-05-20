package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/runtime"
	clientsvc "github.com/cern/3xui-dashboard/internal/service/client"
)

// ClientHandler serves /api/admin/clients/*.
type ClientHandler struct{ svc *clientsvc.Service }

// NewClientHandler wires the handler to the client service.
func NewClientHandler(svc *clientsvc.Service) *ClientHandler { return &ClientHandler{svc: svc} }

// RegisterRoutes mounts every client endpoint under rg.
func (h *ClientHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/clients")
	g.GET("/nodes/:nodeID/inbounds/:tag", h.ListOnInbound)
	g.POST("/nodes/:nodeID/inbounds/:tag/provision", h.Provision)
	g.POST("/nodes/:nodeID/inbounds/:tag/add", h.AddDirect)
	g.PUT("/nodes/:nodeID/inbounds/:tag/clients/:email", h.UpdateDirect)
	g.DELETE("/nodes/:nodeID/inbounds/:tag/clients/:email", h.Delete)
	g.POST("/nodes/:nodeID/inbounds/:tag/clients/:email/link", h.Link)
	g.POST("/nodes/:nodeID/inbounds/:tag/clients/:email/unlink", h.Unlink)

	// Per-node lookups for online status + last-online — frontend uses
	// these when expanding an inbound row to annotate per-client state.
	g.GET("/nodes/:nodeID/snapshot", h.Snapshot)
}

// ---- handlers -------------------------------------------------------------

func (h *ClientHandler) ListOnInbound(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	rows, err := h.svc.ListOnInbound(c.Request.Context(), nodeID, tag)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"node_id": nodeID, "inbound_tag": tag, "clients": rows})
}

type provisionRequest struct {
	UserID            int64  `json:"user_id" binding:"required"`
	PlanID            *int64 `json:"plan_id,omitempty"`
	DurationDays      int    `json:"duration_days"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
}

func (h *ClientHandler) Provision(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")

	var body provisionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	row, err := h.svc.ProvisionClient(c.Request.Context(), body.UserID, nodeID, tag, clientsvc.PlanParams{
		PlanID:            body.PlanID,
		DurationDays:      body.DurationDays,
		TrafficLimitBytes: body.TrafficLimitBytes,
	})
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *ClientHandler) Delete(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	email := c.Param("email")
	if err := h.svc.DeleteClient(c.Request.Context(), nodeID, tag, email); err != nil {
		h.upstreamError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type linkRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	PlanID *int64 `json:"plan_id,omitempty"`
}

type addDirectRequest struct {
	Client runtime.Client `json:"client"`
	UserID int64          `json:"user_id"` // optional — links to ownership row
}

// AddDirect creates a panel-side client without going through plan
// purchase. Admin controls UUID/password/flow/limits/expiry directly.
func (h *ClientHandler) AddDirect(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	var body addDirectRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if body.Client.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client.email is required"})
		return
	}
	out, err := h.svc.AddClientDirect(c.Request.Context(), nodeID, tag, body.Client, body.UserID)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// UpdateDirect edits an existing panel client identified by email.
// Body shape: full Client struct (only mutated fields matter on the
// 3x-ui side, but we forward the whole thing for simplicity).
func (h *ClientHandler) UpdateDirect(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	email := c.Param("email")
	var client runtime.Client
	if err := c.ShouldBindJSON(&client); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if client.Email == "" {
		client.Email = email
	}
	if client.Email != email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client.email does not match path email"})
		return
	}
	out, err := h.svc.UpdateClientDirect(c.Request.Context(), nodeID, tag, client)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// Snapshot returns inbounds + online emails + last-online map for a
// single node. Frontend uses this when rendering the per-client
// online badges on an expanded inbound row.
func (h *ClientHandler) Snapshot(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	snap, err := h.svc.FetchSnapshot(c.Request.Context(), nodeID)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, snap)
}

func (h *ClientHandler) Link(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	email := c.Param("email")

	var body linkRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	row, err := h.svc.LinkToUser(c.Request.Context(), nodeID, tag, email, body.UserID, body.PlanID)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *ClientHandler) Unlink(c *gin.Context) {
	nodeID, ok := parseInt64(c, "nodeID")
	if !ok {
		return
	}
	tag := c.Param("tag")
	email := c.Param("email")
	if err := h.svc.UnlinkUser(c.Request.Context(), nodeID, tag, email); err != nil {
		h.upstreamError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- helpers --------------------------------------------------------------

func parseInt64(c *gin.Context, key string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " must be an integer"})
		return 0, false
	}
	return v, true
}

func (h *ClientHandler) upstreamError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, clientsvc.ErrUserNotFound),
		errors.Is(err, clientsvc.ErrPlanNotFound),
		errors.Is(err, runtime.ErrTagNotFound),
		errors.Is(err, runtime.ErrClientNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, runtime.ErrNodeNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
	case errors.Is(err, runtime.ErrNodeDisabled):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	}
}
