package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
)

// SettingHandler serves /api/admin/settings/*.
//
// Each well-known setting is typed: bools store "true"/"false"; ints
// store the decimal text; strings store as-is. The handler validates
// per key so the admin UI can submit free-form input without exotic
// guard rails on the client side.
type SettingHandler struct {
	repo   *repository.SettingRepo
	cfg    *config.Config
	mailer *mailer.Mailer
}

// NewSettingHandler wires the handler.
func NewSettingHandler(repo *repository.SettingRepo, cfg *config.Config, m *mailer.Mailer) *SettingHandler {
	return &SettingHandler{repo: repo, cfg: cfg, mailer: m}
}

// RegisterRoutes mounts /settings under rg.
func (h *SettingHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/settings")
	g.GET("", h.List)
	g.PUT("/:key", h.Put)
	g.DELETE("/:key", h.Delete)
	g.POST("/smtp-test", h.SMTPTest)
}

// SMTPTest sends a one-shot test email so the admin can verify
// SMTP config without waiting for a real user verification flow.
// Body: {"to": "admin@example.com"}.
//   - 400 if `to` missing or invalid
//   - 503 if SMTP is not enabled (cfg.SMTP.Enabled() == false)
//   - 502 if delivery failed — error message in the body
//   - 200 if accepted by the upstream SMTP relay
func (h *SettingHandler) SMTPTest(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.To == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to is required"})
		return
	}
	if h.mailer == nil || !h.mailer.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SMTP not configured"})
		return
	}
	subject := "3xui-dashboard SMTP test"
	body := "If you received this, the dashboard's SMTP config is working.\n\nSent at " + time.Now().UTC().Format(time.RFC3339)
	if err := h.mailer.Send(req.To, subject, body); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "to": req.To})
}

// settingDescriptor enumerates the well-known keys the UI knows how to
// render + validate. Unknown keys are accepted for forward compat but
// shown as raw strings.
type settingDescriptor struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Type         string `json:"type"`   // bool | int | string
	Group        string `json:"group"`  // registration / subscription / traffic
	Default      string `json:"default"`
	Description  string `json:"description"`
}

var knownSettings = []settingDescriptor{
	{
		Key:         model.SettingPublicRegistrationEnabled,
		Label:       "Public registration enabled",
		Type:        "bool",
		Group:       "registration",
		Description: "When true, the user portal /register endpoint accepts new signups. Overrides the PUBLIC_REGISTRATION env var.",
	},
	{
		Key:         model.SettingEmailDomainAllowlist,
		Label:       "Email domain allowlist",
		Type:        "string",
		Group:       "registration",
		Description: "Comma-separated list of email domains permitted to register or bind. Empty = unrestricted. Overrides EMAIL_DOMAIN_ALLOWLIST.",
	},
	{
		Key:         model.SettingSubscriptionRemarkModel,
		Label:       "Subscription remark model",
		Type:        "string",
		Group:       "subscription",
		Default:     "-ieo",
		Description: "Format spec for client link labels in /sub. First rune is the separator; remaining runes are tokens i/e/o/t (inbound, email, node, tag).",
	},
	{
		Key:         model.SettingTrafficWarnPct,
		Label:       "Traffic warning %",
		Type:        "int",
		Group:       "traffic",
		Default:     "80",
		Description: "Emit client.over_limit warning when usage reaches this percentage of the cap (1-100).",
	},
	{
		Key:         model.SettingTrafficCriticalPct,
		Label:       "Traffic critical %",
		Type:        "int",
		Group:       "traffic",
		Default:     "95",
		Description: "Emit critical client.over_limit when usage reaches this percentage (1-100).",
	},
	{
		Key:         model.SettingExpiryWarnDays,
		Label:       "Expiry warning days",
		Type:        "int",
		Group:       "traffic",
		Default:     "3",
		Description: "Emit warning when a client.expires_at is within this many days.",
	},
	{
		Key:         model.SettingClashTemplateYAML,
		Label:       "Clash template (YAML)",
		Type:        "string",
		Group:       "subscription",
		Description: "Override the embedded Mihomo Clash template. Must contain ${proxies} and ${proxy_names} placeholders. Empty = use built-in default.",
	},
	{
		Key:         model.SettingSingBoxTemplateJSON,
		Label:       "Sing-box template (JSON)",
		Type:        "string",
		Group:       "subscription",
		Description: "Override the embedded sing-box template. Must contain ${proxies} and ${proxy_names} placeholders. Empty = use built-in default.",
	},
	{
		Key:         model.SettingProxyGroupStrategy,
		Label:       "Proxy group strategy",
		Type:        "string",
		Group:       "subscription",
		Default:     "auto+select",
		Description: "One of: auto-only / select-only / auto+select. Controls the default Clash template's proxy-groups block. Ignored when clash_template_yaml is set.",
	},
	{
		Key:         model.SettingRuleProvidersEnabled,
		Label:       "Rule providers enabled",
		Type:        "bool",
		Group:       "subscription",
		Default:     "true",
		Description: "When false, the default Clash template strips rule-providers + rules — emitting just proxies + groups + a MATCH fallback. Ignored when clash_template_yaml is set.",
	},
}

