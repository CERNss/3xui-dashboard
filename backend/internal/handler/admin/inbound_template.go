package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/billing"
)

// InboundTemplateHandler serves /api/admin/inbound-templates.
type InboundTemplateHandler struct {
	svc *billing.Service
}

func NewInboundTemplateHandler(svc *billing.Service) *InboundTemplateHandler {
	return &InboundTemplateHandler{svc: svc}
}

func (h *InboundTemplateHandler) RegisterRoutes(rg *gin.RouterGroup) {
	t := rg.Group("/inbound-templates")
	t.GET("", h.List)
	t.POST("", h.Create)
	t.PUT("/:id", h.Update)
	t.DELETE("/:id", h.Delete)
}

func (h *InboundTemplateHandler) List(c *gin.Context) {
	rows, err := h.svc.ListInboundTemplates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": rows})
}

func (h *InboundTemplateHandler) Create(c *gin.Context) {
	var t model.InboundTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	created, err := h.svc.CreateInboundTemplate(c.Request.Context(), &t)
	h.writeMutation(c, created, err, http.StatusCreated)
}

func (h *InboundTemplateHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var fields map[string]any
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	updated, err := h.svc.UpdateInboundTemplate(c.Request.Context(), id, fields)
	h.writeMutation(c, updated, err, http.StatusOK)
}

func (h *InboundTemplateHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteInboundTemplate(c.Request.Context(), id); err != nil {
		h.writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *InboundTemplateHandler) writeMutation(c *gin.Context, body any, err error, okStatus int) {
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(okStatus, body)
}

func (h *InboundTemplateHandler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, billing.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
