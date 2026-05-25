package model

import "time"

// OIDCProvider stores one configured OpenID Connect provider. Secrets
// are intentionally omitted from JSON responses; service-layer public
// DTOs expose only key/display/icon metadata.
type OIDCProvider struct {
	ProviderKey  string      `gorm:"column:provider_key;primaryKey" json:"provider_key"`
	DisplayName  string      `gorm:"column:display_name;not null" json:"display_name"`
	IconURL      string      `gorm:"column:icon_url;not null;default:''" json:"icon_url,omitempty"`
	Issuer       string      `gorm:"column:issuer;not null" json:"issuer"`
	ClientID     string      `gorm:"column:client_id;not null" json:"client_id"`
	ClientSecret string      `gorm:"column:client_secret;not null;default:''" json:"-"`
	RedirectURL  string      `gorm:"column:redirect_url;not null;default:''" json:"redirect_url,omitempty"`
	Scopes       StringSlice `gorm:"column:scopes;type:jsonb;not null" json:"scopes"`
	AuthURL      string      `gorm:"column:auth_url;not null;default:''" json:"auth_url,omitempty"`
	TokenURL     string      `gorm:"column:token_url;not null;default:''" json:"-"`
	JWKSURL      string      `gorm:"column:jwks_url;not null;default:''" json:"-"`
	UserInfoURL  string      `gorm:"column:user_info_url;not null;default:''" json:"-"`
	Enabled      bool        `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt    time.Time   `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (OIDCProvider) TableName() string { return "oidc_providers" }

// UserOIDCIdentity links a local user to the immutable provider
// subject. Provider email is stored for display/audit and may differ
// from users.email.
type UserOIDCIdentity struct {
	ID                    int64     `gorm:"primaryKey" json:"id"`
	UserID                int64     `gorm:"column:user_id;not null" json:"user_id"`
	ProviderKey           string    `gorm:"column:provider_key;not null" json:"provider_key"`
	Subject               string    `gorm:"column:subject;not null" json:"subject"`
	ProviderEmail         string    `gorm:"column:provider_email;not null" json:"provider_email"`
	ProviderEmailVerified bool      `gorm:"column:provider_email_verified;not null;default:false" json:"provider_email_verified"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (UserOIDCIdentity) TableName() string { return "user_oidc_identities" }
