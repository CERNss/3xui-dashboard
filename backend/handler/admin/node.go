package admin

import (
	"net/http"

	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
)

// NodeHandler proxies node operations to 3x-ui.
type NodeHandler struct {
	xui *xui.Client
}

// NewNodeHandler constructs a NodeHandler.
func NewNodeHandler(c *xui.Client) *NodeHandler {
	return &NodeHandler{xui: c}
}

// List handles GET /api/admin/nodes.
func (h *NodeHandler) List(ctx *gin.Context) {
	data, err := h.xui.ListNodes()
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.Data(http.StatusOK, "application/json", data)
}
