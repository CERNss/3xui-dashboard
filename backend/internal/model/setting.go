package model

import "time"

// Setting is one row in the runtime-mutable key/value store used by
// admins to override env-defined defaults at runtime (public-
// registration toggle, email domain allowlist, subscription remark
// template, traffic thresholds, …). All values are stored as TEXT;
// typed coercion lives in the repository layer.
type Setting struct {
	Key       string    `gorm:"primaryKey;column:key"                       json:"key"`
	Value     string    `gorm:"column:value;not null"                       json:"value"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:now()"    json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }

// Well-known setting keys. Service-layer code should reference these
// constants instead of bare strings.
const (
	SettingPublicRegistrationEnabled  = "public_registration_enabled"
	SettingEmailVerificationRequired  = "email_verification_required"
	SettingEmailDomainAllowlist       = "email_domain_allowlist"
	SettingSubscriptionRemarkModel    = "subscription_remark_model"
	SettingTrafficWarnPct             = "traffic_warn_pct"
	SettingTrafficCriticalPct         = "traffic_critical_pct"
	SettingExpiryWarnDays             = "expiry_warn_days"
	SettingBrandIconURL               = "brand_icon_url"
	SettingBrandTitle                 = "brand_title"
	SettingBrandSubtitle              = "brand_subtitle"
	SettingBrandDescription           = "brand_description"
	SettingBrandFooter                = "brand_footer"
	SettingNewUserInitialBalanceCents = "new_user_initial_balance_cents"
	SettingNewUserPlanIDs             = "new_user_plan_ids"
	SettingOIDCIssuer                 = "oidc_issuer"
	SettingOIDCClientID               = "oidc_client_id"
	SettingOIDCClientSecret           = "oidc_client_secret"
	SettingOIDCRedirectURL            = "oidc_redirect_url"
	SettingOIDCScopes                 = "oidc_scopes"
	SettingOIDCDisplayName            = "oidc_display_name"
	SettingOIDCIconURL                = "oidc_icon_url"
	SettingOIDCAuthURL                = "oidc_auth_url"
	SettingOIDCTokenURL               = "oidc_token_url"
	SettingOIDCJWKSURL                = "oidc_jwks_url"
	SettingOIDCUserInfoURL            = "oidc_userinfo_url"

	// Subscription format templates — admins override the embedded
	// defaults shipped by internal/sub/template/defaults.go.
	SettingClashTemplateYAML   = "clash_template_yaml"
	SettingSingBoxTemplateJSON = "singbox_template_json"
	// One of "auto-only" / "select-only" / "auto+select". Only affects
	// the default Clash template; ignored when clash_template_yaml is
	// non-empty.
	SettingProxyGroupStrategy = "proxy_group_strategy"
	// When false, the default Clash template emits no rule-providers /
	// rules — just proxies + a single MATCH fallback. Ignored when
	// clash_template_yaml is non-empty.
	SettingRuleProvidersEnabled = "rule_providers_enabled"
)
