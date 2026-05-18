package admin

import (
	"net/http"

	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
)

// ClientHandler proxies client CRUD operations to 3x-ui.
type ClientHandler struct {
	xui *xui.Client
}

// NewClientHandler constructs a ClientHandler.
func NewClientHandler(c *xui.Client) *ClientHandler {
	return &ClientHandler{xui: c}
}

// List handles GET /api/admin/clients.
// Returns all inbounds (clients are embedded in inbound settings).
func (h *ClientHandler) List(ctx *gin.Context) {
	data, err := h.xui.ListInbounds()
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Create handles POST /api/admin/clients.
// Expects { inboundId, settings: { clients: [...] } }
func (h *ClientHandler) Create(ctx *gin.Context) {
	var payload map[string]interface{}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := h.xui.AddClient(payload)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Update handles PUT /api/admin/clients/:uuid.
func (h *ClientHandler) Update(ctx *gin.Context) {
	uuid := ctx.Param("uuid")
	var payload map[string]interface{}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := h.xui.UpdateClient(uuid, payload)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}

// Delete handles DELETE /api/admin/clients/:uuid.
func (h *ClientHandler) Delete(ctx *gin.Context) {
	uuid := ctx.Param("uuid")
	data, err := h.xui.DeleteClient(uuid)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}
