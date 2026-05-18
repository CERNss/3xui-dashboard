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
	SettingPublicRegistrationEnabled = "public_registration_enabled"
	SettingEmailDomainAllowlist      = "email_domain_allowlist"
	SettingSubscriptionRemarkModel   = "subscription_remark_model"
	SettingTrafficWarnPct            = "traffic_warn_pct"
	SettingTrafficCriticalPct        = "traffic_critical_pct"
	SettingExpiryWarnDays            = "expiry_warn_days"
)
