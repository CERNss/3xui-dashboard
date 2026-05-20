package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	inboundsvc "github.com/cern/3xui-dashboard/internal/service/inbound"
)

// InboundHandler exposes a slim, user-safe view of the fleet's
// inbounds so the portal can render a "where to provision" picker
// during plan purchase. Admin-only fields (settings JSON, traffic
// counters, client list) are stripped — users only see what they
// need to choose a node.
type InboundHandler struct {
	svc *inboundsvc.Service
}

// NewInboundHandler wires the handler.
func NewInboundHandler(s *inboundsvc.Service) *InboundHandler {
	return &InboundHandler{svc: s}
}

// RegisterRoutes mounts /inbounds under the user group.
func (h *InboundHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/inbounds", h.List)
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
	res, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]portalInbound, 0, len(res.Inbounds))
	for _, fi := range res.Inbounds {
		if !fi.Inbound.Enable {
			continue
		}
		out = append(out, portalInbound{
			NodeID:     fi.NodeID,
			NodeName:   fi.NodeName,
			InboundTag: fi.Inbound.Tag,
			Protocol:   fi.Inbound.Protocol,
			Remark:     fi.Inbound.Remark,
			Port:       fi.Inbound.Port,
		})
	}
	c.JSON(http.StatusOK, gin.H{"inbounds": out})
}
