package model

import "time"

// NotificationLog records every (kind, ownership_id) pair we've
// successfully notified. The unique index on (kind, ownership_id)
// is the dedup boundary — see repository.NotificationLogRepo.
type NotificationLog struct {
	ID          int64     `gorm:"primaryKey"                                json:"id"`
	Kind        string    `gorm:"column:kind;not null"                      json:"kind"`
	OwnershipID int64     `gorm:"column:ownership_id;not null"              json:"ownership_id"`
	UserEmail   string    `gorm:"column:user_email;not null;default:''"     json:"user_email"`
	SentAt      time.Time `gorm:"column:sent_at;not null;default:now()"     json:"sent_at"`
}

func (NotificationLog) TableName() string { return "notification_log" }
