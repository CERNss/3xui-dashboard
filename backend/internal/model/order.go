package model

import "time"

// Order is one purchase attempt. IdempotencyKey is unique so a
// retried POST returns the original order. ClientOwnershipID is
// populated when provisioning succeeds; the same row may be linked by
// multiple orders if the client was re-provisioned (extended) rather
// than freshly created.
type Order struct {
	ID                int64      `gorm:"primaryKey"                                       json:"id"`
	UserID            int64      `gorm:"column:user_id;not null"                          json:"user_id"`
	PlanID            int64      `gorm:"column:plan_id;not null"                          json:"plan_id"`
	IdempotencyKey    string     `gorm:"column:idempotency_key;not null;uniqueIndex"      json:"idempotency_key"`
	PriceCents        int64      `gorm:"column:price_cents;not null"                      json:"price_cents"`
	Status            string     `gorm:"column:status;not null;default:pending"           json:"status"`
	ClientOwnershipID *int64     `gorm:"column:client_ownership_id"                       json:"client_ownership_id,omitempty"`
	ErrorMessage      string     `gorm:"column:error_message;not null;default:''"         json:"error_message"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;default:now()"         json:"created_at"`
	CompletedAt       *time.Time `gorm:"column:completed_at"                              json:"completed_at,omitempty"`

	// Payment-gateway columns. For balance orders these stay at zero
	// values (method='balance', empty strings, NULL expires_at).
	PaymentMethod          string     `gorm:"column:payment_method;not null;default:balance" json:"payment_method"`
	PaymentProviderOrderID string     `gorm:"column:payment_provider_order_id;not null;default:''" json:"payment_provider_order_id,omitempty"`
	PaymentQRURL           string     `gorm:"column:payment_qr_url;not null;default:''"       json:"payment_qr_url,omitempty"`
	PaymentExpiresAt       *time.Time `gorm:"column:payment_expires_at"                       json:"payment_expires_at,omitempty"`
}

func (Order) TableName() string { return "orders" }
