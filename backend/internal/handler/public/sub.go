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
	"github.com/cern/3xui-dashboard/internal/sub/policy"
	"github.com/cern/3xui-dashboard/internal/sub/ruleset"
)

// Format names the supported subscription output formats.
type Format string

const (
	FormatBase64    Format = "base64"
	FormatJSON      Format = "json"
	FormatClash     Format = "clash"
	FormatSingBox   Format = "singbox"
	FormatSurge     Format = "surge"
	FormatSIP008    Format = "sip008"
	FormatWireGuard Format = "wireguard"
	FormatWGZip     Format = "wireguard-zip"
)

// SubHandler serves /sub/*.
type SubHandler struct {
	asm           *sub.Assembler
	settings      settingsReader
	profiles      *repository.SubscriptionProfileRepo
	rulesets      *repository.SubscriptionRulesetRepo
	rulesetCache  *ruleset.Cache
	remarkFmt     string
	publicBaseURL string
	log           *slog.Logger
}

type settingsReader interface {
	Get(context.Context, string) (string, bool, error)
}

// NewSubHandler returns a handler. settings may be nil — when nil,
// format calls fall back to embedded default templates and the
// strategy/rule knobs use compile-time defaults.
func NewSubHandler(a *sub.Assembler, settings settingsReader, remarkFmt, publicBaseURL string, lg *slog.Logger) *SubHandler {
	if remarkFmt == "" {
		remarkFmt = "-ieo"
	}
	if lg == nil {
		lg = slog.Default()
	}
	return &SubHandler{
		asm:           a,
		settings:      settings,
		remarkFmt:     remarkFmt,
		publicBaseURL: normalizePublicBaseURL(publicBaseURL),
		log:           lg.With(slog.String("component", "handler.public.sub")),
	}
}

// SetProfileStore wires the DB-backed subscription profile + ruleset
// repos and the ruleset fetch cache. When unset (e.g. in unit tests),
// the handler falls back to the built-in default profile so rendering
// still works.
func (h *SubHandler) SetProfileStore(profiles *repository.SubscriptionProfileRepo, rulesets *repository.SubscriptionRulesetRepo, cache *ruleset.Cache) {
	h.profiles = profiles
	h.rulesets = rulesets
	h.rulesetCache = cache
}

// resolveProfile picks the routing profile + its rulesets for a request:
// ?profile=<key> if present and found, else the DB default, else the
// built-in code default. Rulesets fall back to the built-in set when
// none are configured.
func (h *SubHandler) resolveProfile(ctx context.Context, key string) (model.SubscriptionProfile, []model.SubscriptionRuleset) {
	if h.profiles == nil {
		return policy.DefaultProfile(), policy.DefaultRulesets()
	}
	var p *model.SubscriptionProfile
	if key != "" {
		p, _ = h.profiles.GetByKey(ctx, key)
	}
	if p == nil {
		p, _ = h.profiles.GetDefault(ctx)
	}
	if p == nil {
		return policy.DefaultProfile(), policy.DefaultRulesets()
	}
	var rs []model.SubscriptionRuleset
	if h.rulesets != nil {
		rs, _ = h.rulesets.List(ctx)
	}
	if len(rs) == 0 {
		rs = policy.DefaultRulesets()
	}
	return *p, rs
}

// RegisterRoutes mounts /sub/* on the supplied engine (no auth).
//
// Two access patterns supported:
//
//	/sub/:subId          — format selected by ?format= or User-Agent
//	/sub/<format>/:subId — explicit format in the path
//
// `limiter` is an optional per-IP rate-limit middleware. nil falls
// back to no limit (test fixtures); production wires the same
// middleware.IPRateLimiter the login endpoint uses, since a
// successful sub fetch hands out the user's WG private keys + the
// full traffic snapshot — abuse surface that warrants throttling.
func (h *SubHandler) RegisterRoutes(r *gin.Engine, limiter gin.HandlerFunc) {
	group := r.Group("/sub")
	if limiter != nil {
		group.Use(limiter)
	}
	group.GET("/:subId", h.Auto)
	// Explicit-format routes for direct linking.
	group.GET("/json/:subId", h.bind(FormatJSON))
	group.GET("/clash/:subId", h.bind(FormatClash))
	group.GET("/singbox/:subId", h.bind(FormatSingBox))
	group.GET("/surge/:subId", h.bind(FormatSurge))
	group.GET("/sip008/:subId", h.bind(FormatSIP008))
	group.GET("/wireguard/:subId", h.bind(FormatWireGuard))
	group.GET("/wireguard-zip/:subId", h.bind(FormatWGZip))
	// Self-hosted rule lists: profiles in self_hosted mode point their
	// rule-providers here instead of at the upstream URL.
	group.GET("/ruleset/:key", h.ServeRuleset)
}

