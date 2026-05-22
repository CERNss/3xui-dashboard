package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	inboundsvc "github.com/cern/3xui-dashboard/internal/service/inbound"
)

// InboundHandler is kept only so older internal tests can construct
// it. The portal no longer exposes fleet inbounds: purchases resolve
// through plan-bound provisioning pools on the server.
type InboundHandler struct {
	svc *inboundsvc.Service
}

// NewInboundHandler wires the handler.
func NewInboundHandler(s *inboundsvc.Service) *InboundHandler {
	return &InboundHandler{svc: s}
}

// RegisterRoutes intentionally mounts no routes. User-facing inbound
// selection was removed when provisioning pools became the only
// purchase target source.
func (h *InboundHandler) RegisterRoutes(rg *gin.RouterGroup) {
	_ = h
	_ = rg
}

// portalInbound is the slim shape returned to the portal. Keep field
// names in sync with frontend/src/api/portal/billing.ts::PortalInbound.
type portalInbound struct {
	NodeID     int64  `json:"node_id"`
	NodeName   string `json:"node_name"`
	InboundTag string `json:"inbound_tag"`
	Protocol   string `json:"protocol"`
	Remark     string `json:"remark"`
	Port       int    `json:"port"`
}

// List returns every enabled inbound across the fleet, projected to
// the slim portal shape. Disabled inbounds are filtered out so users
// can't try to provision onto something that's been turned off.
//
// Per-node fetch failures are dropped silently for the user-facing
// surface — the portal doesn't need the same per-node-error map the
// admin view gets (an unreachable node just doesn't show up as a
// choice).
func (h *InboundHandler) List(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "user-facing inbound selection has been replaced by provisioning pools"})
}
