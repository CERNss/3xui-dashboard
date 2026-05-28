package model

import "time"

// ProvisioningPool is a curated list of real upstream inbounds that
// a purchasable plan can land new clients into. Inbounds themselves
// are created manually by the operator (with optional template
// pre-fill); this pool only decides which inbound a purchase picks.
type ProvisioningPool struct {
	ID               int64                    `gorm:"primaryKey"                                json:"id"`
	Name             string                   `gorm:"column:name;not null"                      json:"name"`
	Description      string                   `gorm:"column:description;not null;default:''"    json:"description"`
	Enabled          bool                     `gorm:"column:enabled;not null"                   json:"enabled"`
	AllowedProtocols StringSlice              `gorm:"column:allowed_protocols;type:jsonb"       json:"allowed_protocols"`
	CreatedAt        time.Time                `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt        time.Time                `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
	Targets          []ProvisioningPoolTarget `gorm:"foreignKey:PoolID"                         json:"targets,omitempty"`
}

func (ProvisioningPool) TableName() string { return "provisioning_pools" }

// InboundTemplate is a saved preset that pre-fills the "create
// inbound" form. It carries the wire-shape (protocol / stream /
// sniffing / quotas) but not port, tag, listen, or clients — those
// are filled in at create time by the operator.
type InboundTemplate struct {
	ID             int64     `gorm:"primaryKey"                                json:"id"`
	Name           string    `gorm:"column:name;not null"                      json:"name"`
	Description    string    `gorm:"column:description;not null;default:''"    json:"description"`
	Enabled        bool      `gorm:"column:enabled;not null"                   json:"enabled"`
	Protocol       string    `gorm:"column:protocol;not null"                  json:"protocol"`
	Remark         string    `gorm:"column:remark;not null;default:''"         json:"remark"`
	Listen         string    `gorm:"column:listen;not null;default:''"         json:"listen"`
	Total          int64     `gorm:"column:total;not null;default:0"           json:"total"`
	ExpiryTime     int64     `gorm:"column:expiry_time;not null;default:0"     json:"expiryTime"`
	TrafficReset   string    `gorm:"column:traffic_reset;not null;default:'never'" json:"trafficReset"`
	Settings       string    `gorm:"column:settings;not null;default:'{}'"     json:"settings"`
	StreamSettings string    `gorm:"column:stream_settings;not null;default:'{}'" json:"streamSettings"`
	Sniffing       string    `gorm:"column:sniffing;not null;default:'{}'"      json:"sniffing"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
}

func (InboundTemplate) TableName() string { return "inbound_templates" }

// ProvisioningPoolTarget is one eligible existing inbound inside a pool.
// MaxClients == 0 means no dashboard-side capacity limit.
type ProvisioningPoolTarget struct {
	ID          int64     `gorm:"primaryKey"                                json:"id"`
	PoolID      int64     `gorm:"column:pool_id;not null"                    json:"pool_id"`
	NodeID      int64     `gorm:"column:node_id;not null"                    json:"node_id"`
	InboundTag  string    `gorm:"column:inbound_tag;not null"                json:"inbound_tag"`
	Protocol    string    `gorm:"column:protocol;not null;default:''"        json:"protocol"`
	MaxClients  int       `gorm:"column:max_clients;not null"                json:"max_clients"`
	Priority    int       `gorm:"column:priority;not null"                   json:"priority"`
	Enabled     bool      `gorm:"column:enabled;not null"                    json:"enabled"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:now()"   json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;default:now()"   json:"updated_at"`
	NodeName    string    `gorm:"->;column:node_name"                        json:"node_name,omitempty"`
	UsedClients int       `gorm:"->;column:used_clients"                     json:"used_clients"`
}

func (ProvisioningPoolTarget) TableName() string { return "provisioning_pool_targets" }