// ServeRuleset returns a ruleset's rule list (the body a self_hosted
// profile's rule-providers point at). The :key maps to an admin-
// configured ruleset (or a built-in default); the body is fetched +
// cached server-side.
func (h *SubHandler) ServeRuleset(c *gin.Context) {
	rs := h.lookupRuleset(c.Request.Context(), c.Param("key"))
	if rs == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ruleset not found"})
		return
	}
	if h.rulesetCache == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ruleset cache unavailable"})
		return
	}
	content, err := h.rulesetCache.Content(c.Request.Context(), *rs)
	if err != nil {
		h.log.Warn("ruleset fetch failed", slog.String("key", rs.Key), slog.String("err", err.Error()))
		c.JSON(http.StatusBadGateway, gin.H{"error": "ruleset fetch failed"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, content)
}

// lookupRuleset resolves a ruleset key to its definition: the DB row if
// present, else a built-in default ruleset of that key.
func (h *SubHandler) lookupRuleset(ctx context.Context, key string) *model.SubscriptionRuleset {
	if h.rulesets != nil {
		if rs, _ := h.rulesets.GetByKey(ctx, key); rs != nil {
			return rs
		}
	}
	for _, rs := range policy.DefaultRulesets() {
		if rs.Key == key {
			rs := rs
			return &rs
		}
	}
	return nil
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
		// Routing policy comes from the selected profile (?profile= or the
		// default), falling back to the built-in default. `base` is the
		// optional operator template override.
		profile, rulesets := h.resolveProfile(c.Request.Context(), c.Query("profile"))
		base := h.clashBase(c.Request.Context())
		serveBase := h.subscriptionPublicBaseURL(c)
		body, err := h.asm.FormatClash(data, profile, rulesets, base, serveBase)
		if err != nil {
			h.log.Error("FormatClash failed, retrying without operator base", "err", err)
			body, err = h.asm.FormatClash(data, profile, rulesets, "", serveBase)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	case FormatSingBox:
		base := h.singboxBase(c.Request.Context())
		body, err := h.asm.FormatSingBox(data, base)
		if err != nil {
			h.log.Error("FormatSingBox failed, retrying without operator base", "err", err)
			body, err = h.asm.FormatSingBox(data, "")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	case FormatSurge:
		profile, rulesets := h.resolveProfile(c.Request.Context(), c.Query("profile"))
		body, err := h.asm.FormatSurge(data, profile, rulesets, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "text/plain; charset=utf-8")
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
	case FormatWireGuard:
		// Plain .conf format. If the user has exactly one WG peer
		// we serve it as a single config; if more, concatenate with
		// a [Interface] block per peer (still valid wg-quick input,
		// but most clients prefer the ZIP variant for multi-peer).
		body := strings.Join(wgConfBodies(data), "\n\n")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="wireguard.conf"`)
		c.String(http.StatusOK, body)
	case FormatWGZip:
		body, err := sub.BuildWGConfZip(data.Links)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", `attachment; filename="wireguard.zip"`)
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(body)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "unsupported format; valid: base64, json, clash, singbox, surge, sip008, wireguard, wireguard-zip",
		})
	}
}

// wgConfBodies returns the .conf body for every WG link in data,
// preserving link order. Non-WG links are skipped silently.
func wgConfBodies(data *sub.SubscriptionData) []string {
	if data == nil {
		return nil
	}
	out := make([]string, 0, len(data.Links))
	for _, l := range data.Links {
		if l.Protocol != "wireguard" {
			continue
		}
		body := sub.BuildWGConf(l)
		if body == "" {
			continue
		}
		out = append(out, body)
	}
	return out
}

// detectFormat picks the response format from either an explicit query
// param or the User-Agent. `?format=` always wins; UA is the fallback.
//
// Recognized UA needles (case-insensitive):
//
//	clash, mihomo, stash        → clash
//	sing-box, singbox           → singbox
//	shadowsocks                 → sip008
//	anything else               → base64
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
		case "surge":
			return FormatSurge
		case "sip008":
			return FormatSIP008
		case "wireguard", "wg":
			return FormatWireGuard
		case "wireguard-zip", "wg-zip":
			return FormatWGZip
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
	case strings.Contains(l, "surge"):
		return FormatSurge
	case strings.Contains(l, "shadowsocks"):
		return FormatSIP008
	default:
		return FormatBase64
	}
}

// requestOrigin reconstructs the absolute origin the client used to
// reach us (scheme + host) so self-hosted rule-provider URLs point back
// at this dashboard. Honors X-Forwarded-Proto for TLS-terminating
// proxies.
func requestOrigin(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func (h *SubHandler) subscriptionPublicBaseURL(c *gin.Context) string {
	if h.settings != nil {
		v, _, _ := h.settings.Get(c.Request.Context(), model.SettingSubscriptionPublicBaseURL)
		if normalized := normalizePublicBaseURL(v); normalized != "" {
			return normalized
		}
	}
	if h.publicBaseURL != "" {
		return h.publicBaseURL
	}
	return requestOrigin(c)
}

func normalizePublicBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

// clashBase returns the operator's Clash template override, or "" when
// unset (use the built-in skeleton).
func (h *SubHandler) clashBase(ctx context.Context) string {
	if h.settings == nil {
		return ""
	}
	v, _, _ := h.settings.Get(ctx, model.SettingClashTemplateYAML)
	return v
}

// singboxBase returns the operator's sing-box template override, or ""
// when unset.
func (h *SubHandler) singboxBase(ctx context.Context) string {
	if h.settings == nil {
		return ""
	}
	v, _, _ := h.settings.Get(ctx, model.SettingSingBoxTemplateJSON)
	return v
}

func (h *SubHandler) errorResponse(c *gin.Context, err error) {
	if errors.Is(err, sub.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
