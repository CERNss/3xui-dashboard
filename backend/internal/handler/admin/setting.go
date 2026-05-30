package admin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	g.POST("/branding/icon", h.UploadBrandIcon)
}

// BrandingHandler exposes the public, unauthenticated branding
// metadata used by the login page and shell chrome.
type BrandingHandler struct {
	repo *repository.SettingRepo
}

func NewBrandingHandler(repo *repository.SettingRepo) *BrandingHandler {
	return &BrandingHandler{repo: repo}
}

func (h *BrandingHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/branding", h.Get)
}

func (h *BrandingHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	values := map[string]string{}
	for _, key := range []string{
		model.SettingBrandIconURL,
		model.SettingBrandTitle,
		model.SettingBrandSubtitle,
		model.SettingBrandDescription,
		model.SettingBrandFooter,
		model.SettingBrandDocsURL,
		model.SettingBrandHomepageContent,
	} {
		value, _, err := h.repo.Get(ctx, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		values[key] = value
	}
	c.JSON(http.StatusOK, gin.H{
		"icon_url":         values[model.SettingBrandIconURL],
		"title":            firstNonEmpty(values[model.SettingBrandTitle], defaultBrandTitle),
		"subtitle":         firstNonEmpty(values[model.SettingBrandSubtitle], defaultBrandSubtitle),
		"description":      firstNonEmpty(values[model.SettingBrandDescription], defaultBrandDescription),
		"footer":           firstNonEmpty(values[model.SettingBrandFooter], defaultBrandFooter),
		"docs_url":         values[model.SettingBrandDocsURL],
		"homepage_content": values[model.SettingBrandHomepageContent],
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

const (
	defaultBrandTitle       = "3xui Central"
	defaultBrandSubtitle    = "中央面板"
	defaultBrandDescription = "多节点 3x-ui · 集群聚合 · 流量分账 · 订阅导出"
	defaultBrandFooter      = "© 2026 3xui Central · 自托管多节点控制面板"
)

func validateBrandText(key, value string) error {
	limit := 0
	switch key {
	case model.SettingBrandTitle:
		limit = 80
	case model.SettingBrandSubtitle:
		limit = 120
	case model.SettingBrandDescription, model.SettingBrandFooter:
		limit = 240
	case model.SettingBrandHomepageContent:
		limit = 4000
	default:
		return nil
	}
	if len([]rune(strings.TrimSpace(value))) > limit {
		return fmt.Errorf("%s must be %d characters or fewer", key, limit)
	}
	return nil
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

const (
	brandIconMaxBytes = 1 << 20 // 1 MiB
	brandUploadDir    = "uploads/branding"
)

const (
	templateProxiesPlaceholder     = "${proxies}"
	templateProxyNamesPlaceholder  = "${proxy_names}"
	templateProxyGroupsPlaceholder = "${proxy_groups}"
)

// Brand icons accept raster formats only. SVG was historically
// permitted but is dropped because it can carry inline <script> and
// event-handler attributes that browsers would execute when the icon
// is rendered from /uploads/branding/* with image/svg+xml — and
// sanitizing SVG is too easy to get wrong. Operators wanting vector
// art should export to PNG.
var allowedBrandIconTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// UploadBrandIcon accepts multipart field "file", stores it under
// uploads/branding, and persists the public URL in settings. The
// public file handler is wired in app.Build at /uploads/branding/*.
func (h *SettingHandler) UploadBrandIcon(c *gin.Context) {
	req := c.Request
	req.Body = http.MaxBytesReader(c.Writer, req.Body, brandIconMaxBytes+1024)
	file, header, err := req.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}
	defer file.Close()
	if header.Size > brandIconMaxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "icon must be 1 MiB or smaller"})
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, brandIconMaxBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read upload: " + err.Error()})
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "icon file is empty"})
		return
	}
	if len(data) > brandIconMaxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "icon must be 1 MiB or smaller"})
		return
	}

	contentType := http.DetectContentType(data)
	ext, ok := allowedBrandIconTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "icon must be PNG, JPEG, or WebP"})
		return
	}
	if err := os.MkdirAll(brandUploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare upload directory: " + err.Error()})
		return
	}

	name := "icon-" + randomHex(12) + ext
	dst := filepath.Join(brandUploadDir, name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save icon: " + err.Error()})
		return
	}
	url := "/uploads/branding/" + name
	if err := h.repo.Set(c.Request.Context(), model.SettingBrandIconURL, url); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"key":          model.SettingBrandIconURL,
		"value":        url,
		"url":          url,
		"content_type": contentType,
		"size":         len(data),
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	const hex = "0123456789abcdef"
	out := make([]byte, n*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}

