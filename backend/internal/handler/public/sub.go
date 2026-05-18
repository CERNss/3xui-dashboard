// Package public holds handlers for routes that do not require any
// auth — currently just the central /sub/* subscription endpoints.
package public

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/sub"
)

// SubHandler serves /sub/*.
type SubHandler struct {
	asm       *sub.Assembler
	remarkFmt string
}

// NewSubHandler returns a handler with the optional remark-format
// string (typically read from the settings table; default "-ieo").
func NewSubHandler(a *sub.Assembler, remarkFmt string) *SubHandler {
	if remarkFmt == "" {
		remarkFmt = "-ieo"
	}
	return &SubHandler{asm: a, remarkFmt: remarkFmt}
}

// RegisterRoutes mounts /sub/* on the supplied engine (no auth).
func (h *SubHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/sub/:subId", h.Base64)
	r.GET("/sub/json/:subId", h.JSON)
}

// Base64 returns the newline-joined-then-base64-encoded form every
// Xray-family client understands.
func (h *SubHandler) Base64(c *gin.Context) {
	subID := c.Param("subId")
	data, err := h.asm.Build(c.Request.Context(), subID, h.remarkFmt)
	if err != nil {
		h.errorResponse(c, err)
		return
	}
	body := h.asm.FormatBase64(data)
	c.Header("Subscription-Userinfo", h.asm.UserInfoHeader(data))
	c.Header("Profile-Update-Interval", "12")
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}

// JSON returns the structured list of links.
func (h *SubHandler) JSON(c *gin.Context) {
	subID := c.Param("subId")
	data, err := h.asm.Build(c.Request.Context(), subID, h.remarkFmt)
	if err != nil {
		h.errorResponse(c, err)
		return
	}
	body, err := h.asm.FormatJSON(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Subscription-Userinfo", h.asm.UserInfoHeader(data))
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(body)
}

func (h *SubHandler) errorResponse(c *gin.Context, err error) {
	if errors.Is(err, sub.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
