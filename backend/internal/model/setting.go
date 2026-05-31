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
	SettingPublicRegistrationEnabled   = "public_registration_enabled"
	SettingEmailVerificationRequired   = "email_verification_required"
	SettingEmailDomainAllowlist        = "email_domain_allowlist"
	SettingSubscriptionRemarkModel     = "subscription_remark_model"
	SettingTrafficWarnPct              = "traffic_warn_pct"
	SettingTrafficCriticalPct          = "traffic_critical_pct"
	SettingExpiryWarnDays              = "expiry_warn_days"
	SettingBrandIconURL                = "brand_icon_url"
	SettingBrandTitle                  = "brand_title"
	SettingBrandSubtitle               = "brand_subtitle"
	SettingBrandDescription            = "brand_description"
	SettingBrandFooter                 = "brand_footer"
	SettingBrandDocsURL                = "brand_docs_url"
	SettingBrandHomepageContent        = "brand_homepage_content"
	SettingNewUserInitialBalanceCents  = "new_user_initial_balance_cents"
	SettingNewUserPlanIDs              = "new_user_plan_ids"
	SettingOIDCEnabled                 = "oidc_enabled"
	SettingOIDCIssuer                  = "oidc_issuer"
	SettingOIDCClientID                = "oidc_client_id"
	SettingOIDCClientSecret            = "oidc_client_secret"
	SettingOIDCRedirectURL             = "oidc_redirect_url"
	SettingOIDCScopes                  = "oidc_scopes"
	SettingOIDCDisplayName             = "oidc_display_name"
	SettingOIDCIconURL                 = "oidc_icon_url"
	SettingOIDCAuthURL                 = "oidc_auth_url"
	SettingOIDCTokenURL                = "oidc_token_url"
	SettingOIDCJWKSURL                 = "oidc_jwks_url"
	SettingOIDCUserInfoURL             = "oidc_userinfo_url"
	// SMTP delivery — runtime-editable in the panel; smtp_password is
	// stored encrypted (SettingRepo.SetSecret). Empty rows fall back to
	// the SMTP_* env / config.yaml values.
	SettingSMTPHost     = "smtp_host"
	SettingSMTPPort     = "smtp_port"
	SettingSMTPFrom     = "smtp_from"
	SettingSMTPUsername = "smtp_username"
	SettingSMTPPassword = "smtp_password"
	// Notify ops fan-out — runtime-editable in the panel. Bot tokens and
	// webhook URLs (the URL alone is the credential) are stored encrypted
	// (SettingRepo.SetSecret). Empty rows fall back to the NOTIFY_* /
	// TELEGRAM_* / DISCORD_* / FEISHU_* env values.
	SettingNotifyRoutes             = "notify_routes"
	SettingNotifyOpsRecipient       = "notify_ops_recipient"
	SettingNotifyTelegramBotToken   = "notify_telegram_bot_token"
	SettingNotifyTelegramChatID     = "notify_telegram_chat_id"
	SettingNotifyDiscordWebhookURL  = "notify_discord_webhook_url"
	SettingNotifyFeishuWebhookURL   = "notify_feishu_webhook_url"
	SettingNotifyFeishuCardTemplate = "notify_feishu_card_template"
	// Payment gateways — runtime-editable in the panel. The Alipay
	// private key and the Stripe secret key + webhook secret are stored
	// encrypted (SettingRepo.SetSecret). Empty rows fall back to the
	// ALIPAY_* / STRIPE_* env values.
	SettingAlipayAppID                = "alipay_app_id"
	SettingAlipayPrivateKey           = "alipay_private_key"
	SettingAlipayPublicKey            = "alipay_public_key"
	SettingAlipayGateway              = "alipay_gateway"
	SettingAlipayNotifyURL            = "alipay_notify_url"
	SettingStripeSecretKey            = "stripe_secret_key"
	SettingStripeWebhookSecret        = "stripe_webhook_secret"
	SettingStripeCurrency             = "stripe_currency"
	SettingStripeSuccessURL           = "stripe_success_url"
	SettingStripeCancelURL            = "stripe_cancel_url"
	SettingStripeSessionExpiryMinutes = "stripe_session_expiry_minutes"
	SettingOpsCollectEnabled           = "ops_collect_enabled"
	SettingOpsCollectIntervalSeconds   = "ops_collect_interval_seconds"
	SettingOpsCollectConcurrency       = "ops_collect_concurrency"
	SettingOpsCollectTimeoutSeconds    = "ops_collect_timeout_seconds"
	SettingOpsCollectRetryAttempts     = "ops_collect_retry_attempts"
	SettingOpsRetentionSeconds         = "ops_retention_seconds"
	SettingTrafficCollectEnabled       = "traffic_collect_enabled"
	SettingTrafficCollectIntervalSecs  = "traffic_collect_interval_seconds"
	SettingTrafficCollectConcurrency   = "traffic_collect_concurrency"
	SettingTrafficCollectTimeoutSecs   = "traffic_collect_timeout_seconds"
	SettingTrafficCollectRetryAttempts = "traffic_collect_retry_attempts"
	SettingTrafficRetentionSeconds     = "traffic_retention_seconds"

	// Subscription format templates — admins override the embedded
	// defaults shipped by internal/sub/template/defaults.go.
	// Operator base-template overrides for the subscription renderers.
	// Routing policy (groups + rules) is configured via subscription
	// profiles, not these — these only replace the base skeleton.
	SettingClashTemplateYAML   = "clash_template_yaml"
	SettingSingBoxTemplateJSON = "singbox_template_json"
)
