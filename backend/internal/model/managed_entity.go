package model

import "time"

// ManagedInbound is one provenance-ledger row: the dashboard created
// this inbound on this node. Upstream inbounds that pre-date the
// dashboard (or were created directly on the 3x-ui panel) have no row
// and render as "external" in the admin views. There is deliberately
// no adopt flow — the ledger is written only by the dashboard's own
// create path.
type ManagedInbound struct {
	ID         int64     `gorm:"primaryKey"                                json:"id"`
	NodeID     int64     `gorm:"column:node_id;not null"                   json:"node_id"`
	InboundTag string    `gorm:"column:inbound_tag;not null"               json:"inbound_tag"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
}

func (ManagedInbound) TableName() string { return "managed_inbounds" }

// ManagedClient is the client-level provenance twin: the dashboard
// created this panel client (provision flow, admin direct-add, or WG
// peer provision). Unlike ClientOwnership it exists regardless of
// whether a portal user is bound — ownership is the billing bridge,
// this is the "created by us" signal.
type ManagedClient struct {
	ID          int64     `gorm:"primaryKey"                                json:"id"`
	NodeID      int64     `gorm:"column:node_id;not null"                   json:"node_id"`
	InboundTag  string    `gorm:"column:inbound_tag;not null"               json:"inbound_tag"`
	ClientEmail string    `gorm:"column:client_email;not null"              json:"client_email"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
}

func (ManagedClient) TableName() string { return "managed_clients" }
