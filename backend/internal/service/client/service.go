// Package client owns the ProvisionClient flow that creates or
// extends a 3x-ui client on a node inbound and upserts the matching
// ClientOwnership row in one go. It is the single shared path used
// by both admin "create client" actions and user-portal plan
// purchases.
package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// Errors callers can branch on.
var (
	ErrUserNotFound  = errors.New("client: user not found")
	ErrPlanNotFound  = errors.New("client: plan not found")
	ErrInboundLookup = errors.New("client: inbound lookup failed")
)

// PlanParams is the subset of a Plan row provisioning needs. Decoupled
// from model.Plan so callers can drive provisioning from arbitrary
// sources (admin direct create, plan purchase, scripted backfill).
type PlanParams struct {
	PlanID            *int64 // optional — for traceability on the ownership row
	DurationDays      int    // 0 = non-expiring
	TrafficLimitBytes int64  // 0 = unlimited
}

// Service composes the runtime manager, the user/plan/ownership
// repositories, and the logger. Construct once at startup.
type Service struct {
	rt        *runtime.Manager
	ownership *repository.ClientOwnershipRepo
	users     UserLookup
	plans     PlanLookup
	wg        *WGProvisioner // optional; nil when WG_MASTER_KEY not set
	log       *slog.Logger
}

// SetWGProvisioner attaches the WG provisioner so ProvisionClient
// can delegate WG inbounds to its RMW path. Idempotent. Called by
// app.Build once at startup when cfg.WireGuard.Enabled().
func (s *Service) SetWGProvisioner(p *WGProvisioner) { s.wg = p }

// PreflightProvision is a cheap "would ProvisionClient fail?"
// probe that does NOT mutate the node. Used by billing.Purchase
// to reject impossible provisions before charging the user.
//
// Returns nil when the target is provisionable. Errors mean
// the resolved (nodeID, inboundTag) is unreachable / disabled /
// requires WG_MASTER_KEY which isn't set / inbound vanished.
func (s *Service) PreflightProvision(ctx context.Context, nodeID int64, inboundTag string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node: %w", err)
	}
	in, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return fmt.Errorf("inbound %q: %w", inboundTag, err)
	}
	if !in.Enable {
		return fmt.Errorf("inbound %q is disabled on the panel", inboundTag)
	}
	if in.IsWireguard() && s.wg == nil {
		return fmt.Errorf("WG inbound %q requires WG_MASTER_KEY but it is not configured on this dashboard", inboundTag)
	}
	return nil
}

// UserLookup / PlanLookup are tiny interfaces to keep the client
// package decoupled from full user / plan services. main wires
// minimal adapters around the GORM repos.
type UserLookup interface {
	GetUser(ctx context.Context, id int64) (*model.User, error)
}

type PlanLookup interface {
	GetPlan(ctx context.Context, id int64) (*model.Plan, error)
}

// New constructs the service.
func New(rt *runtime.Manager, ownership *repository.ClientOwnershipRepo, users UserLookup, plans PlanLookup, lg *slog.Logger) *Service {
	return &Service{
		rt:        rt,
		ownership: ownership,
		users:     users,
		plans:     plans,
		log:       lg.With(slog.String("component", "service.client")),
	}
}

