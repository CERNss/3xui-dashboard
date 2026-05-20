package model

import "time"

// Node is one remote 3x-ui panel the dashboard talks to. APIToken is
// the Bearer token issued by the upstream panel admin. It is stored
// plaintext for now — a follow-up will encrypt at rest with a KEK
// derived from a separate env secret.
type Node struct {
	ID          int64      `gorm:"primaryKey"                                  json:"id"`
	Name        string     `gorm:"column:name;not null"                        json:"name"`
	Scheme      string     `gorm:"column:scheme;not null;default:https"        json:"scheme"`
	Host        string     `gorm:"column:host;not null"                        json:"host"`
	Port        int        `gorm:"column:port;not null"                        json:"port"`
	BasePath    string     `gorm:"column:base_path;not null;default:''"        json:"base_path"`
	APIToken    string     `gorm:"column:api_token;not null"                   json:"-"`
	Enabled     bool       `gorm:"column:enabled;not null"                     json:"enabled"`
	LastSeenAt  *time.Time `gorm:"column:last_seen_at"                         json:"last_seen_at,omitempty"`
	CPUPercent  float64    `gorm:"column:cpu_pct;not null;default:0"           json:"cpu_pct"`
	MemPercent  float64    `gorm:"column:mem_pct;not null;default:0"           json:"mem_pct"`
	XrayVersion string     `gorm:"column:xray_version;not null;default:''"     json:"xray_version"`
	UptimeSecs  int64      `gorm:"column:uptime_s;not null;default:0"          json:"uptime_s"`
	Status      string     `gorm:"column:status;not null;default:unknown"      json:"status"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;default:now()"    json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;default:now()"    json:"updated_at"`
}

func (Node) TableName() string { return "nodes" }

// IsOnline reports whether the most recent probe succeeded.
func (n *Node) IsOnline() bool { return n.Status == NodeStatusOnline }
