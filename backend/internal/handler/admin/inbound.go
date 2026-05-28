package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/inbound"
)

// TemplateLookup is the minimum surface InboundHandler needs to
// resolve an optional `template_id` on the create-inbound POST body.
// Satisfied by repository.ProvisioningPoolRepo.
type TemplateLookup interface {
	GetTemplate(ctx context.Context, id int64) (*model.InboundTemplate, error)
}

// InboundHandler serves /api/admin/inbounds/*.
type InboundHandler struct {
	svc       *inbound.Service
	templates TemplateLookup
}

// NewInboundHandler wires the handler to the inbound service.
func NewInboundHandler(svc *inbound.Service) *InboundHandler { return &InboundHandler{svc: svc} }

// SetTemplateLookup attaches a template repository so the Create
// handler can resolve POST bodies that pass `template_id` instead of
// providing the full inbound wire shape. Left nil = template_id is
// silently ignored.
func (h *InboundHandler) SetTemplateLookup(t TemplateLookup) { h.templates = t }

// RegisterRoutes mounts every inbound endpoint under the supplied
// admin router group.
func (h *InboundHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/inbounds")
	g.GET("", h.ListAll)
	g.GET("/nodes/:nodeID", h.ListOnNode)
	g.POST("/nodes/:nodeID", h.Create)
	g.GET("/nodes/:nodeID/:tag", h.Get)
	g.PUT("/nodes/:nodeID/:tag", h.Update)
	g.DELETE("/nodes/:nodeID/:tag", h.Delete)
	g.POST("/nodes/:nodeID/:tag/enable", h.Enable)
	g.POST("/nodes/:nodeID/:tag/disable", h.Disable)
}

// ---- handlers -------------------------------------------------------------

func (h *InboundHandler) ListAll(c *gin.Context) {
	res, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *InboundHandler) ListOnNode(c *gin.Context) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	inbounds, err := h.svc.List(c.Request.Context(), nodeID)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"node_id": nodeID, "inbounds": inbounds})
}

func (h *InboundHandler) Get(c *gin.Context) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")
	in, err := h.svc.Get(c.Request.Context(), nodeID, tag)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func (h *InboundHandler) Create(c *gin.Context) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	// The body is `runtime.Inbound` for backward compat, plus an
	// optional `template_id` sidecar field. When template_id is set,
	// we materialize a runtime.Inbound from the template + body's
	// port/tag/listen/remark and ignore protocol/settings/stream
	// settings/sniffing from the body (template wins).
	var body struct {
		runtime.Inbound
		TemplateID *int64 `json:"template_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	in := body.Inbound
	if body.TemplateID != nil && *body.TemplateID > 0 {
		if h.templates == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "template lookup not wired on this deployment"})
			return
		}
		tpl, err := h.templates.GetTemplate(c.Request.Context(), *body.TemplateID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "template lookup: " + err.Error()})
			return
		}
		if tpl == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
			return
		}
		if !tpl.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "template disabled"})
			return
		}
		if in.Port <= 0 || in.Port > 65535 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "port is required (1..65535) when creating from a template"})
			return
		}
		built := inbound.BuildTemplateInbound(tpl, in.Port, in.Tag)
		// Body overrides for fields the operator may want to tweak at
		// create-time even when starting from a template.
		if in.Listen != "" {
			built.Listen = in.Listen
		}
		if in.Remark != "" {
			built.Remark = in.Remark
		}
		in = *built
	}
	created, err := h.svc.Add(c.Request.Context(), nodeID, &in)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *InboundHandler) Update(c *gin.Context) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")
	var body runtime.Inbound
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), nodeID, tag, &body)
	if err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *InboundHandler) Delete(c *gin.Context) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")
	if err := h.svc.Delete(c.Request.Context(), nodeID, tag); err != nil {
		h.upstreamError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *InboundHandler) Enable(c *gin.Context)  { h.toggleEnable(c, true) }
func (h *InboundHandler) Disable(c *gin.Context) { h.toggleEnable(c, false) }

func (h *InboundHandler) toggleEnable(c *gin.Context, enable bool) {
	nodeID, ok := h.parseNodeID(c)
	if !ok {
		return
	}
	tag := c.Param("tag")
	if err := h.svc.SetEnable(c.Request.Context(), nodeID, tag, enable); err != nil {
		h.upstreamError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"node_id": nodeID, "tag": tag, "enabled": enable})
}

// ---- helpers --------------------------------------------------------------

func (h *InboundHandler) parseNodeID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("nodeID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nodeID must be an integer"})
		return 0, false
	}
	return id, true
}

func (h *InboundHandler) upstreamError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, runtime.ErrNodeNotFound), errors.Is(err, runtime.ErrTagNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, runtime.ErrNodeDisabled):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	}
}
