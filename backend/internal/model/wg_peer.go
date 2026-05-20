package model

import "time"

// WGPeer is the dashboard-side mirror of one WireGuard peer that
// lives in a node's inbound.settings.peers[] JSON. The dashboard
// re-renders subscriptions from this row so it doesn't have to
// fetch the (rotated, opaque) keypair from the panel on every
// request.
//
// One row per ClientOwnership (uniqueness enforced at the SQL
// layer). PrivateKeyEncrypted is the AES-256-GCM-sealed raw
// private key bytes (nonce || ct || tag); wgcrypto.Cipher.Open
// is the only path that opens it.
type WGPeer struct {
	ID                  int64     `gorm:"primaryKey"                                  json:"id"`
	ClientOwnershipID   int64     `gorm:"column:client_ownership_id;not null;unique"  json:"client_ownership_id"`
	InboundID           int64     `gorm:"column:inbound_id;not null"                  json:"inbound_id"`
	PublicKey           string    `gorm:"column:public_key;not null"                  json:"public_key"`
	PrivateKeyEncrypted []byte    `gorm:"column:private_key_encrypted;not null"       json:"-"`
	AllocatedIP         string    `gorm:"column:allocated_ip;not null;type:inet"      json:"allocated_ip"`
	CreatedAt           time.Time `gorm:"column:created_at;not null;default:now()"    json:"created_at"`
}

func (WGPeer) TableName() string { return "wg_peers" }