// ProvisionClient is the workhorse: ensures a 3x-ui client exists on
// the named inbound for the given user, extends its expiry / traffic
// limit by params, and upserts the ClientOwnership row.
//
// Behaviour:
//   - First call:  creates the 3x-ui client + inserts a fresh
//     ownership row. The client.email is derived from the user's
//     sub_id (stable across email changes and OIDC-only accounts).
//   - Subsequent call (same user/node/inbound): calls UpdateClient on
//     the panel with new expiry = max(now, existingExpiry) + duration,
//     refreshes the traffic limit, and upserts (which becomes update).
//
// Returns the persisted ClientOwnership row.
func (s *Service) ProvisionClient(ctx context.Context, userID, nodeID int64, inboundTag string, params PlanParams) (*model.ClientOwnership, error) {
	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("provision: lookup user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("%w: id=%d", ErrUserNotFound, userID)
	}

	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	in, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInboundLookup, err)
	}

	// Stable 3x-ui email handle for this (user, node, inbound) triple.
	clientEmail := user.SubID

	// WireGuard branch: peer mutation goes through the advisory-locked
	// RMW path, not the unified /clients/add endpoint. The WG flow
	// also persists the encrypted private key in wg_peers and applies
	// expiry/limit on the ownership row only (the panel's WG inbound
	// has no per-peer expiry / traffic-cap concept).
	if in.IsWireguard() {
		if s.wg == nil {
			return nil, fmt.Errorf("provision: WG inbound %q encountered but WG_MASTER_KEY not configured", inboundTag)
		}
		ownership, _, err := s.wg.ProvisionPeer(ctx, userID, nodeID, inboundTag, clientEmail, params)
		if err != nil {
			return nil, err
		}
		s.log.Info("provisioned WG peer",
			slog.Int64("user_id", userID), slog.Int64("node_id", nodeID),
			slog.String("inbound", inboundTag),
		)
		return ownership, nil
	}

	existing, err := s.ownership.GetByTriple(ctx, nodeID, inboundTag, clientEmail)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	newExpiry := computeExpiry(now, existing, params.DurationDays)
	newLimit := params.TrafficLimitBytes

	wireClient := buildWireClient(in.Protocol, clientEmail, user.SubID, newExpiry, newLimit)

	if existing == nil {
		if err := r.AddClient(ctx, inboundTag, wireClient); err != nil {
			return nil, fmt.Errorf("provision: add panel client: %w", err)
		}
	} else {
		if err := r.UpdateClient(ctx, inboundTag, wireClient); err != nil {
			// Update can fail on rare-but-possible identifier
			// rotations — fall back to add to keep provisioning
			// idempotent at the row level.
			if errors.Is(err, runtime.ErrClientNotFound) {
				if err := r.AddClient(ctx, inboundTag, wireClient); err != nil {
					return nil, fmt.Errorf("provision: re-add after missing: %w", err)
				}
			} else {
				return nil, fmt.Errorf("provision: update panel client: %w", err)
			}
		}
	}

	var trafficLimit *int64
	if newLimit > 0 {
		v := newLimit
		trafficLimit = &v
	}
	var expiryPtr *time.Time
	if !newExpiry.IsZero() {
		v := newExpiry
		expiryPtr = &v
	}
	row := &model.ClientOwnership{
		UserID:            userID,
		NodeID:            nodeID,
		InboundTag:        inboundTag,
		ClientEmail:       clientEmail,
		Protocol:          strings.ToLower(in.Protocol),
		PlanID:            params.PlanID,
		ExpiresAt:         expiryPtr,
		TrafficLimitBytes: trafficLimit,
		Enabled:           true,
	}
	if existing != nil {
		row.ID = existing.ID
	}
	saved, err := s.ownership.Upsert(ctx, row)
	if err != nil {
		return nil, err
	}
	s.log.Info("provisioned client",
		slog.Int64("user_id", userID),
		slog.Int64("node_id", nodeID),
		slog.String("inbound", inboundTag),
		slog.String("protocol", row.Protocol),
		slog.Bool("created", existing == nil),
	)
	return saved, nil
}

// AddClientDirect adds a client to a panel inbound without going
// through the plan-driven Provision flow. Used by the admin "manual
// add client" path where the caller controls the full Client struct
// (email + protocol identifier + limits + flow + sub_id, etc.) and
// optionally links to a user via userID > 0.
func (s *Service) AddClientDirect(ctx context.Context, nodeID int64, inboundTag string, c runtime.Client, userID int64) (*runtime.Client, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	in, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return nil, err
	}
	if err := r.AddClient(ctx, inboundTag, c); err != nil {
		return nil, err
	}
	// Optional ownership upsert when an owner is named.
	if userID > 0 && c.Email != "" {
		var expiry *time.Time
		if c.ExpiryTime > 0 {
			t := time.UnixMilli(c.ExpiryTime).UTC()
			expiry = &t
		}
		var limit *int64
		if c.TotalGB > 0 {
			v := c.TotalGB
			limit = &v
		}
		row := &model.ClientOwnership{
			UserID:            userID,
			NodeID:            nodeID,
			InboundTag:        inboundTag,
			ClientEmail:       c.Email,
			Protocol:          strings.ToLower(in.Protocol),
			ExpiresAt:         expiry,
			TrafficLimitBytes: limit,
			Enabled:           c.Enable,
		}
		if _, err := s.ownership.Upsert(ctx, row); err != nil {
			s.log.Warn("ownership upsert failed after direct add",
				slog.Int64("user_id", userID),
				slog.String("error", err.Error()),
			)
		}
	}
	return &c, nil
}

// UpdateClientDirect mutates an existing client identified by email.
// Mirrors 3x-ui's edit-client form: limits, expiry, flow, security
// can all be changed in place.
func (s *Service) UpdateClientDirect(ctx context.Context, nodeID int64, inboundTag string, c runtime.Client) (*runtime.Client, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if err := r.UpdateClient(ctx, inboundTag, c); err != nil {
		return nil, err
	}
	// Update the corresponding ownership row when one exists.
	existing, _ := s.ownership.GetByTriple(ctx, nodeID, inboundTag, c.Email)
	if existing != nil {
		if c.ExpiryTime > 0 {
			t := time.UnixMilli(c.ExpiryTime).UTC()
			existing.ExpiresAt = &t
		} else {
			existing.ExpiresAt = nil
		}
		if c.TotalGB > 0 {
			v := c.TotalGB
			existing.TrafficLimitBytes = &v
		} else {
			existing.TrafficLimitBytes = nil
		}
		existing.Enabled = c.Enable
		_, _ = s.ownership.Upsert(ctx, existing)
	}
	return &c, nil
}

// FetchSnapshot returns the dashboard-side composite — inbounds +
// online emails + last-online map — for one node. Frontend uses this
// when expanding an inbound row to show per-client online status.
func (s *Service) FetchSnapshot(ctx context.Context, nodeID int64) (*runtime.TrafficSnapshot, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return r.FetchTrafficSnapshot(ctx)
}

