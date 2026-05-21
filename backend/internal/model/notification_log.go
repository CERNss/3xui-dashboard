package model

import "time"

// NotificationLog records every (surface, kind, ownership_id) tuple
// we've successfully notified. The unique index on
// (surface, kind, ownership_id) is the dedup boundary — see
// repository.NotificationLogRepo. Surface distinguishes user-facing
// messages (SMTP only) from ops-facing notifications (multi-channel
// + admin webhooks).
type NotificationLog struct {
	ID          int64     `gorm:"primaryKey"                                json:"id"`
	Surface     string    `gorm:"column:surface;not null;default:notification" json:"surface"`
	Kind        string    `gorm:"column:kind;not null"                      json:"kind"`
	OwnershipID int64     `gorm:"column:ownership_id;not null"              json:"ownership_id"`
	UserEmail   string    `gorm:"column:user_email;not null;default:''"     json:"user_email"`
	SentAt      time.Time `gorm:"column:sent_at;not null;default:now()"     json:"sent_at"`
}

// Surface values for NotificationLog.Surface. Kept here so model
// and repo callers share a single source of truth.
const (
	SurfaceMessage      = "message"      // user-facing, SMTP only
	SurfaceNotification = "notification" // ops-facing, multi-channel + admin webhooks
)

func (NotificationLog) TableName() string { return "notification_log" }
