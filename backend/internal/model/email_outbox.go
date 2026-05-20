package model

import "time"

// EmailOutbox is one queued outbound email. Producers (verification
// codes, notify channels/email) Enqueue; the EmailQueueJob worker
// drains the queue with exponential-backoff retries.
type EmailOutbox struct {
	ID            int64      `gorm:"primaryKey"                                json:"id"`
	ToAddr        string     `gorm:"column:to_addr;not null"                   json:"to_addr"`
	Subject       string     `gorm:"column:subject;not null"                   json:"subject"`
	Body          string     `gorm:"column:body;not null"                      json:"body"`
	Status        string     `gorm:"column:status;not null"                    json:"status"`
	Attempts      int        `gorm:"column:attempts;not null"                  json:"attempts"`
	LastError     string     `gorm:"column:last_error;not null"                json:"last_error,omitempty"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;not null"           json:"next_attempt_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:now()"  json:"created_at"`
	SentAt        *time.Time `gorm:"column:sent_at"                            json:"sent_at,omitempty"`
}

func (EmailOutbox) TableName() string { return "email_outbox" }

// EmailOutbox status enum.
const (
	EmailOutboxStatusPending = "pending"
	EmailOutboxStatusSending = "sending"
	EmailOutboxStatusSent    = "sent"
	EmailOutboxStatusFailed  = "failed"
)
