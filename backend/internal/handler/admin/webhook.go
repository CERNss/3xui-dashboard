package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/webhook"
)

// WebhookHandler serves /api/admin/webhooks/*.
type WebhookHandler struct{ svc *webhook.Service }

// NewWebhookHandler wires the handler.
func NewWebhookHandler(svc *webhook.Service) *WebhookHandler { return &WebhookHandler{svc: svc} }

// RegisterRoutes mounts /webhooks under rg.
func (h *WebhookHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/webhooks")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/test", h.Test)
	g.GET("/:id/deliveries", h.ListDeliveries)
	g.POST("/deliveries/:deliveryID/replay", h.Replay)
}

func (h *WebhookHandler) List(c *gin.Context) {
	rows, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": rows})
}

func (h *WebhookHandler) Get(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	w, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if w == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var w model.Webhook
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := h.svc.Create(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, w)
}

func (h *WebhookHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var fields map[string]any
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WebhookHandler) Test(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	d, err := h.svc.SendTest(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, d)
}

func (h *WebhookHandler) ListDeliveries(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := h.svc.ListDeliveries(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deliveries": rows, "limit": limit, "offset": offset})
}

func (h *WebhookHandler) Replay(c *gin.Context) {
	id, ok := parseInt64(c, "deliveryID")
	if !ok {
		return
	}
	d, err := h.svc.Replay(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, d)
}
