package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/billing"
)

// ProvisioningPoolHandler serves /api/admin/provisioning-pools.
type ProvisioningPoolHandler struct {
	svc *billing.Service
}

func NewProvisioningPoolHandler(svc *billing.Service) *ProvisioningPoolHandler {
	return &ProvisioningPoolHandler{svc: svc}
}

func (h *ProvisioningPoolHandler) RegisterRoutes(rg *gin.RouterGroup) {
	p := rg.Group("/provisioning-pools")
	p.GET("", h.List)
	p.POST("", h.Create)
	p.PUT("/:id", h.Update)
	p.DELETE("/:id", h.Delete)
	p.POST("/:id/targets", h.CreateTarget)
	p.PUT("/targets/:targetID", h.UpdateTarget)
	p.DELETE("/targets/:targetID", h.DeleteTarget)
}

func (h *ProvisioningPoolHandler) List(c *gin.Context) {
	rows, err := h.svc.ListProvisioningPools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pools": rows})
}

func (h *ProvisioningPoolHandler) Create(c *gin.Context) {
	var p model.ProvisioningPool
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	created, err := h.svc.CreateProvisioningPool(c.Request.Context(), &p)
	h.writeMutation(c, created, err, http.StatusCreated)
}

func (h *ProvisioningPoolHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var fields map[string]any
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	updated, err := h.svc.UpdateProvisioningPool(c.Request.Context(), id, fields)
	h.writeMutation(c, updated, err, http.StatusOK)
}

func (h *ProvisioningPoolHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteProvisioningPool(c.Request.Context(), id); err != nil {
		h.writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProvisioningPoolHandler) CreateTarget(c *gin.Context) {
	poolID, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var target model.ProvisioningPoolTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	target.PoolID = poolID
	created, err := h.svc.CreateProvisioningTarget(c.Request.Context(), &target)
	h.writeMutation(c, created, err, http.StatusCreated)
}

func (h *ProvisioningPoolHandler) UpdateTarget(c *gin.Context) {
	id, ok := parseInt64(c, "targetID")
	if !ok {
		return
	}
	var fields map[string]any
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := h.svc.UpdateProvisioningTarget(c.Request.Context(), id, fields); err != nil {
		h.writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProvisioningPoolHandler) DeleteTarget(c *gin.Context) {
	id, ok := parseInt64(c, "targetID")
	if !ok {
		return
	}
	if err := h.svc.DeleteProvisioningTarget(c.Request.Context(), id); err != nil {
		h.writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProvisioningPoolHandler) writeMutation(c *gin.Context, body any, err error, okStatus int) {
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(okStatus, body)
}

func (h *ProvisioningPoolHandler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, billing.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
