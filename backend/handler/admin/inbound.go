package admin

import (
	"net/http"
	"strconv"

	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
)

// InboundHandler proxies inbound CRUD operations to 3x-ui.
type InboundHandler struct {
	xui *xui.Client
}

// NewInboundHandler constructs an InboundHandler.
func NewInboundHandler(c *xui.Client) *InboundHandler {
	return &InboundHandler{xui: c}
}

// List handles GET /api/admin/inbounds.
func (h *InboundHandler) List(ctx *gin.Context) {
	data, err := h.xui.ListInbounds()
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Create handles POST /api/admin/inbounds.
func (h *InboundHandler) Create(ctx *gin.Context) {
	var payload map[string]interface{}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := h.xui.AddInbound(payload)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Update handles PUT /api/admin/inbounds/:id.
func (h *InboundHandler) Update(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var payload map[string]interface{}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := h.xui.UpdateInbound(id, payload)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Delete handles DELETE /api/admin/inbounds/:id.
func (h *InboundHandler) Delete(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	data, err := h.xui.DeleteInbound(id)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}