// settingDescriptor enumerates the well-known keys the UI knows how to
// render + validate. Unknown keys are shown as raw strings.
//
// LabelZh / DescriptionZh hold the zh-CN translations. The frontend
// picks between en (Label/Description) and zh (LabelZh/DescriptionZh)
// based on its active locale — that keeps locale switching purely
// client-side. Group labels are translated by the frontend's i18n
// dictionary (zh.ts admin.settings.group*), so we deliberately do
// not duplicate them here.
type settingDescriptor struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	LabelZh       string `json:"label_zh,omitempty"`
	Type          string `json:"type"`  // bool | int | string
	Group         string `json:"group"` // registration / subscription / traffic / data_collection / other
	Default       string `json:"default"`
	Description   string `json:"description"`
	DescriptionZh string `json:"description_zh,omitempty"`
}

var knownSettings = []settingDescriptor{
	{
		Key:           model.SettingPublicRegistrationEnabled,
		Label:         "Public registration enabled",
		LabelZh:       "公开注册",
		Type:          "bool",
		Group:         "registration",
		Default:       "true",
		Description:   "When true, the user portal /register endpoint accepts new signups. Overrides the PUBLIC_REGISTRATION env var.",
		DescriptionZh: "启用后用户端 /register 接受新注册；覆盖 PUBLIC_REGISTRATION 环境变量。",
	},
	{
		Key:           model.SettingEmailVerificationRequired,
		Label:         "Email verification required",
		LabelZh:       "邮箱验证",
		Type:          "bool",
		Group:         "registration",
		Description:   "When true, new users must verify their email with a code during registration. Empty follows SMTP availability.",
		DescriptionZh: "启用后新用户注册时必须验证邮箱；留空时跟随 SMTP 是否可用。",
	},
	{
		Key:           model.SettingEmailDomainAllowlist,
		Label:         "Email domain allowlist",
		LabelZh:       "邮箱域名白名单",
		Type:          "string",
		Group:         "registration",
		Description:   "Comma-separated list of email domains permitted to register or bind. Empty = unrestricted. Overrides EMAIL_DOMAIN_ALLOWLIST.",
		DescriptionZh: "允许注册/绑定的邮箱域名，英文逗号分隔；空 = 不限制。覆盖 EMAIL_DOMAIN_ALLOWLIST。",
	},
	{
		Key:           model.SettingNewUserInitialBalanceCents,
		Label:         "New-user initial balance",
		LabelZh:       "新用户初始余额",
		Type:          "int",
		Group:         "registration",
		Default:       "0",
		Description:   "Balance credited to self-service and brand-new OIDC users at signup, stored in cents. Admin-created users use the create-user form field instead.",
		DescriptionZh: "自助注册和全新 OIDC 用户创建时自动发放的余额，单位为分。管理员手动创建用户时使用创建表单里的初始余额。",
	},
	{
		Key:           model.SettingNewUserPlanIDs,
		Label:         "New-user starter plans",
		LabelZh:       "新用户可选套餐",
		Type:          "string",
		Group:         "registration",
		Description:   "Comma-separated plan IDs visible and purchasable only while a user has no paid/completed order history. Empty = all enabled plans.",
		DescriptionZh: "逗号分隔的套餐 ID。用户没有付款中/已付款/已完成订单前，只能看到并购买这些套餐；空 = 不限制。",
	},
	{
		Key:           model.SettingOIDCEnabled,
		Label:         "OIDC login enabled",
		LabelZh:       "启用 OIDC 登录",
		Type:          "bool",
		Group:         "other",
		Default:       "true",
		Description:   "When false, hides OIDC sign-in even if provider settings are complete.",
		DescriptionZh: "关闭后即使 Provider 配置完整，也不会在登录页显示 OIDC 入口。",
	},
	{
		Key:           model.SettingOIDCIssuer,
		Label:         "OIDC issuer",
		LabelZh:       "OIDC Issuer",
		Type:          "string",
		Group:         "other",
		Description:   "OIDC issuer base URL. Empty falls back to OIDC_ISSUER.",
		DescriptionZh: "OIDC 签发方基础 URL。留空回退 OIDC_ISSUER 环境变量。",
	},
	{
		Key:           model.SettingOIDCClientID,
		Label:         "OIDC client ID",
		LabelZh:       "OIDC Client ID",
		Type:          "string",
		Group:         "other",
		Description:   "OIDC OAuth client ID. Empty falls back to OIDC_CLIENT_ID.",
		DescriptionZh: "OIDC OAuth Client ID。留空回退 OIDC_CLIENT_ID。",
	},
	{
		Key:           model.SettingOIDCClientSecret,
		Label:         "OIDC client secret",
		LabelZh:       "OIDC Client Secret",
		Type:          "string",
		Group:         "other",
		Description:   "OIDC OAuth client secret. Empty falls back to OIDC_CLIENT_SECRET.",
		DescriptionZh: "OIDC OAuth Client Secret。留空回退 OIDC_CLIENT_SECRET。",
	},
	{
		Key:           model.SettingOIDCRedirectURL,
		Label:         "OIDC redirect URL",
		LabelZh:       "OIDC 回调地址",
		Type:          "string",
		Group:         "other",
		Description:   "Dashboard callback URL registered with the IDP. Empty falls back to OIDC_REDIRECT_URL.",
		DescriptionZh: "在身份提供商里登记的 Dashboard 回调地址。留空回退 OIDC_REDIRECT_URL。",
	},
	{
		Key:           model.SettingOIDCScopes,
		Label:         "OIDC scopes",
		LabelZh:       "OIDC Scopes",
		Type:          "string",
		Group:         "other",
		Default:       "openid,profile,email",
		Description:   "Comma-separated scopes requested from the IDP. Empty falls back to OIDC_SCOPES.",
		DescriptionZh: "向身份提供商请求的 scopes，英文逗号分隔。留空回退 OIDC_SCOPES。",
	},
	{
		Key:           model.SettingOIDCDisplayName,
		Label:         "OIDC display name",
		LabelZh:       "OIDC 显示名称",
		Type:          "string",
		Group:         "other",
		Description:   "Name shown on the login button. Empty falls back to OIDC_DISPLAY_NAME or issuer host.",
		DescriptionZh: "登录按钮上显示的名称。留空回退 OIDC_DISPLAY_NAME 或 issuer 域名。",
	},
	{
		Key:           model.SettingOIDCIconURL,
		Label:         "OIDC icon URL",
		LabelZh:       "OIDC 图标 URL",
		Type:          "string",
		Group:         "other",
		Description:   "Optional login button icon. Empty falls back to OIDC_ICON_URL.",
		DescriptionZh: "登录按钮图标，可选。留空回退 OIDC_ICON_URL。",
	},
	{
		Key:           model.SettingOIDCAuthURL,
		Label:         "OIDC auth URL",
		LabelZh:       "OIDC 授权端点",
		Type:          "string",
		Group:         "other",
		Description:   "Optional authorization endpoint override. Empty uses discovery or OIDC_AUTH_URL.",
		DescriptionZh: "可选授权端点覆盖。留空使用 discovery 或 OIDC_AUTH_URL。",
	},
	{
		Key:           model.SettingOIDCTokenURL,
		Label:         "OIDC token URL",
		LabelZh:       "OIDC Token 端点",
		Type:          "string",
		Group:         "other",
		Description:   "Optional token endpoint override. Empty uses discovery or OIDC_TOKEN_URL.",
		DescriptionZh: "可选 token 端点覆盖。留空使用 discovery 或 OIDC_TOKEN_URL。",
	},
	{
		Key:           model.SettingOIDCJWKSURL,
		Label:         "OIDC JWKS URL",
		LabelZh:       "OIDC JWKS 端点",
		Type:          "string",
		Group:         "other",
		Description:   "Optional JWKS endpoint override. Empty uses discovery or OIDC_JWKS_URL.",
		DescriptionZh: "可选 JWKS 端点覆盖。留空使用 discovery 或 OIDC_JWKS_URL。",
	},
	{
		Key:           model.SettingOIDCUserInfoURL,
		Label:         "OIDC userinfo URL",
		LabelZh:       "OIDC UserInfo 端点",
		Type:          "string",
		Group:         "other",
		Description:   "Optional userinfo endpoint override. Empty uses discovery or OIDC_USERINFO_URL.",
		DescriptionZh: "可选 userinfo 端点覆盖。留空使用 discovery 或 OIDC_USERINFO_URL。",
	},
	{
		Key:           model.SettingOpsCollectEnabled,
		Label:         "Node health collection",
		LabelZh:       "节点健康采集",
		Type:          "bool",
		Group:         "data_collection",
		Default:       "true",
		Description:   "Controls whether the background collector samples node health from upstream panels.",
		DescriptionZh: "控制后台采集器是否定时从上游面板采集节点健康数据。",
	},
	{
		Key:           model.SettingOpsCollectIntervalSeconds,
		Label:         "Health collection interval",
		LabelZh:       "健康采集间隔",
		Type:          "int",
		Group:         "data_collection",
		Default:       "60",
		Description:   "Seconds between health collection passes. Minimum 5 seconds.",
		DescriptionZh: "健康数据采集间隔，单位秒；最小 5 秒。",
	},
	{
		Key:           model.SettingOpsCollectConcurrency,
		Label:         "Health collection concurrency",
		LabelZh:       "健康采集并发",
		Type:          "int",
		Group:         "data_collection",
		Default:       "8",
		Description:   "Maximum number of nodes probed at the same time. Range: 1-64.",
		DescriptionZh: "健康采集单轮最多同时请求的节点数，范围 1-64。",
	},
	{
		Key:           model.SettingOpsCollectTimeoutSeconds,
		Label:         "Health request timeout",
		LabelZh:       "健康请求超时",
		Type:          "int",
		Group:         "data_collection",
		Default:       "12",
		Description:   "Per-node health probe timeout in seconds. Range: 1-300.",
		DescriptionZh: "单节点健康探测超时时间，单位秒；范围 1-300。",
	},
	{
		Key:           model.SettingOpsCollectRetryAttempts,
		Label:         "Health retry attempts",
		LabelZh:       "健康重试次数",
		Type:          "int",
		Group:         "data_collection",
		Default:       "0",
		Description:   "Additional retry attempts after a failed health request. Range: 0-5.",
		DescriptionZh: "健康请求失败后的额外重试次数，范围 0-5。",
	},
	{
		Key:           model.SettingOpsRetentionSeconds,
		Label:         "Health history retention",
		LabelZh:       "健康历史保留",
		Type:          "int",
		Group:         "data_collection",
		Default:       "21600",
		Description:   "Seconds of in-memory health samples retained for status and ops charts.",
		DescriptionZh: "系统状态和 ops 图表保留的内存健康样本时长，单位秒。",
	},
	{
		Key:           model.SettingTrafficCollectEnabled,
		Label:         "Node traffic collection",
		LabelZh:       "节点流量采集",
		Type:          "bool",
		Group:         "data_collection",
		Default:       "true",
		Description:   "Controls whether the background collector samples inbound and client traffic counters from nodes.",
		DescriptionZh: "控制后台采集器是否定时采集节点入站和客户端流量计数。",
	},
	{
		Key:           model.SettingTrafficCollectIntervalSecs,
		Label:         "Traffic collection interval",
		LabelZh:       "流量采集间隔",
		Type:          "int",
		Group:         "data_collection",
		Default:       "60",
		Description:   "Seconds between node traffic collection passes. Minimum 5 seconds.",
		DescriptionZh: "节点流量采集间隔，单位秒；最小 5 秒。",
	},
	{
		Key:           model.SettingTrafficCollectConcurrency,
		Label:         "Traffic collection concurrency",
		LabelZh:       "流量采集并发",
		Type:          "int",
		Group:         "data_collection",
		Default:       "8",
		Description:   "Maximum number of nodes queried for traffic snapshots at the same time. Range: 1-64.",
		DescriptionZh: "流量采集单轮最多同时请求的节点数，范围 1-64。",
	},
	{
		Key:           model.SettingTrafficCollectTimeoutSecs,
		Label:         "Traffic request timeout",
		LabelZh:       "流量请求超时",
		Type:          "int",
		Group:         "data_collection",
		Default:       "30",
		Description:   "Per-node traffic snapshot timeout in seconds. Range: 1-300.",
		DescriptionZh: "单节点流量快照超时时间，单位秒；范围 1-300。",
	},
	{
		Key:           model.SettingTrafficCollectRetryAttempts,
		Label:         "Traffic retry attempts",
		LabelZh:       "流量重试次数",
		Type:          "int",
		Group:         "data_collection",
		Default:       "0",
		Description:   "Additional retry attempts after a failed traffic snapshot request. Range: 0-5.",
		DescriptionZh: "流量请求失败后的额外重试次数，范围 0-5。",
	},
	{
		Key:           model.SettingTrafficRetentionSeconds,
		Label:         "Traffic sample retention",
		LabelZh:       "流量样本保留",
		Type:          "int",
		Group:         "data_collection",
		Default:       "2592000",
		Description:   "Seconds of persisted traffic samples retained for usage history. Set 0 to disable cleanup.",
		DescriptionZh: "用量历史保留的持久化流量样本时长，单位秒；设为 0 则不清理。",
	},
	{
		Key:           model.SettingSubscriptionRemarkModel,
		Label:         "Subscription remark model",
		LabelZh:       "订阅链接备注格式",
		Type:          "string",
		Group:         "subscription",
		Default:       "-ieo",
		Description:   "Format spec for client link labels in /sub. First rune is the separator; remaining runes are tokens i/e/o/t (inbound, email, node, tag).",
		DescriptionZh: "/sub 客户端链接 label 的格式串。首字符为分隔符；后续字符为字段标记 i/e/o/t（inbound、email、node、tag）。",
	},
	{
		Key:           model.SettingTrafficWarnPct,
		Label:         "Traffic warning %",
		LabelZh:       "流量预警 %",
		Type:          "int",
		Group:         "traffic",
		Default:       "80",
		Description:   "Emit client.over_limit warning when usage reaches this percentage of the cap (1-100).",
		DescriptionZh: "用量达到限额此百分比时触发 client.over_limit 预警事件（1-100）。",
	},
	{
		Key:           model.SettingTrafficCriticalPct,
		Label:         "Traffic critical %",
		LabelZh:       "流量紧急 %",
		Type:          "int",
		Group:         "traffic",
		Default:       "95",
		Description:   "Emit critical client.over_limit when usage reaches this percentage (1-100).",
		DescriptionZh: "用量达到限额此百分比时触发 critical 级 client.over_limit（1-100）。",
	},
	{
		Key:           model.SettingExpiryWarnDays,
		Label:         "Expiry warning days",
		LabelZh:       "到期预警天数",
		Type:          "int",
		Group:         "traffic",
		Default:       "3",
		Description:   "Emit warning when a client.expires_at is within this many days.",
		DescriptionZh: "客户端到期时间小于这么多天时触发预警。",
	},
	{
		Key:           model.SettingBrandIconURL,
		Label:         "Brand icon URL",
		LabelZh:       "品牌图标 URL",
		Type:          "string",
		Group:         "other",
		Description:   "Uploaded panel icon URL. Prefer the upload control above; manual values must be a relative /uploads/ URL or an http(s) URL.",
		DescriptionZh: "面板品牌图标地址。建议使用上方上传控件；手动值必须是 /uploads/ 相对地址或 http(s) URL。",
	},
	{
		Key:           model.SettingBrandTitle,
		Label:         "Brand title",
		LabelZh:       "品牌标题",
		Type:          "string",
		Group:         "other",
		Default:       defaultBrandTitle,
		Description:   "Main display name shown in the login page, admin shell, and user portal.",
		DescriptionZh: "展示在登录页、后台侧栏和用户端顶部的主名称。",
	},
	{
		Key:           model.SettingBrandSubtitle,
		Label:         "Brand subtitle",
		LabelZh:       "品牌副标题",
		Type:          "string",
		Group:         "other",
		Default:       defaultBrandSubtitle,
		Description:   "Short label shown under the main brand title in compact navigation surfaces.",
		DescriptionZh: "展示在主标题下方的短说明，用于后台侧栏等紧凑区域。",
	},
	{
		Key:           model.SettingBrandDescription,
		Label:         "Brand description",
		LabelZh:       "品牌描述",
		Type:          "string",
		Group:         "other",
		Default:       defaultBrandDescription,
		Description:   "Supporting copy shown on the login page.",
		DescriptionZh: "登录页标题下方的展示文案。",
	},
	{
		Key:           model.SettingBrandFooter,
		Label:         "Brand footer",
		LabelZh:       "品牌页脚",
		Type:          "string",
		Group:         "other",
		Default:       defaultBrandFooter,
		Description:   "Footer line shown under the login panel.",
		DescriptionZh: "登录面板下方展示的页脚文案。",
	},
	{
		Key:           model.SettingBrandDocsURL,
		Label:         "Documentation link",
		LabelZh:       "文档链接",
		Type:          "string",
		Group:         "other",
		Description:   "Optional documentation URL for operators and users. Leave empty to hide the documentation link.",
		DescriptionZh: "面向管理员和用户的文档链接；留空则隐藏文档链接。",
	},
	{
		Key:           model.SettingBrandHomepageContent,
		Label:         "Homepage content",
		LabelZh:       "首页内容",
		Type:          "string",
		Group:         "other",
		Description:   "Optional homepage copy. Supports Markdown/HTML when rendered by a trusted public page.",
		DescriptionZh: "可选首页内容；由受信任的公开页面渲染时可支持 Markdown/HTML。",
	},
	{
		Key:           model.SettingClashTemplateYAML,
		Label:         "Clash template (YAML)",
		LabelZh:       "Clash 模板（YAML）",
		Type:          "string",
		Group:         "subscription",
		Description:   "Override the embedded Mihomo Clash template. Must contain ${proxies} and ${proxy_names} placeholders. Empty = use built-in default.",
		DescriptionZh: "覆盖内置 Mihomo Clash 模板。必须包含 ${proxies} 和 ${proxy_names} 占位符。空 = 用内置默认。",
	},
	{
		Key:           model.SettingSingBoxTemplateJSON,
		Label:         "Sing-box template (JSON)",
		LabelZh:       "Sing-box 模板（JSON）",
		Type:          "string",
		Group:         "subscription",
		Description:   "Override the embedded sing-box template. Must contain ${proxies} and ${proxy_names} placeholders. Empty = use built-in default.",
		DescriptionZh: "覆盖内置 sing-box 模板。必须包含 ${proxies} 和 ${proxy_names} 占位符。空 = 用内置默认。",
	},
	{
		Key:           model.SettingProxyGroupStrategy,
		Label:         "Proxy group strategy",
		LabelZh:       "代理组策略",
		Type:          "string",
		Group:         "subscription",
		Default:       "auto+select",
		Description:   "One of: auto-only / select-only / auto+select. Controls the default Clash template's proxy-groups block. Ignored when clash_template_yaml is set.",
		DescriptionZh: "取值之一：auto-only / select-only / auto+select。控制默认 Clash 模板 proxy-groups。如已设置 clash_template_yaml 则忽略。",
	},
	{
		Key:           model.SettingRuleProvidersEnabled,
		Label:         "Rule providers enabled",
		LabelZh:       "启用 rule providers",
		Type:          "bool",
		Group:         "subscription",
		Default:       "true",
		Description:   "When false, the default Clash template strips rule-providers + rules — emitting just proxies + groups + a MATCH fallback. Ignored when clash_template_yaml is set.",
		DescriptionZh: "关闭后默认 Clash 模板会剥离 rule-providers + rules，仅输出 proxies + groups + MATCH 兜底。如已设置 clash_template_yaml 则忽略。",
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
// unknown rows in the table. The UI uses this for the Settings form
// initial render.
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
	if err := h.validateSettingState(c.Request.Context(), key, body.Value); err != nil {
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
	case model.SettingEmailVerificationRequired:
		return strconv.FormatBool(h.cfg.SMTP.Enabled())
	case model.SettingEmailDomainAllowlist:
		return strings.Join(h.cfg.EmailDomainAllowlist, ",")
	case model.SettingOIDCEnabled:
		return "true"
	case model.SettingOIDCIssuer:
		return h.cfg.OIDC.Issuer
	case model.SettingOIDCClientID:
		return h.cfg.OIDC.ClientID
	case model.SettingOIDCClientSecret:
		return h.cfg.OIDC.ClientSecret
	case model.SettingOIDCRedirectURL:
		return h.cfg.OIDC.RedirectURL
	case model.SettingOIDCScopes:
		return strings.Join(h.cfg.OIDC.Scopes, ",")
	case model.SettingOIDCDisplayName:
		return h.cfg.OIDC.DisplayName
	case model.SettingOIDCIconURL:
		return h.cfg.OIDC.IconURL
	case model.SettingOIDCAuthURL:
		return h.cfg.OIDC.AuthURL
	case model.SettingOIDCTokenURL:
		return h.cfg.OIDC.TokenURL
	case model.SettingOIDCJWKSURL:
		return h.cfg.OIDC.JWKSURL
	case model.SettingOIDCUserInfoURL:
		return h.cfg.OIDC.UserURL
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

func (h *SettingHandler) validateSettingState(ctx context.Context, key, value string) error {
	switch key {
	case model.SettingOpsCollectIntervalSeconds, model.SettingOpsCollectTimeoutSeconds:
		return h.validateCollectionTimeoutWithinInterval(ctx, key, value,
			model.SettingOpsCollectIntervalSeconds,
			model.SettingOpsCollectTimeoutSeconds,
			60,
			12,
		)
	case model.SettingTrafficCollectIntervalSecs, model.SettingTrafficCollectTimeoutSecs:
		return h.validateCollectionTimeoutWithinInterval(ctx, key, value,
			model.SettingTrafficCollectIntervalSecs,
			model.SettingTrafficCollectTimeoutSecs,
			60,
			30,
		)
	default:
		return nil
	}
}

func (h *SettingHandler) validateCollectionTimeoutWithinInterval(ctx context.Context, changedKey, changedValue, intervalKey, timeoutKey string, defaultInterval, defaultTimeout int64) error {
	interval := defaultInterval
	timeout := defaultTimeout
	if h != nil && h.repo != nil {
		var err error
		interval, err = h.repo.GetInt(ctx, intervalKey, defaultInterval)
		if err != nil {
			return err
		}
		timeout, err = h.repo.GetInt(ctx, timeoutKey, defaultTimeout)
		if err != nil {
			return err
		}
	}

	n, err := strconv.ParseInt(strings.TrimSpace(changedValue), 10, 64)
	if err != nil {
		return err
	}
	if changedKey == intervalKey {
		interval = n
	} else if changedKey == timeoutKey {
		timeout = n
	}
	if timeout > interval {
		return fmt.Errorf("%s cannot be greater than %s", timeoutKey, intervalKey)
	}
	return nil
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
		if key == model.SettingNewUserInitialBalanceCents && n < 0 {
			return fmt.Errorf("value %d cannot be negative for %q", n, key)
		}
		switch key {
		case model.SettingOpsCollectIntervalSeconds, model.SettingTrafficCollectIntervalSecs:
			if n < 5 {
				return fmt.Errorf("%s must be at least 5 seconds", key)
			}
		case model.SettingOpsCollectConcurrency, model.SettingTrafficCollectConcurrency:
			if n < 1 || n > 64 {
				return fmt.Errorf("%s must be between 1 and 64", key)
			}
		case model.SettingOpsCollectTimeoutSeconds, model.SettingTrafficCollectTimeoutSecs:
			if n < 1 || n > 300 {
				return fmt.Errorf("%s must be between 1 and 300 seconds", key)
			}
		case model.SettingOpsCollectRetryAttempts, model.SettingTrafficCollectRetryAttempts:
			if n < 0 || n > 5 {
				return fmt.Errorf("%s must be between 0 and 5", key)
			}
		case model.SettingOpsRetentionSeconds, model.SettingTrafficRetentionSeconds:
			if n < 0 {
				return fmt.Errorf("%s cannot be negative", key)
			}
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
			if err := yaml.Unmarshal([]byte(protectTemplateYAMLPlaceholders(value)), &probe); err != nil {
				return fmt.Errorf("clash_template_yaml: invalid YAML: %w", err)
			}
			if probe == nil {
				return errors.New("clash_template_yaml: must be a YAML object (got null or scalar)")
			}
			// Require the placeholder so we know where to inject proxies.
			if !strings.Contains(value, templateProxiesPlaceholder) {
				return errors.New("clash_template_yaml: must contain the " + templateProxiesPlaceholder + " placeholder")
			}
		case model.SettingSingBoxTemplateJSON:
			if strings.TrimSpace(value) == "" {
				return nil
			}
			var probe map[string]any
			if err := json.Unmarshal([]byte(protectTemplateJSONPlaceholders(value)), &probe); err != nil {
				return fmt.Errorf("singbox_template_json: invalid JSON: %w", err)
			}
			if probe == nil {
				return errors.New("singbox_template_json: must be a JSON object")
			}
			if !strings.Contains(value, templateProxiesPlaceholder) {
				return errors.New("singbox_template_json: must contain the " + templateProxiesPlaceholder + " placeholder")
			}
		case model.SettingProxyGroupStrategy:
			switch strings.TrimSpace(value) {
			case "", "auto-only", "select-only", "auto+select":
				return nil
			default:
				return fmt.Errorf("proxy_group_strategy must be one of: auto-only, select-only, auto+select")
			}
		case model.SettingBrandIconURL:
			v := strings.TrimSpace(value)
			if v == "" || strings.HasPrefix(v, "/uploads/") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://") {
				return nil
			}
			return errors.New("brand_icon_url must be empty, an /uploads/ URL, or an http(s) URL")
		case model.SettingBrandTitle, model.SettingBrandSubtitle, model.SettingBrandDescription, model.SettingBrandFooter, model.SettingBrandHomepageContent:
			if err := validateBrandText(key, value); err != nil {
				return err
			}
		case model.SettingBrandDocsURL:
			if err := validateOptionalURL(key, value); err != nil {
				return err
			}
		case model.SettingNewUserPlanIDs:
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				n, err := strconv.ParseInt(part, 10, 64)
				if err != nil || n <= 0 {
					return errors.New("new_user_plan_ids must be empty or comma-separated positive plan IDs")
				}
			}
		case model.SettingOIDCIssuer, model.SettingOIDCRedirectURL, model.SettingOIDCIconURL,
			model.SettingOIDCAuthURL, model.SettingOIDCTokenURL, model.SettingOIDCJWKSURL,
			model.SettingOIDCUserInfoURL:
			if err := validateOptionalURL(key, value); err != nil {
				return err
			}
		case model.SettingOIDCScopes:
			for _, part := range strings.Split(value, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if strings.ContainsAny(part, " \t\r\n") {
					return errors.New("oidc_scopes must be comma-separated scope names without spaces inside each scope")
				}
			}
		}
		return nil
	}
}

