package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/repository"
)

// AuditHandler serves /api/admin/audit-log: the admin-side
// audit-trail browser. Read-only.
type AuditHandler struct {
	repo *repository.AdminActionRepo
}

// NewAuditHandler wires the handler.
func NewAuditHandler(repo *repository.AdminActionRepo) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// RegisterRoutes mounts /audit-log under rg.
func (h *AuditHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/audit-log", h.List)
}

// List supports optional ?username= ?resource= ?id= ?method=
// filters + limit/offset pagination. Defaults to 100/0.
func (h *AuditHandler) List(c *gin.Context) {
	filter := repository.AdminActionFilter{}
	if v := c.Query("username"); v != "" {
		filter.AdminUsername = &v
	}
	if v := c.Query("resource"); v != "" {
		filter.TargetResource = &v
	}
	if v := c.Query("id"); v != "" {
		filter.TargetID = &v
	}
	if v := c.Query("method"); v != "" {
		filter.Method = &v
	}
	limit, offset := 100, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	rows, err := h.repo.List(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"actions": rows, "limit": limit, "offset": offset})
}
