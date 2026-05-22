package model

import "time"

// ProvisioningPool is an admin-defined set of node inbounds that a
// purchasable plan may auto-provision onto.
type ProvisioningPool struct {
	ID               int64                    `gorm:"primaryKey"                                json:"id"`
	Name             string                   `gorm:"column:name;not null"                      json:"name"`
	Description      string                   `gorm:"column:description;not null;default:''"    json:"description"`
	Enabled          bool                     `gorm:"column:enabled;not null"                   json:"enabled"`
	AutoCreate       bool                     `gorm:"column:auto_create;not null"               json:"auto_create"`
	PortMin          *int                     `gorm:"column:port_min"                           json:"port_min,omitempty"`
	PortMax          *int                     `gorm:"column:port_max"                           json:"port_max,omitempty"`
	AllowedProtocols StringSlice              `gorm:"column:allowed_protocols;type:jsonb"       json:"allowed_protocols"`
	CreatedAt        time.Time                `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt        time.Time                `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
	Targets          []ProvisioningPoolTarget `gorm:"foreignKey:PoolID"            json:"targets,omitempty"`
}

func (ProvisioningPool) TableName() string { return "provisioning_pools" }

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
