package model

import "time"

// TrafficSample is one cumulative byte-counter reading taken by the
// periodic collection job. Two granularities coexist in this table:
//
//   - Inbound-level samples: ClientEmail IS NULL, InboundTag is set.
//   - Client-level samples:  ClientEmail is set, InboundTag may be set
//     (when known) or NULL.
//
// Deltas are computed at query time by sorting on (taken_at) and
// diffing successive rows. A decrease is treated as a counter reset
// (3x-ui restart, manual zero) and the new value becomes the next
// baseline; no negative delta is ever produced.
type TrafficSample struct {
	ID            int64     `gorm:"primaryKey"                                 json:"id"`
	NodeID        int64     `gorm:"column:node_id;not null;index"              json:"node_id"`
	InboundTag    *string   `gorm:"column:inbound_tag"                         json:"inbound_tag,omitempty"`
	ClientEmail   *string   `gorm:"column:client_email"                        json:"client_email,omitempty"`
	UpCumBytes    int64     `gorm:"column:up_cum_bytes;not null"               json:"up_cum_bytes"`
	DownCumBytes  int64     `gorm:"column:down_cum_bytes;not null"             json:"down_cum_bytes"`
	TakenAt       time.Time `gorm:"column:taken_at;not null"                   json:"taken_at"`
}

func (TrafficSample) TableName() string { return "traffic_samples" }
