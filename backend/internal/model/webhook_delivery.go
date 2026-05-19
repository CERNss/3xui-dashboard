package model

import (
	"encoding/json"
	"time"
)

// WebhookDelivery is one delivery attempt. Payload is the full
// versioned envelope as posted. ResponseBody is truncated by the app
// layer before insert.
//
// Status semantics:
//   - "pending" — eligible for delivery when NextAttemptAt <= now().
//                 Both the initial insert and not-yet-exhausted retries
//                 sit in this state, so a crash never strands a retry.
//   - "success" — 2xx response received (terminal).
//   - "failed"  — attempt count reached MaxAttempts (terminal).
type WebhookDelivery struct {
	ID            int64           `gorm:"primaryKey"                                 json:"id"`
	WebhookID     int64           `gorm:"column:webhook_id;not null"                 json:"webhook_id"`
	EventType     string          `gorm:"column:event_type;not null"                 json:"event_type"`
	Payload       json.RawMessage `gorm:"column:payload;type:jsonb;not null"         json:"payload"`
	Status        string          `gorm:"column:status;not null;default:pending"     json:"status"`
	HTTPStatus    int             `gorm:"column:http_status;not null;default:0"      json:"http_status"`
	ResponseBody  string          `gorm:"column:response_body;not null;default:''"   json:"response_body,omitempty"`
	Attempt       int             `gorm:"column:attempt;not null;default:0"          json:"attempt"`
	ScheduledAt   time.Time       `gorm:"column:scheduled_at;not null;default:now()" json:"scheduled_at"`
	NextAttemptAt time.Time       `gorm:"column:next_attempt_at;not null;default:now()" json:"next_attempt_at"`
	DeliveredAt   *time.Time      `gorm:"column:delivered_at"                        json:"delivered_at,omitempty"`
	Error         string          `gorm:"column:error;not null;default:''"           json:"error,omitempty"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }
