package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/service/node"
)

// NodeHandler serves /api/admin/nodes/*.
type NodeHandler struct {
	svc *node.Service
}

// NewNodeHandler wires the handler to the node service.
func NewNodeHandler(svc *node.Service) *NodeHandler { return &NodeHandler{svc: svc} }

// RegisterRoutes mounts every handler under the supplied admin
// router group (which already carries RequireAdmin middleware).
func (h *NodeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/nodes")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/enable", h.Enable)
	g.POST("/:id/disable", h.Disable)
	g.POST("/:id/probe", h.Probe)
	g.GET("/:id/metrics", h.Metrics)
}

// ---- handlers -------------------------------------------------------------

func (h *NodeHandler) List(c *gin.Context) {
	nodes, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

func (h *NodeHandler) Get(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	n, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *NodeHandler) Create(c *gin.Context) {
	var in node.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	created, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		h.serviceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *NodeHandler) Update(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	var in node.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		h.serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *NodeHandler) Delete(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.serviceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NodeHandler) Enable(c *gin.Context)  { h.toggleEnable(c, true) }
func (h *NodeHandler) Disable(c *gin.Context) { h.toggleEnable(c, false) }

func (h *NodeHandler) toggleEnable(c *gin.Context, enable bool) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if err := h.svc.SetEnabled(c.Request.Context(), id, enable); err != nil {
		h.serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "enabled": enable})
}

func (h *NodeHandler) Probe(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	res, err := h.svc.Probe(c.Request.Context(), id)
	if err != nil {
		// Probe failure isn't a server error per se — it means the
		// admin needs to look at why the node is unreachable.
		c.JSON(http.StatusBadGateway, gin.H{
			"id":    id,
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           id,
		"prior_status": res.PriorStatus,
		"status":       res.Status,
	})
}

func (h *NodeHandler) Metrics(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	from := parseTimeQuery(c, "from", time.Now().Add(-3*time.Hour))
	to := parseTimeQuery(c, "to", time.Now())
	bucket := parseDurationQuery(c, "bucket", 0)

	var points []node.MetricSample
	if bucket > 0 {
		points = h.svc.MetricsBucketed(id, from, to, bucket)
	} else {
		points = h.svc.MetricsRaw(id, from, to)
	}
	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"from":   from.Unix(),
		"to":     to.Unix(),
		"bucket": bucket.String(),
		"points": points,
	})
}

// ---- error mapping --------------------------------------------------------

func (h *NodeHandler) parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be an integer"})
		return 0, false
	}
	return id, true
}

func (h *NodeHandler) serviceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, node.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, node.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
	case errors.Is(err, node.ErrDuplicateName):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		h.serverError(c, err)
	}
}

func (h *NodeHandler) serverError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// ---- query parsing --------------------------------------------------------

func parseTimeQuery(c *gin.Context, key string, fallback time.Time) time.Time {
	s := c.Query(key)
	if s == "" {
		return fallback
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return fallback
}

func parseDurationQuery(c *gin.Context, key string, fallback time.Duration) time.Duration {
	s := c.Query(key)
	if s == "" {
		return fallback
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return fallback
}
