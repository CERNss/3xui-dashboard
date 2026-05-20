package model

import "time"

// ClientOwnership bridges a portal user to a specific 3x-ui client on
// one inbound of one node. There is at most one ownership per
// (node_id, inbound_tag, client_email) triple — re-provisioning an
// existing client extends this row in place rather than creating a
// duplicate.
type ClientOwnership struct {
	ID                int64      `gorm:"primaryKey"                                json:"id"`
	UserID            int64      `gorm:"column:user_id;not null;index"             json:"user_id"`
	NodeID            int64      `gorm:"column:node_id;not null"                   json:"node_id"`
	InboundTag        string     `gorm:"column:inbound_tag;not null"               json:"inbound_tag"`
	ClientEmail       string     `gorm:"column:client_email;not null"              json:"client_email"`
	Protocol          string     `gorm:"column:protocol"                           json:"protocol,omitempty"`
	PlanID            *int64     `gorm:"column:plan_id"                            json:"plan_id,omitempty"`
	ExpiresAt         *time.Time `gorm:"column:expires_at"                         json:"expires_at,omitempty"`
	TrafficLimitBytes *int64     `gorm:"column:traffic_limit_bytes"                json:"traffic_limit_bytes,omitempty"`
	Enabled           bool       `gorm:"column:enabled;not null;default:true"      json:"enabled"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null;default:now()"  json:"updated_at"`
}

func (ClientOwnership) TableName() string { return "client_ownerships" }
