package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/billing"
)

// PlanHandler serves /api/admin/plans/* and /api/admin/orders/*.
type PlanHandler struct {
	svc *billing.Service
}

// NewPlanHandler wires the handler.
func NewPlanHandler(svc *billing.Service) *PlanHandler { return &PlanHandler{svc: svc} }

// RegisterRoutes mounts both /plans and /orders under rg.
func (h *PlanHandler) RegisterRoutes(rg *gin.RouterGroup) {
	p := rg.Group("/plans")
	p.GET("", h.List)
	p.POST("", h.Create)
	p.PUT("/:id", h.Update)
	p.DELETE("/:id", h.Delete)

	o := rg.Group("/orders")
	o.GET("", h.ListOrders)
}

func (h *PlanHandler) List(c *gin.Context) {
	rows, err := h.svc.ListPlans(c.Request.Context(), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": rows})
}

func (h *PlanHandler) Create(c *gin.Context) {
	var p model.Plan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	created, err := h.svc.CreatePlan(c.Request.Context(), &p)
	if err != nil {
		if errors.Is(err, billing.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *PlanHandler) Update(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	var fields map[string]any
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	updated, err := h.svc.UpdatePlan(c.Request.Context(), id, fields)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *PlanHandler) Delete(c *gin.Context) {
	id, ok := parseInt64(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PlanHandler) ListOrders(c *gin.Context) {
	filter := repository.OrderFilter{}
	if v := c.Query("user_id"); v != "" {
		if id, err := parseInt64Q(v); err == nil {
			filter.UserID = &id
		}
	}
	if v := c.Query("status"); v != "" {
		filter.Status = &v
	}
	limit, offset := 50, 0
	if v := c.Query("limit"); v != "" {
		if n, err := parseInt64Q(v); err == nil {
			limit = int(n)
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := parseInt64Q(v); err == nil {
			offset = int(n)
		}
	}
	rows, err := h.svc.ListOrdersAdmin(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": rows, "limit": limit, "offset": offset})
}

func parseInt64Q(s string) (int64, error) {
	var v int64
	_, err := fmtSscan(s, &v)
	return v, err
}

// fmtSscan is a thin wrapper so we don't pull fmt into the package
// for one call. We accept just digits to keep this terse.
func fmtSscan(s string, out *int64) (int, error) {
	var n int64
	var neg bool
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, errors.New("invalid integer")
		}
		n = n*10 + int64(r-'0')
	}
	if neg {
		n = -n
	}
	*out = n
	return 1, nil
}
