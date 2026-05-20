// Package payload centralizes the typed payloads published on the
// event bus.
//
// Why a separate package: the notify service consumes events from
// multiple publishers (job, billing, traffic). Defining payloads in
// the publisher packages forces notify to either import each one
// (creating a cycle if any publisher subscribed to other events) or
// extract fields by reflection. Reflection silently drops field
// renames; importing back would couple notify to all publishers.
//
// One shared payload package means publishers and subscribers
// import the same canonical types — compile-time safety.
package payload

import "time"

// ---- client lifecycle -----------------------------------------------------

// ClientExpired is published when a client ownership row crosses
// expires_at. Emitted by both the DB-side ExpiryJob (with full
// ownership info) and the traffic service (panel-reported expiry —
// see TrafficClientExpired below for the lookup-required variant).
type ClientExpired struct {
	OwnershipID int64     `json:"ownership_id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email,omitempty"`
	NodeID      int64     `json:"node_id"`
	InboundTag  string    `json:"inbound_tag"`
	ClientEmail string    `json:"client_email"`
	ExpiredAt   time.Time `json:"expired_at"`
}

// ClientExpiringSoon describes one client whose plan runs out
// within the configured warning window (default 3 days).
type ClientExpiringSoon struct {
	OwnershipID   int64     `json:"ownership_id"`
	UserID        int64     `json:"user_id"`
	UserEmail     string    `json:"user_email,omitempty"`
	NodeID        int64     `json:"node_id"`
	InboundTag    string    `json:"inbound_tag"`
	ClientEmail   string    `json:"client_email"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
}

// TrafficClientExpired is the panel-reported variant — the traffic
// service notices a client's `expiry_time` is past and re-emits.
// Distinct from ClientExpired because we don't have OwnershipID at
// the moment of publish; subscribers lookup the row by
// (node, inbound, email) triple.
type TrafficClientExpired struct {
	NodeID      int64     `json:"node_id"`
	NodeName    string    `json:"node_name"`
	InboundTag  string    `json:"inbound_tag"`
	ClientEmail string    `json:"client_email"`
	ExpiredAt   time.Time `json:"expired_at"`
}

// ClientThreshold is the per-client over-limit payload — published
// when the traffic service detects a client has used past its quota.
type ClientThreshold struct {
	NodeID      int64  `json:"node_id"`
	NodeName    string `json:"node_name"`
	InboundTag  string `json:"inbound_tag"`
	ClientEmail string `json:"client_email"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
	Limit       int64  `json:"limit"`
}

// ---- node ------------------------------------------------------------------

// NodeStatusChanged covers node.online / node.offline / node.recovered.
type NodeStatusChanged struct {
	NodeID int64  `json:"node_id"`
	Name   string `json:"name"`
	Prior  string `json:"prior_status"`
	Now    string `json:"new_status"`
}

// NodeProbeFailed carries the failure reason for node.probe_failed.
type NodeProbeFailed struct {
	NodeID int64  `json:"node_id"`
	Name   string `json:"name"`
	Error  string `json:"error"`
}

// ---- order -----------------------------------------------------------------

// Order is the universal order-event shape used by:
//   - order.created
//   - order.completed
//   - order.failed
//   - order.payment_confirmed
//   - order.payment_failed
//   - order.payment_expired
//
// `Reason` is set on the failed/expired variants; empty on success.
type Order struct {
	OrderID    int64  `json:"order_id"`
	UserID     int64  `json:"user_id"`
	PlanID     int64  `json:"plan_id"`
	PriceCents int64  `json:"price_cents"`
	Reason     string `json:"reason,omitempty"`
}
