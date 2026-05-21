package model

import "time"

// AdminAction is one audited mutating request against /api/admin/*.
// Captured by the AuditLog middleware after RequireAdmin resolves
// the JWT. Read-only GET requests are NOT logged — the table would
// fill up with admin-page list refreshes without any incident-
// response value.
type AdminAction struct {
	ID             int64     `gorm:"primaryKey"                                json:"id"`
	AdminUsername  string    `gorm:"column:admin_username;not null"            json:"admin_username"`
	Method         string    `gorm:"column:method;not null"                    json:"method"`
	Path           string    `gorm:"column:path;not null"                      json:"path"`
	TargetResource string    `gorm:"column:target_resource;not null"           json:"target_resource"`
	TargetID       string    `gorm:"column:target_id;not null"                 json:"target_id"`
	QueryString    string    `gorm:"column:query_string;not null"              json:"query_string,omitempty"`
	IP             string    `gorm:"column:ip;not null"                        json:"ip"`
	UserAgent      string    `gorm:"column:user_agent;not null"                json:"user_agent,omitempty"`
	StatusCode     int       `gorm:"column:status_code;not null"               json:"status_code"`
	ErrorMsg       string    `gorm:"column:error_msg;not null"                 json:"error_msg,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
}

func (AdminAction) TableName() string { return "admin_actions" }