func validateOptionalURL(key, value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be empty or an absolute http(s) URL", key)
	}
	switch u.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("%s must use http or https", key)
	}
}

func protectTemplateJSONPlaceholders(value string) string {
	replacements := map[string]string{
		templateProxiesPlaceholder:    `{"__3xui_template_placeholder":"proxies"}`,
		templateProxyNamesPlaceholder: `"__3xui_template_proxy_names__"`,
	}
	out := value
	for placeholder, replacement := range replacements {
		out = strings.ReplaceAll(out, placeholder, replacement)
	}
	return out
}

func protectTemplateYAMLPlaceholders(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
		indent := line[:indentLen]
		switch trimmed {
		case templateProxiesPlaceholder:
			if indent == "" {
				indent = "  "
			}
			lines[i] = indent + "[]"
		case templateProxyGroupsPlaceholder:
			lines[i] = indent + "proxy-groups: []"
		default:
			replacer := strings.NewReplacer(
				templateProxiesPlaceholder, "__3xui_template_proxies__",
				templateProxyNamesPlaceholder, "__3xui_template_proxy_names__",
				templateProxyGroupsPlaceholder, "__3xui_template_proxy_groups__",
			)
			lines[i] = replacer.Replace(line)
		}
	}
	return strings.Join(lines, "\n")
}
