// Package public holds handlers for routes that do not require any
// auth — currently just the central /sub/* subscription endpoints.
package public

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/sub"
)

// Format names the supported subscription output formats.
type Format string

const (
	FormatBase64  Format = "base64"
	FormatJSON    Format = "json"
	FormatClash   Format = "clash"
	FormatSingBox Format = "singbox"
	FormatSIP008  Format = "sip008"
)

// SubHandler serves /sub/*.
type SubHandler struct {
	asm       *sub.Assembler
	settings  *repository.SettingRepo
	remarkFmt string
	log       *slog.Logger
}

// NewSubHandler returns a handler. settings may be nil — when nil,
// format calls fall back to embedded default templates and the
// strategy/rule knobs use compile-time defaults.
func NewSubHandler(a *sub.Assembler, settings *repository.SettingRepo, remarkFmt string, lg *slog.Logger) *SubHandler {
	if remarkFmt == "" {
		remarkFmt = "-ieo"
	}
	if lg == nil {
		lg = slog.Default()
	}
	return &SubHandler{
		asm:       a,
		settings:  settings,
		remarkFmt: remarkFmt,
		log:       lg.With(slog.String("component", "handler.public.sub")),
	}
}

// RegisterRoutes mounts /sub/* on the supplied engine (no auth).
//
// Two access patterns supported:
//   /sub/:subId          — format selected by ?format= or User-Agent
//   /sub/<format>/:subId — explicit format in the path (legacy + clarity)
func (h *SubHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/sub/:subId", h.Auto)
	// Explicit-format routes for direct linking.
	r.GET("/sub/json/:subId", h.bind(FormatJSON))
	r.GET("/sub/clash/:subId", h.bind(FormatClash))
	r.GET("/sub/singbox/:subId", h.bind(FormatSingBox))
	r.GET("/sub/sip008/:subId", h.bind(FormatSIP008))
}

// Auto picks the format from ?format= or User-Agent and dispatches.
func (h *SubHandler) Auto(c *gin.Context) {
	f := detectFormat(c.Query("format"), c.GetHeader("User-Agent"))
	h.serve(c, f)
}

// bind returns a handler that always serves the given format.
func (h *SubHandler) bind(f Format) gin.HandlerFunc {
	return func(c *gin.Context) { h.serve(c, f) }
}

func (h *SubHandler) serve(c *gin.Context, f Format) {
	subID := c.Param("subId")
	data, err := h.asm.Build(c.Request.Context(), subID, h.remarkFmt)
	if err != nil {
		h.errorResponse(c, err)
		return
	}
	c.Header("Subscription-Userinfo", h.asm.UserInfoHeader(data))
	c.Header("Profile-Update-Interval", "12")

	switch f {
	case FormatBase64:
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, h.asm.FormatBase64(data))
	case FormatJSON:
		body, err := h.asm.FormatJSON(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	case FormatClash:
		opts := h.loadFormatOpts(c.Request.Context())
		body, err := h.asm.FormatClash(data, opts)
		if err != nil {
			h.log.Error("FormatClash failed, falling back to default", "err", err)
			// Last-ditch fallback — call again with zero opts to ignore
			// the (broken) operator template.
			opts.ClashTemplate = ""
			body, err = h.asm.FormatClash(data, opts)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	case FormatSingBox:
		opts := h.loadFormatOpts(c.Request.Context())
		body, err := h.asm.FormatSingBox(data, opts)
		if err != nil {
			h.log.Error("FormatSingBox failed, falling back to default", "err", err)
			opts.SingBoxTemplate = ""
			body, err = h.asm.FormatSingBox(data, opts)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	case FormatSIP008:
		body, err := h.asm.FormatSIP008(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "unsupported format; valid: base64, json, clash, singbox, sip008",
		})
	}
}

// detectFormat picks the response format from either an explicit query
// param or the User-Agent. `?format=` always wins; UA is the fallback.
//
// Recognized UA needles (case-insensitive):
//   clash, mihomo, stash        → clash
//   sing-box, singbox           → singbox
//   shadowsocks                 → sip008
//   anything else               → base64 (preserves legacy default)
func detectFormat(qs, ua string) Format {
	if qs != "" {
		switch strings.ToLower(qs) {
		case "base64":
			return FormatBase64
		case "json":
			return FormatJSON
		case "clash":
			return FormatClash
		case "singbox", "sing-box":
			return FormatSingBox
		case "sip008":
			return FormatSIP008
		default:
			return Format(qs) // pass through; serve() returns 400
		}
	}
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "clash"),
		strings.Contains(l, "mihomo"),
		strings.Contains(l, "stash"):
		return FormatClash
	case strings.Contains(l, "sing-box"),
		strings.Contains(l, "singbox"):
		return FormatSingBox
	case strings.Contains(l, "shadowsocks"):
		return FormatSIP008
	default:
		return FormatBase64
	}
}

// loadFormatOpts populates FormatOpts from the settings repo. Missing
// keys leave the corresponding field at its zero value so the template
// engine uses its embedded default.
func (h *SubHandler) loadFormatOpts(ctx context.Context) sub.FormatOpts {
	opts := sub.FormatOpts{RuleProvidersEnabled: true} // default ON
	if h.settings == nil {
		return opts
	}
	if v, ok, _ := h.settings.Get(ctx, model.SettingClashTemplateYAML); ok {
		opts.ClashTemplate = v
	}
	if v, ok, _ := h.settings.Get(ctx, model.SettingSingBoxTemplateJSON); ok {
		opts.SingBoxTemplate = v
	}
	if v, ok, _ := h.settings.Get(ctx, model.SettingProxyGroupStrategy); ok && v != "" {
		opts.ProxyGroupStrategy = v
	}
	if v, ok, _ := h.settings.Get(ctx, model.SettingRuleProvidersEnabled); ok {
		// only "false" turns it off; any other string (or absent) means on
		if strings.EqualFold(v, "false") || v == "0" {
			opts.RuleProvidersEnabled = false
		}
	}
	return opts
}

func (h *SubHandler) errorResponse(c *gin.Context, err error) {
	if errors.Is(err, sub.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
