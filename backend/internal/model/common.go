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

// Int64Slice serializes []int64 as JSONB. It is used for compact
// id allowlists where an empty list means "all eligible rows".
type Int64Slice []int64

// Value implements driver.Valuer.
func (s Int64Slice) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]int64(s))
}

// Scan implements sql.Scanner.
func (s *Int64Slice) Scan(value any) error {
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
		return fmt.Errorf("model.Int64Slice: unsupported scan type %T", value)
	}
	if len(bytes) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(bytes, (*[]int64)(s))
}

// StringMap serializes map[string]string as JSONB. Used for columns
// holding arbitrary admin-defined key/value pairs — currently just
// webhooks.headers, but reusable for future similar shapes.
type StringMap map[string]string

// Value implements driver.Valuer.
func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]string(m))
}

// Scan implements sql.Scanner.
func (m *StringMap) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("model.StringMap: unsupported scan type %T", value)
	}
	if len(bytes) == 0 {
		*m = nil
		return nil
	}
	return json.Unmarshal(bytes, (*map[string]string)(m))
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
