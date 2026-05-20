// Package model defines the GORM-mapped Go structs that mirror the
// schema produced by migrations/0001_init.up.sql. The migrations are
// the source of truth — these models exist purely so handlers and
// repositories can speak Go. GORM AutoMigrate is never run.
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice serializes []string as JSONB. Use for columns typed
// `jsonb` that hold a string array (e.g. webhooks.events).
type StringSlice []string

// Value implements driver.Valuer.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(s))
}

// Scan implements sql.Scanner.
func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("model.StringSlice: unsupported scan type %T", value)
	}
	if len(bytes) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(bytes, (*[]string)(s))
}

// User account status values.
const (
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
)

// Node status values populated by the periodic probe.
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusUnknown = "unknown"
)

// Order lifecycle values.
//
// Balance-based flow: pending → completed | failed | refunded.
// Payment-gateway flow: payment_pending → paid → completed,
// or payment_pending → payment_failed | payment_expired (terminals).
// The `paid` state is transient — only visible during the brief
// window between payment confirmation and provisioning success.
const (
	OrderStatusPending         = "pending"
	OrderStatusCompleted       = "completed"
	OrderStatusFailed          = "failed"
	OrderStatusRefunded        = "refunded"
	OrderStatusPaymentPending  = "payment_pending"
	OrderStatusPaid            = "paid"
	OrderStatusPaymentFailed   = "payment_failed"
	OrderStatusPaymentExpired  = "payment_expired"
)

// Payment method values stored in orders.payment_method.
const (
	PaymentMethodBalance = "balance"
	PaymentMethodAlipay  = "alipay"
)

// BalanceLog reason values.
const (
	BalanceReasonAdminAdjust = "admin_adjust"
	BalanceReasonOrderCharge = "order_charge"
	BalanceReasonOrderRefund = "order_refund"
	BalanceReasonBonus       = "bonus"
)

// WebhookDelivery status values.
const (
	WebhookDeliveryStatusPending = "pending"
	WebhookDeliveryStatusSuccess = "success"
	WebhookDeliveryStatusFailed  = "failed"
)
