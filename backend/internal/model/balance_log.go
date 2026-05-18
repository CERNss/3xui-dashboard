package model

import "time"

// BalanceLog is the audit trail for every change to users.balance_cents.
// DeltaCents is signed (negative on charge, positive on top-up or
// refund). BalanceAfterCents is the running balance after this entry
// was applied, used so reads do not need to fold the entire history.
type BalanceLog struct {
	ID                 int64     `gorm:"primaryKey"                                 json:"id"`
	UserID             int64     `gorm:"column:user_id;not null"                    json:"user_id"`
	DeltaCents         int64     `gorm:"column:delta_cents;not null"                json:"delta_cents"`
	BalanceAfterCents  int64     `gorm:"column:balance_after_cents;not null"        json:"balance_after_cents"`
	Reason             string    `gorm:"column:reason;not null"                     json:"reason"`
	OrderID            *int64    `gorm:"column:order_id"                            json:"order_id,omitempty"`
	Note               string    `gorm:"column:note;not null;default:''"            json:"note,omitempty"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;default:now()"   json:"created_at"`
}

func (BalanceLog) TableName() string { return "balance_logs" }
