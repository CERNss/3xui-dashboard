package model

import "time"

// User is a portal user account. The single admin account lives in
// environment variables (ADMIN_USERNAME/ADMIN_PASSWORD) and is not
// stored here.
//
// Email and OIDCSubject are independently nullable: an account can be
// email/password, OIDC-only, or both after linking. Unique constraints
// are partial so multiple NULLs are allowed.
type User struct {
	ID            int64      `gorm:"primaryKey"               json:"id"`
	Email         *string    `gorm:"column:email"             json:"email,omitempty"`
	PasswordHash  *string    `gorm:"column:password_hash"     json:"-"`
	OIDCSubject   *string    `gorm:"column:oidc_subject"      json:"oidc_subject,omitempty"`
	EmailVerified bool       `gorm:"column:email_verified;not null;default:false" json:"email_verified"`
	Status        string     `gorm:"column:status;not null;default:active"        json:"status"`
	BalanceCents  int64      `gorm:"column:balance_cents;not null;default:0"      json:"balance_cents"`
	AutoRenew     bool       `gorm:"column:auto_renew;not null"                   json:"auto_renew"`
	SubID         string     `gorm:"column:sub_id;not null;uniqueIndex:users_sub_id_unique" json:"sub_id"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:now()"     json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null;default:now()"     json:"updated_at"`
}

// TableName overrides the default pluralized form (already correct, but
// pin it explicitly to immunize against GORM naming-strategy changes).
func (User) TableName() string { return "users" }

// IsActive reports whether the user can authenticate.
func (u *User) IsActive() bool { return u.Status == UserStatusActive }