// listResponse pairs the descriptor with the current persisted value
// (and a fallback derived from cfg when the row is absent).
type settingItem struct {
	settingDescriptor
	Value       string `json:"value"`        // current persisted override; empty if no row
	HasOverride bool   `json:"has_override"` // true when a row exists
	EnvFallback string `json:"env_fallback"` // computed from config — purely informational
}

// List returns every known descriptor with its current value, plus any
// unknown rows in the table (forward compat). The UI uses this for
// the Settings form initial render.
func (h *SettingHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	persisted, err := h.repo.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out := make([]settingItem, 0, len(knownSettings))
	for _, d := range knownSettings {
		v, ok := persisted[d.Key]
		out = append(out, settingItem{
			settingDescriptor: d,
			Value:             v,
			HasOverride:       ok,
			EnvFallback:       h.envFallback(d.Key),
		})
	}
	// Bring along any unknown persisted rows so admins see them.
	for k, v := range persisted {
		if isKnown(k) {
			continue
		}
		out = append(out, settingItem{
			settingDescriptor: settingDescriptor{
				Key: k, Label: k, Type: "string", Group: "other",
			},
			Value:       v,
			HasOverride: true,
		})
	}
	c.JSON(http.StatusOK, gin.H{"settings": out})
}

// putRequest binds the body for PUT /:key.
type putRequest struct {
	Value string `json:"value"`
}

// Put validates per-key and upserts.
func (h *SettingHandler) Put(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}
	var body putRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := validate(key, body.Value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Set(c.Request.Context(), key, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "value": body.Value})
}

// Delete clears the override row so the setting falls back to env / code default.
func (h *SettingHandler) Delete(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// envFallback reports the env-driven default for the matching key so
// the UI can show "(currently using env default: …)".
func (h *SettingHandler) envFallback(key string) string {
	switch key {
	case model.SettingPublicRegistrationEnabled:
		return strconv.FormatBool(h.cfg.PublicRegistration)
	case model.SettingEmailDomainAllowlist:
		return strings.Join(h.cfg.EmailDomainAllowlist, ",")
	default:
		// no env equivalent
		return ""
	}
}

func isKnown(key string) bool {
	for _, d := range knownSettings {
		if d.Key == key {
			return true
		}
	}
	return false
}

// validate enforces type rules per known key. Unknown keys accept any
// string.
func validate(key, value string) error {
	var typ string
	for _, d := range knownSettings {
		if d.Key == key {
			typ = d.Type
			break
		}
	}
	switch typ {
	case "bool":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "false", "1", "0", "yes", "no", "on", "off":
			return nil
		default:
			return errors.New("value must be a boolean (true/false/1/0/yes/no/on/off)")
		}
	case "int":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return errors.New("value must be an integer")
		}
		// All current int settings are percentages or day counts; constrain to a sane positive range.
		if strings.HasSuffix(key, "_pct") && (n < 0 || n > 100) {
			return fmt.Errorf("value %d outside 0-100 range for %q", n, key)
		}
		if strings.HasSuffix(key, "_days") && n < 0 {
			return fmt.Errorf("value %d cannot be negative for %q", n, key)
		}
		return nil
	default:
		// String-type per-key validation (only kicks in for keys that
		// need format checking; everything else accepts arbitrary text).
		switch key {
		case model.SettingClashTemplateYAML:
			if strings.TrimSpace(value) == "" {
				return nil // empty = use embedded default
			}
			// Clash config must be a top-level mapping. yaml.Unmarshal
			// into a map (not `any`) rejects bare scalars like
			// "::not::yaml::" that would otherwise pass as a string.
			var probe map[string]any
			if err := yaml.Unmarshal([]byte(value), &probe); err != nil {
				return fmt.Errorf("clash_template_yaml: invalid YAML: %w", err)
			}
			if probe == nil {
				return errors.New("clash_template_yaml: must be a YAML object (got null or scalar)")
			}
			// Require the placeholder so we know where to inject proxies.
			if !strings.Contains(value, "${proxies}") {
				return errors.New("clash_template_yaml: must contain the ${proxies} placeholder")
			}
		case model.SettingSingBoxTemplateJSON:
			if strings.TrimSpace(value) == "" {
				return nil
			}
			var probe map[string]any
			if err := json.Unmarshal([]byte(value), &probe); err != nil {
				return fmt.Errorf("singbox_template_json: invalid JSON: %w", err)
			}
			if probe == nil {
				return errors.New("singbox_template_json: must be a JSON object")
			}
			if !strings.Contains(value, "${proxies}") {
				return errors.New("singbox_template_json: must contain the ${proxies} placeholder")
			}
		case model.SettingProxyGroupStrategy:
			switch strings.TrimSpace(value) {
			case "", "auto-only", "select-only", "auto+select":
				return nil
			default:
				return fmt.Errorf("proxy_group_strategy must be one of: auto-only, select-only, auto+select")
			}
		}
		return nil
	}
}
