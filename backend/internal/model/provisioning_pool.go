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
	TemplateID       *int64                   `gorm:"column:template_id"                         json:"template_id,omitempty"`
	PortMin          *int                     `gorm:"column:port_min"                           json:"port_min,omitempty"`
	PortMax          *int                     `gorm:"column:port_max"                           json:"port_max,omitempty"`
	MaxClients        int                      `gorm:"column:max_clients;not null"               json:"max_clients"`
	AllowedProtocols StringSlice              `gorm:"column:allowed_protocols;type:jsonb"       json:"allowed_protocols"`
	NodeIDs          Int64Slice                `gorm:"column:node_ids;type:jsonb"                json:"node_ids"`
	CreatedAt        time.Time                `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt        time.Time                `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
	Template         *InboundTemplate         `gorm:"foreignKey:TemplateID"                     json:"template,omitempty"`
	Targets          []ProvisioningPoolTarget `gorm:"foreignKey:PoolID"            json:"targets,omitempty"`
}

func (ProvisioningPool) TableName() string { return "provisioning_pools" }

// InboundTemplate stores the reusable inbound wire-shape used by
// provisioning pools to create real upstream inbounds on demand.
// Port and tag are deliberately omitted from the template: they are
// allocated by the pool at provisioning time.
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
	TemplateID  *int64    `gorm:"column:template_id"                         json:"template_id,omitempty"`
	NodeID      int64     `gorm:"column:node_id;not null"                    json:"node_id"`
	InboundTag  string    `gorm:"column:inbound_tag;not null"                json:"inbound_tag"`
	Protocol    string    `gorm:"column:protocol;not null;default:''"        json:"protocol"`
	MaxClients  int       `gorm:"column:max_clients;not null"                json:"max_clients"`
	Priority    int       `gorm:"column:priority;not null"                   json:"priority"`
	Enabled     bool      `gorm:"column:enabled;not null"                    json:"enabled"`
	Generated   bool      `gorm:"column:generated;not null"                  json:"generated"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:now()"   json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;default:now()"   json:"updated_at"`
	TemplateName string   `gorm:"->;column:template_name"                    json:"template_name,omitempty"`
	NodeName    string    `gorm:"->;column:node_name"                        json:"node_name,omitempty"`
	UsedClients int       `gorm:"->;column:used_clients"                     json:"used_clients"`
}

func (ProvisioningPoolTarget) TableName() string { return "provisioning_pool_targets" }
