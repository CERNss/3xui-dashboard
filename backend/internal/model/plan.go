package model

import "time"

// Plan is a purchasable plan template. TrafficLimitBytes == 0 means
// unlimited; DurationDays == 0 means non-expiring.
type Plan struct {
	ID                 int64     `gorm:"primaryKey"                                json:"id"`
	Name               string    `gorm:"column:name;not null"                      json:"name"`
	Description        string    `gorm:"column:description;not null;default:''"    json:"description"`
	DurationDays       int       `gorm:"column:duration_days;not null"             json:"duration_days"`
	TrafficLimitBytes  int64     `gorm:"column:traffic_limit_bytes;not null"       json:"traffic_limit_bytes"`
	PriceCents         int64     `gorm:"column:price_cents;not null"               json:"price_cents"`
	IPLimit            int       `gorm:"column:ip_limit;not null"                  json:"ip_limit"`
	ProvisioningPoolID *int64    `gorm:"column:provisioning_pool_id"               json:"provisioning_pool_id,omitempty"`
	Enabled            bool      `gorm:"column:enabled;not null"                   json:"enabled"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
}

func (Plan) TableName() string { return "plans" }