// DeleteClient removes the panel-side client and clears the
// ownership row. Idempotent — missing client or missing ownership
// is success.
func (s *Service) DeleteClient(ctx context.Context, nodeID int64, inboundTag, clientEmail string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := r.DeleteClientByEmail(ctx, inboundTag, clientEmail); err != nil {
		return fmt.Errorf("delete panel client: %w", err)
	}
	if err := s.ownership.ClearForClient(ctx, nodeID, inboundTag, clientEmail); err != nil {
		return err
	}
	return nil
}

// ListOnInbound lists every 3x-ui client on the inbound and
// annotates each with the matching ClientOwnership row (or nil for
// unmapped clients). Inputs: nodeID + inboundTag.
type AnnotatedClient struct {
	Client    runtime.Client          `json:"client"`
	Ownership *model.ClientOwnership  `json:"ownership,omitempty"`
}

func (s *Service) ListOnInbound(ctx context.Context, nodeID int64, inboundTag string) ([]AnnotatedClient, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	in, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return nil, err
	}
	clients, err := parseClientsFromInbound(in)
	if err != nil {
		return nil, err
	}
	out := make([]AnnotatedClient, 0, len(clients))
	for _, c := range clients {
		row, _ := s.ownership.GetByTriple(ctx, nodeID, inboundTag, c.Email)
		out = append(out, AnnotatedClient{Client: c, Ownership: row})
	}
	return out, nil
}

// LinkToUser attaches an unmapped 3x-ui client to a user by writing
// an ownership row. Used by admins after they manually create a
// client on a node panel. Best-effort populates the protocol from
// the inbound; falls back to empty (ExpiryJob will runtime-lookup).
func (s *Service) LinkToUser(ctx context.Context, nodeID int64, inboundTag, clientEmail string, userID int64, planID *int64) (*model.ClientOwnership, error) {
	protocol := ""
	if r, err := s.rt.Get(ctx, nodeID); err == nil {
		if in, err := r.GetInbound(ctx, inboundTag); err == nil {
			protocol = strings.ToLower(in.Protocol)
		}
	}
	row := &model.ClientOwnership{
		UserID:      userID,
		NodeID:      nodeID,
		InboundTag:  inboundTag,
		ClientEmail: clientEmail,
		Protocol:    protocol,
		PlanID:      planID,
		Enabled:     true,
	}
	return s.ownership.Upsert(ctx, row)
}

// UnlinkUser clears the ownership row without touching the panel.
func (s *Service) UnlinkUser(ctx context.Context, nodeID int64, inboundTag, clientEmail string) error {
	return s.ownership.ClearForClient(ctx, nodeID, inboundTag, clientEmail)
}

// ---- helpers --------------------------------------------------------------

// computeExpiry derives the new expiry timestamp. duration=0 means
// non-expiring (returns zero time).
func computeExpiry(now time.Time, existing *model.ClientOwnership, durationDays int) time.Time {
	if durationDays == 0 {
		return time.Time{}
	}
	base := now
	if existing != nil && existing.ExpiresAt != nil && existing.ExpiresAt.After(now) {
		base = *existing.ExpiresAt
	}
	return base.Add(time.Duration(durationDays) * 24 * time.Hour)
}

// buildWireClient constructs the 3x-ui Client object for the given
// protocol. VLESS / VMess get a UUID; Trojan / Shadowsocks get a
// random hex password. Everything else gets a UUID id (safe default).
func buildWireClient(protocol, email, subID string, expiry time.Time, trafficBytes int64) runtime.Client {
	c := runtime.Client{
		Email:   email,
		SubID:   subID,
		Enable:  true,
		TotalGB: trafficBytes, // bytes despite the name
	}
	if !expiry.IsZero() {
		c.ExpiryTime = expiry.UnixMilli()
	}
	switch strings.ToLower(protocol) {
	case "vless":
		c.ID = uuid.NewString()
		c.Flow = "" // dashboard-level default; admins can update later
	case "vmess":
		c.ID = uuid.NewString()
		c.Security = "auto"
	case "trojan":
		c.Password = randomHex(16)
	case "shadowsocks":
		c.Password = randomHex(16)
	case "hysteria", "hysteria2":
		c.Auth = randomAuthString(16)
	default:
		c.ID = uuid.NewString()
	}
	return c
}

// authAlphabet is the URL-safe character set Hysteria's auth
// strings draw from. Excludes ambiguous chars (0/O/1/l/I) so
// operators copy-pasting a value into a config file don't
// confuse them.
const authAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// randomAuthString returns an n-character random string from the
// URL-safe authAlphabet, drawn via crypto/rand (never math/rand —
// auth strings are auth bearers, NOT identifiers).
func randomAuthString(n int) string {
	buf := make([]byte, n)
	rb := make([]byte, n)
	if _, err := rand.Read(rb); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	for i := range rb {
		buf[i] = authAlphabet[int(rb[i])%len(authAlphabet)]
	}
	return string(buf)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is a fatal condition for the host.
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

