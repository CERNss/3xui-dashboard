package model

import "time"

// Webhook is an outbound event-subscription target configured by an
// admin. Events is a JSONB array of event-name patterns, e.g.
// ["node.*", "order.completed"]. AllowPrivate must be set to true to
// permit URLs targeting private/RFC1918 hosts (otherwise the SSRF
// guard in the delivery transport rejects them).
type Webhook struct {
	ID           int64       `gorm:"primaryKey"                                  json:"id"`
	Name         string      `gorm:"column:name;not null"                        json:"name"`
	URL          string      `gorm:"column:url;not null"                         json:"url"`
	Secret       string      `gorm:"column:secret;not null"                      json:"-"`
	Events       StringSlice `gorm:"column:events;type:jsonb;not null;default:'[]'::jsonb" json:"events"`
	Enabled      bool        `gorm:"column:enabled;not null"                     json:"enabled"`
	AllowPrivate bool        `gorm:"column:allow_private;not null;default:false" json:"allow_private"`
	CreatedAt    time.Time   `gorm:"column:created_at;not null;default:now()"    json:"created_at"`
	UpdatedAt    time.Time   `gorm:"column:updated_at;not null;default:now()"    json:"updated_at"`
}

func (Webhook) TableName() string { return "webhooks" }
